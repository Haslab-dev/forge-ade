import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import {
  HelpCircle,
  ChevronLeft,
  ChevronRight,
  Minus,
  X,
  Sparkles,
  CornerDownLeft,
  Check,
  Pencil
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentQuestionItem, AgentQuestionOption } from '../../types';
import { RespondAgentAsk, ACPRespondPermissionOption, ACPRespondPermission } from '../../lib/wails';
import { cn } from '../../lib/utils';

export function normalizeAgentQuestions(rawList?: any[] | null): AgentQuestionItem[] {
  if (!Array.isArray(rawList)) return [];
  return rawList.map((raw, qi) => {
    const id = String(raw.id || raw.ID || `q_${qi + 1}`);
    const header = String(raw.header || raw.Header || raw.scope || raw.category || 'SCOPE').toUpperCase();
    const question = String(raw.question || raw.Question || raw.title || raw.text || raw.prompt || '');
    const subtitle = String(raw.subtitle || raw.Subtitle || raw.description || raw.help || 'Please answer the following questions.');
    const isMultiSelect = Boolean(raw.isMultiSelect || raw.multi || raw.multiple || raw.Multi);
    const allowCustom = raw.allowCustom ?? true;
    const acpRequestId = raw.acpRequestId || raw.requestId;

    let options: AgentQuestionOption[] = [];
    if (Array.isArray(raw.options)) {
      options = raw.options.map((opt: any, oi: number) => {
        if (typeof opt === 'string') {
          const isRec = raw.recommended === oi || raw.recommended === opt || opt.toLowerCase().includes('(recommended)');
          const cleanLabel = opt.replace(/\s*\(recommended\)/i, '').trim();
          return {
            id: `opt_${oi + 1}`,
            label: cleanLabel,
            description: undefined,
            isRecommended: isRec,
            custom: false
          };
        }
        const isRec = Boolean(
          opt.isRecommended ||
          opt.recommended ||
          raw.recommended === oi ||
          raw.recommended === opt.id ||
          raw.recommended === opt.label ||
          (typeof opt.label === 'string' && opt.label.toLowerCase().includes('(recommended)'))
        );
        const cleanLabel = String(opt.label || opt.name || opt.text || opt.title || `Option ${oi + 1}`)
          .replace(/\s*\(recommended\)/i, '')
          .trim();
        return {
          id: String(opt.id || opt.optionId || `opt_${oi + 1}`),
          label: cleanLabel,
          description: opt.description || opt.desc || opt.detail || undefined,
          isRecommended: isRec,
          custom: Boolean(opt.custom)
        };
      });
    }

    return {
      id,
      header,
      question,
      subtitle,
      options,
      isMultiSelect,
      allowCustom,
      acpRequestId
    };
  });
}

export interface AgentQuestionModalProps {
  isOpen?: boolean;
  onClose?: () => void;
  sessionId?: string;
  goSessionId?: string;
  acpSessionId?: string;
  questions?: (AgentQuestionItem | any)[];
  onAnswer?: (answers: Record<string, string | string[]>) => Promise<void> | void;
  onSkip?: () => Promise<void> | void;
}

export const AgentQuestionModal: React.FC<AgentQuestionModalProps> = ({
  isOpen: controlledIsOpen,
  onClose: controlledOnClose,
  sessionId: propSessionId,
  goSessionId: propGoSessionId,
  acpSessionId: propAcpSessionId,
  questions: propQuestions,
  onAnswer: propOnAnswer,
  onSkip: propOnSkip
}) => {
  const activeSession = useWorkspace(s => s.activeSession);
  const setSessions = useWorkspace(s => s.setSessions);

  // Target session ID resolution
  const targetSession = activeSession;
  const goSessionId = propGoSessionId || targetSession?.goSessionId;
  const acpSessionId = propAcpSessionId || targetSession?.acpSessionId;
  const effectiveSessionId = propSessionId || targetSession?.id || goSessionId || acpSessionId;

  // Resolve raw questions list
  const rawQuestions = useMemo(() => {
    if (propQuestions && propQuestions.length > 0) return propQuestions;
    if (targetSession?.goState === 'awaiting_input' && (targetSession?.pendingQuestions?.length ?? 0) > 0) {
      return targetSession.pendingQuestions;
    }
    return [];
  }, [propQuestions, targetSession?.goState, targetSession?.pendingQuestions]);

  const questions: AgentQuestionItem[] = useMemo(() => {
    return normalizeAgentQuestions(rawQuestions);
  }, [rawQuestions]);

  const hasPendingQuestions = questions.length > 0;
  const [isMinimized, setIsMinimized] = useState(false);
  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(0);
  const [answers, setAnswers] = useState<Record<string, string | string[]>>({});
  const [customInputValues, setCustomInputValues] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [highlightedIndex, setHighlightedIndex] = useState<number>(0);

  const customInputRef = useRef<HTMLInputElement>(null);

  // Synchronize index bounds if questions change
  useEffect(() => {
    if (currentQuestionIndex >= questions.length && questions.length > 0) {
      setCurrentQuestionIndex(questions.length - 1);
    }
  }, [questions.length, currentQuestionIndex]);

  // Determine open state
  const isOpen = controlledIsOpen !== undefined ? controlledIsOpen : (hasPendingQuestions && !isMinimized);

  const currentQuestion: AgentQuestionItem | undefined = questions[currentQuestionIndex];

  // Prepare full option list including custom editable option if allowCustom is true
  const allOptions = useMemo<AgentQuestionOption[]>(() => {
    if (!currentQuestion) return [];
    const baseOpts = [...currentQuestion.options];
    const hasCustomOption = baseOpts.some(o => o.custom);
    if (!hasCustomOption && currentQuestion.allowCustom) {
      baseOpts.push({
        id: `custom_${currentQuestion.id}`,
        label: customInputValues[currentQuestion.id] || '',
        description: undefined,
        isRecommended: false,
        custom: true
      });
    }
    return baseOpts;
  }, [currentQuestion, customInputValues]);

  // Check if an option is currently selected
  const isOptionSelected = useCallback((opt: AgentQuestionOption) => {
    if (!currentQuestion) return false;
    const currentAnswer = answers[currentQuestion.id];
    if (opt.custom) {
      if (Array.isArray(currentAnswer)) {
        return currentAnswer.some(a => !currentQuestion.options.some(base => base.label === a || base.id === a));
      }
      return !!currentAnswer && !currentQuestion.options.some(base => base.label === currentAnswer || base.id === currentAnswer);
    }
    if (Array.isArray(currentAnswer)) {
      return currentAnswer.includes(opt.id) || currentAnswer.includes(opt.label);
    }
    return currentAnswer === opt.id || currentAnswer === opt.label;
  }, [currentQuestion, answers]);

  // Handle selecting or toggling an option
  const handleToggleOption = useCallback((opt: AgentQuestionOption) => {
    if (!currentQuestion) return;
    const qId = currentQuestion.id;
    const val = opt.label || opt.id;

    if (currentQuestion.isMultiSelect) {
      setAnswers(prev => {
        const cur = Array.isArray(prev[qId]) ? (prev[qId] as string[]) : [];
        const exists = cur.includes(val) || cur.includes(opt.id);
        const updated = exists
          ? cur.filter(x => x !== val && x !== opt.id)
          : [...cur, val];
        return { ...prev, [qId]: updated };
      });
    } else {
      setAnswers(prev => ({ ...prev, [qId]: val }));
    }
  }, [currentQuestion]);

  // Handle custom input change
  const handleCustomChange = useCallback((text: string) => {
    if (!currentQuestion) return;
    const qId = currentQuestion.id;
    setCustomInputValues(prev => ({ ...prev, [qId]: text }));

    if (currentQuestion.isMultiSelect) {
      setAnswers(prev => {
        const cur = Array.isArray(prev[qId]) ? (prev[qId] as string[]) : [];
        // replace non-standard option with typed text
        const baseLabels = currentQuestion.options.map(o => o.label);
        const baseIds = currentQuestion.options.map(o => o.id);
        const filtered = cur.filter(x => baseLabels.includes(x) || baseIds.includes(x));
        if (text.trim()) filtered.push(text);
        return { ...prev, [qId]: filtered };
      });
    } else {
      setAnswers(prev => ({ ...prev, [qId]: text }));
    }
  }, [currentQuestion]);

  // Select custom editable option
  const handleSelectCustom = useCallback((opt: AgentQuestionOption) => {
    setHighlightedIndex(allOptions.findIndex(o => o.id === opt.id));
    if (customInputRef.current) {
      customInputRef.current.focus();
    }
    if (!currentQuestion) return;
    const qId = currentQuestion.id;
    const currentVal = customInputValues[qId] || '';
    if (currentVal) {
      handleCustomChange(currentVal);
    }
  }, [allOptions, currentQuestion, customInputValues, handleCustomChange]);

  // Pagination navigation
  const handlePrev = useCallback(() => {
    if (currentQuestionIndex > 0) {
      setCurrentQuestionIndex(prev => prev - 1);
      setHighlightedIndex(0);
    }
  }, [currentQuestionIndex]);

  const handleNext = useCallback(() => {
    if (currentQuestionIndex < questions.length - 1) {
      setCurrentQuestionIndex(prev => prev + 1);
      setHighlightedIndex(0);
    }
  }, [currentQuestionIndex, questions.length]);

  const handleMinimize = useCallback(() => {
    setIsMinimized(true);
    controlledOnClose?.();
  }, [controlledOnClose]);

  const handleClose = useCallback(() => {
    setIsMinimized(true);
    controlledOnClose?.();
  }, [controlledOnClose]);

  // Cleanup pending state in workspaceStore
  const clearSessionQuestions = useCallback(() => {
    if (!effectiveSessionId) return;
    setSessions(prev => prev.map(s => {
      if (s.id === effectiveSessionId || s.goSessionId === effectiveSessionId || s.acpSessionId === effectiveSessionId) {
        return {
          ...s,
          goState: 'idle',
          pendingQuestions: []
        };
      }
      return s;
    }));
  }, [effectiveSessionId, setSessions]);

  // Submit answers to agent
  const handleSubmit = useCallback(async () => {
    if (submitting) return;
    setSubmitting(true);
    try {
      if (propOnAnswer) {
        await propOnAnswer(answers);
      } else if (goSessionId) {
        // Go harness internal agent
        await RespondAgentAsk(goSessionId, answers);
      } else if (acpSessionId) {
        // ACP orchestrator agent
        const firstQ = questions[0];
        const reqId = firstQ?.acpRequestId || firstQ?.id || 'req-1';
        const chosen = answers[firstQ.id] || Object.values(answers)[0] || 'allow';
        const chosenStr = Array.isArray(chosen) ? chosen.join(', ') : String(chosen);
        await ACPRespondPermissionOption(acpSessionId, reqId, true, chosenStr);
      }
      clearSessionQuestions();
      setIsMinimized(false);
      controlledOnClose?.();
    } catch (err) {
      console.error('Failed to submit agent questions answer:', err);
    } finally {
      setSubmitting(false);
    }
  }, [submitting, propOnAnswer, goSessionId, acpSessionId, questions, answers, clearSessionQuestions, controlledOnClose]);

  // Skip answers
  const handleSkip = useCallback(async () => {
    if (submitting) return;
    setSubmitting(true);
    try {
      if (propOnSkip) {
        await propOnSkip();
      } else if (goSessionId) {
        await RespondAgentAsk(goSessionId, {});
      } else if (acpSessionId) {
        const firstQ = questions[0];
        const reqId = firstQ?.acpRequestId || firstQ?.id || 'req-1';
        await ACPRespondPermission(acpSessionId, reqId, false);
      }
      clearSessionQuestions();
      setIsMinimized(false);
      controlledOnClose?.();
    } catch (err) {
      console.error('Failed to skip agent questions:', err);
    } finally {
      setSubmitting(false);
    }
  }, [submitting, propOnSkip, goSessionId, acpSessionId, questions, clearSessionQuestions, controlledOnClose]);

  const isLastQuestion = currentQuestionIndex === questions.length - 1;

  // Keyboard navigation
  useEffect(() => {
    if (!isOpen || !currentQuestion) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      const isInput = e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement;

      if (e.key === 'Escape') {
        e.preventDefault();
        handleMinimize();
        return;
      }

      if (e.key === 'Enter') {
        // Enter advances or submits
        e.preventDefault();
        if (isLastQuestion) {
          void handleSubmit();
        } else {
          handleNext();
        }
        return;
      }

      if (!isInput) {
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          setHighlightedIndex(prev => (prev + 1) % (allOptions.length || 1));
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          setHighlightedIndex(prev => (prev - 1 + (allOptions.length || 1)) % (allOptions.length || 1));
        } else if (e.key === 'ArrowLeft' && currentQuestionIndex > 0) {
          e.preventDefault();
          handlePrev();
        } else if (e.key === 'ArrowRight' && currentQuestionIndex < questions.length - 1) {
          e.preventDefault();
          handleNext();
        } else if (/^[1-9]$/.test(e.key)) {
          const num = parseInt(e.key, 10);
          if (num >= 1 && num <= allOptions.length) {
            e.preventDefault();
            const targetOpt = allOptions[num - 1];
            if (targetOpt.custom) {
              handleSelectCustom(targetOpt);
            } else {
              handleToggleOption(targetOpt);
            }
          }
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [
    isOpen,
    currentQuestion,
    currentQuestionIndex,
    questions.length,
    allOptions,
    isLastQuestion,
    handleMinimize,
    handleSubmit,
    handleNext,
    handlePrev,
    handleSelectCustom,
    handleToggleOption
  ]);

  if (!hasPendingQuestions) return null;

  return (
    <>
      {/* 1. TOP FLOATING NOTIFICATION CHIP: "Asking for your input >" */}
      {hasPendingQuestions && (
        <div
          data-testid="agent-input-chip"
          onClick={() => setIsMinimized(false)}
          className={cn(
            "fixed top-3 left-1/2 -translate-x-1/2 z-40 flex items-center gap-2 px-3.5 py-1.5 rounded-full cursor-pointer transition-all duration-200 select-none",
            "bg-surface-elevated/95 backdrop-blur-md border border-border/80 shadow-lg hover:border-foreground/30 hover:shadow-xl",
            isMinimized ? "scale-100 opacity-100 ring-2 ring-blue-500/30" : "opacity-80 hover:opacity-100"
          )}
          title="Agent is waiting for your input (Click to open dialog)"
        >
          <Sparkles className="size-3.5 text-blue-400 shrink-0 animate-pulse" />
          <span className="text-ui-xs font-medium text-foreground tracking-tight">
            Asking for your input
          </span>
          <ChevronRight className="size-3.5 text-muted-foreground ml-0.5" />
        </div>
      )}

      {/* 2. CENTER MODAL QUESTION DIALOG */}
      {isOpen && currentQuestion && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Agent questions dialog"
          data-testid="agent-questions-modal"
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150"
          onClick={e => {
            if (e.target === e.currentTarget) handleMinimize();
          }}
        >
          <div
            className="w-full max-w-[620px] bg-popover text-foreground border border-border/90 rounded-2xl shadow-2xl p-6 flex flex-col gap-4 animate-in zoom-in-95 duration-150 select-text"
            onClick={e => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="flex items-center justify-between">
              {/* Left: Help Circle + Title + Segmented Progress Bars */}
              <div className="flex items-center gap-2.5 min-w-0">
                <HelpCircle className="size-4 text-muted-foreground shrink-0" aria-hidden="true" />
                <span className="text-ui-base font-semibold text-foreground tracking-tight whitespace-nowrap">
                  Agent questions
                </span>
                {/* Segmented dash bar pills */}
                <div className="flex items-center gap-1.5 ml-1">
                  {questions.map((_, idx) => (
                    <div
                      key={idx}
                      className={cn(
                        "h-1 rounded-full transition-all duration-300",
                        idx === currentQuestionIndex
                          ? "w-6 bg-blue-500"
                          : idx < currentQuestionIndex
                          ? "w-3.5 bg-blue-500/50"
                          : "w-3.5 bg-foreground/15"
                      )}
                    />
                  ))}
                </div>
              </div>

              {/* Right: Stepper < 1 / 4 > | — ✕ */}
              <div className="flex items-center gap-1 text-muted-foreground shrink-0">
                <button
                  type="button"
                  onClick={handlePrev}
                  disabled={currentQuestionIndex === 0}
                  className="p-1 rounded hover:bg-surface-hover hover:text-foreground disabled:opacity-30 disabled:pointer-events-none cursor-pointer transition-colors"
                  title="Previous question"
                  aria-label="Previous question"
                >
                  <ChevronLeft className="size-3.5" />
                </button>
                <span className="text-ui-xs font-mono font-medium px-1 select-none">
                  {currentQuestionIndex + 1} / {questions.length}
                </span>
                <button
                  type="button"
                  onClick={handleNext}
                  disabled={isLastQuestion}
                  className="p-1 rounded hover:bg-surface-hover hover:text-foreground disabled:opacity-30 disabled:pointer-events-none cursor-pointer transition-colors"
                  title="Next question"
                  aria-label="Next question"
                >
                  <ChevronRight className="size-3.5" />
                </button>

                <div className="h-3 w-px bg-border/60 mx-1.5" />

                <button
                  type="button"
                  onClick={handleMinimize}
                  className="p-1 rounded hover:bg-surface-hover hover:text-foreground cursor-pointer transition-colors"
                  title="Minimize (Esc)"
                  aria-label="Minimize"
                >
                  <Minus className="size-3.5" />
                </button>
                <button
                  type="button"
                  onClick={handleClose}
                  className="p-1 rounded hover:bg-surface-hover hover:text-foreground cursor-pointer transition-colors"
                  title="Close / Dismiss"
                  aria-label="Close"
                >
                  <X className="size-3.5" />
                </button>
              </div>
            </div>

            {/* Scope / Category pill */}
            <div className="space-y-1">
              <div className="text-[10px] font-bold tracking-widest text-muted-foreground uppercase">
                {currentQuestion.header || 'SCOPE'}
              </div>
              <h3 className="text-ui-lg font-semibold text-foreground leading-snug">
                {currentQuestion.question}
              </h3>
              <p className="text-ui-xs text-muted-foreground">
                {currentQuestion.subtitle || 'Please answer the following questions.'}
              </p>
            </div>

            {/* Options List */}
            <div className="space-y-2 mt-1 max-h-[380px] overflow-y-auto scrollbar-thin pr-1">
              {allOptions.map((opt, oi) => {
                const isSelected = isOptionSelected(opt);
                const isHighlighted = highlightedIndex === oi;

                if (opt.custom) {
                  return (
                    <div
                      key={opt.id}
                      onClick={() => handleSelectCustom(opt)}
                      className={cn(
                        "flex items-center gap-3 p-3 rounded-xl border transition-all cursor-text",
                        isSelected || isHighlighted
                          ? "border-blue-500/80 bg-blue-500/5 ring-1 ring-blue-500/30"
                          : "border-border/60 bg-surface/40 hover:border-border hover:bg-surface"
                      )}
                    >
                      <span className="size-5 rounded flex items-center justify-center text-ui-xs font-mono font-medium bg-foreground/5 text-muted-foreground border border-border/80 shrink-0">
                        {oi + 1}
                      </span>
                      <Pencil className="size-3.5 text-muted-foreground shrink-0" aria-hidden="true" />
                      <input
                        ref={customInputRef}
                        type="text"
                        placeholder="Type custom answer..."
                        value={customInputValues[currentQuestion.id] || ''}
                        onChange={e => handleCustomChange(e.target.value)}
                        onFocus={() => setHighlightedIndex(oi)}
                        className="flex-1 bg-transparent border-none outline-none text-ui-sm text-foreground placeholder:text-muted-foreground/60"
                      />
                    </div>
                  );
                }

                return (
                  <div
                    key={opt.id}
                    onClick={() => handleToggleOption(opt)}
                    className={cn(
                      "flex items-start justify-between gap-3 p-3 rounded-xl border transition-all cursor-pointer select-none",
                      isSelected
                        ? "border-blue-500/60 bg-blue-500/5 ring-1 ring-blue-500/20"
                        : isHighlighted
                        ? "border-border bg-surface-hover"
                        : "border-border/60 bg-surface/30 hover:border-border hover:bg-surface"
                    )}
                  >
                    <div className="flex items-start gap-3 min-w-0 flex-1">
                      <span className="size-5 rounded flex items-center justify-center text-ui-xs font-mono font-medium bg-foreground/5 text-muted-foreground border border-border/80 shrink-0 mt-0.5">
                        {oi + 1}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-ui-sm font-medium text-foreground">
                            {opt.label}
                          </span>
                          {opt.isRecommended && (
                            <span className="inline-flex items-center gap-1 text-[11px] font-medium text-blue-400 bg-blue-500/10 border border-blue-500/20 px-2 py-0.5 rounded-full">
                              <Sparkles className="size-2.5" />
                              Recommended
                            </span>
                          )}
                        </div>
                        {opt.description && (
                          <p className="text-ui-xs text-muted-foreground mt-0.5 leading-normal">
                            {opt.description}
                          </p>
                        )}
                      </div>
                    </div>

                    {/* Indicator: Radio circle or Checkbox square */}
                    <div className="shrink-0 mt-1">
                      {currentQuestion.isMultiSelect ? (
                        <div
                          className={cn(
                            "size-4 rounded border flex items-center justify-center transition-colors",
                            isSelected ? "bg-blue-600 border-blue-600 text-white" : "border-border/80"
                          )}
                        >
                          {isSelected && <Check className="size-3 stroke-[3]" />}
                        </div>
                      ) : (
                        <div
                          className={cn(
                            "size-4 rounded-full border flex items-center justify-center transition-colors",
                            isSelected ? "border-blue-500" : "border-border/80"
                          )}
                        >
                          {isSelected && <div className="size-2 rounded-full bg-blue-500" />}
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Modal Footer */}
            <div className="flex items-center justify-between pt-3 border-t border-border/60 mt-1">
              {/* Keyboard shortcuts hints */}
              <div className="flex items-center gap-3 text-ui-xs text-muted-foreground font-mono select-none">
                <span className="flex items-center gap-1">
                  <span className="text-foreground/70 font-semibold">↑↓</span> move
                </span>
                <span className="flex items-center gap-1">
                  <span className="text-foreground/70 font-semibold">1-{allOptions.length}</span> pick
                </span>
                <span className="flex items-center gap-1">
                  <span className="text-foreground/70 font-semibold">esc</span> close
                </span>
              </div>

              {/* Action buttons */}
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleSkip}
                  disabled={submitting}
                  className="px-3.5 py-1.5 text-ui-xs font-medium text-muted-foreground hover:text-foreground rounded-lg transition-colors cursor-pointer disabled:opacity-50"
                >
                  Skip
                </button>
                <button
                  type="button"
                  onClick={isLastQuestion ? handleSubmit : handleNext}
                  disabled={submitting}
                  className="px-4 py-1.5 text-ui-xs font-semibold text-white bg-blue-600 hover:bg-blue-500 rounded-lg flex items-center gap-1.5 shadow-sm transition-all cursor-pointer disabled:opacity-50"
                >
                  <span>{isLastQuestion ? (submitting ? 'Submitting…' : 'Submit') : 'Next'}</span>
                  <CornerDownLeft className="size-3.5" />
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
};
