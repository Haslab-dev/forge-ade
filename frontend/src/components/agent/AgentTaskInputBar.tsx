import React, { useState, useRef, useEffect, useMemo } from 'react';
import { 
  Plus, 
  ChevronDown, 
  ChevronRight, 
  FileText, 
  GitBranch, 
  Cpu, 
  Terminal, 
  Upload, 
  ArrowUp, 
  Square, 
  Check,
  Shield,
  Wrench,
  X,
  Brain,
  Image as ImageIcon
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { AgentExecutionMode, AgentReasoningLevel } from '../../types';

interface AgentTaskInputBarProps {
  placeholder?: string;
  /** Optional context header row rendered inside the composer card top. */
  headerNode?: React.ReactNode;
  autoFocus?: boolean;
  onSubmitPrompt?: (prompt: string) => void;
  isCompact?: boolean;
}

const MODES: { id: AgentExecutionMode; label: string; desc: string }[] = [
  {
    id: 'ask',
    label: 'Ask before change',
    desc: 'Review and confirm changes before editing'
  },
  {
    id: 'code',
    label: 'Edit automatically',
    desc: 'Write code and edit files automatically'
  },
  {
    id: 'plan',
    label: 'Plan mode',
    desc: 'Generate implementation plan first'
  },
  {
    id: 'bypass',
    label: 'Full access',
    desc: 'Full system access and automatic approvals'
  }
];

const REASONING_LEVELS: { id: AgentReasoningLevel; label: string; desc: string }[] = [
  { id: 'low', label: 'Low', desc: 'Fast, lightweight reasoning' },
  { id: 'high', label: 'High', desc: 'Thorough multi-step reasoning' },
  { id: 'max', label: 'Medium', desc: 'Balanced reasoning depth' }
];

export const AgentTaskInputBar: React.FC<AgentTaskInputBarProps> = ({
  placeholder = 'Ask for follow-up changes',
  headerNode,
  autoFocus = false,
  onSubmitPrompt,
  isCompact = false
}) => {
  const { 
    agentExecutionMode, 
    setAgentExecutionMode,
    reasoningLevel,
    setReasoningLevel,
    currentModel,
    setCurrentModel,
    providers,
    sendAgentPrompt,
    createNewSession,
    activeSession,
    stopAgentExecution,
    files,
    skills,
    mcps,
    contextUsage
  } = useWorkspace();

  const [prompt, setPrompt] = useState('');
  const [attachedFiles, setAttachedFiles] = useState<{ name: string; path?: string; isImage?: boolean }[]>([]);
  const [isContextOpen, setIsContextOpen] = useState(false);
  const [activeSubMenu, setActiveSubMenu] = useState<string | null>(null);
  const [isModeOpen, setIsModeOpen] = useState(false);
  const [isModelOpen, setIsModelOpen] = useState(false);
  const [isReasoningOpen, setIsReasoningOpen] = useState(false);
  const [isUsageContextOpen, setIsUsageContextOpen] = useState(false);

  // Live input token addition (as user types or attaches files)
  const liveInputTokens = useMemo(() => {
    let chars = prompt.length;
    for (const f of attachedFiles) chars += (f.path || f.name).length;
    return Math.round(chars / 4);
  }, [prompt, attachedFiles]);

  const displayUsage = useMemo(() => {
    if (!contextUsage) {
      return {
        usedTokens: 0,
        maxTokens: 1000000,
        percent: 0,
        formattedUsed: '0',
        formattedMax: '1M',
        categories: {
          messages: { tokens: 0, percent: 0, formattedPercent: '0%' },
          mcpTools: { tokens: 0, percent: 0, formattedPercent: '0%' },
          systemTools: { tokens: 0, percent: 0, formattedPercent: '0%' },
          systemPrompt: { tokens: 0, percent: 0, formattedPercent: '0%' },
          skills: { tokens: 0, percent: 0, formattedPercent: '0%' },
          metaContext: { tokens: 0, percent: 0, formattedPercent: '0%' }
        }
      };
    }

    if (liveInputTokens === 0) return contextUsage;

    const newMessagesTokens = contextUsage.categories.messages.tokens + liveInputTokens;
    const newTotalUsed = contextUsage.usedTokens + liveInputTokens;
    const safeTotal = Math.max(newTotalUsed, 1);

    const calcCat = (toks: number) => {
      const pct = (toks / safeTotal) * 100;
      return {
        tokens: toks,
        percent: pct,
        formattedPercent: toks === 0 ? '0%' : pct < 0.1 ? '<0.1%' : `${pct.toFixed(1)}%`
      };
    };

    const percent = Math.min(100, Math.max(0, (newTotalUsed / contextUsage.maxTokens) * 100));

    const formattedUsed = newTotalUsed >= 1000000
      ? `${(newTotalUsed / 1000000).toFixed(2)}M`
      : newTotalUsed >= 1000
      ? `${(newTotalUsed / 1000).toFixed(1)}K`
      : `${newTotalUsed}`;

    return {
      usedTokens: newTotalUsed,
      maxTokens: contextUsage.maxTokens,
      percent,
      formattedUsed,
      formattedMax: contextUsage.formattedMax,
      categories: {
        messages: calcCat(newMessagesTokens),
        mcpTools: calcCat(contextUsage.categories.mcpTools.tokens),
        systemTools: calcCat(contextUsage.categories.systemTools.tokens),
        systemPrompt: calcCat(contextUsage.categories.systemPrompt.tokens),
        skills: calcCat(contextUsage.categories.skills.tokens),
        metaContext: calcCat(contextUsage.categories.metaContext.tokens)
      }
    };
  }, [contextUsage, liveInputTokens]);

  const containerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const isRunning = activeSession?.status === 'running';

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsContextOpen(false);
        setActiveSubMenu(null);
        setIsModeOpen(false);
        setIsModelOpen(false);
        setIsReasoningOpen(false);
        setIsUsageContextOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    setPrompt(val);
    if (val.endsWith('@')) {
      setIsContextOpen(true);
    }
  };

  const handleSubmit = () => {
    let fullPrompt = prompt.trim();
    if (attachedFiles.length > 0) {
      const attachments = attachedFiles.map(f => {
        const isImg = f.isImage ?? /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(f.name);
        const ref = f.path || f.name;
        return isImg ? `[Attached Image: ${ref}]` : `[Attached File: ${ref}]`;
      }).join('\n');
      fullPrompt = fullPrompt ? `${fullPrompt}\n\n${attachments}` : attachments;
    }

    if (!fullPrompt) return;

    if (onSubmitPrompt) {
      onSubmitPrompt(fullPrompt);
    } else if (activeSession) {
      sendAgentPrompt(fullPrompt);
    } else {
      createNewSession(fullPrompt);
    }

    setPrompt('');
    setAttachedFiles([]);
    setIsContextOpen(false);
    setIsModeOpen(false);
    setIsModelOpen(false);
    setIsReasoningOpen(false);
  };

  const insertContextTag = (tag: string) => {
    setPrompt(prev => {
      const clean = prev.endsWith('@') ? prev.slice(0, -1) : prev;
      return `${clean} @${tag} `.trimStart();
    });
    setIsContextOpen(false);
    setActiveSubMenu(null);
    textareaRef.current?.focus();
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      for (let i = 0; i < e.target.files.length; i++) {
        const file = e.target.files[i];
        const filePath = (file as any).path || '';
        const isImage = file.type.startsWith('image/') || /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(file.name);
        setAttachedFiles(prev => [
          ...prev, 
          { 
            name: file.name, 
            path: filePath || file.name, 
            isImage 
          }
        ]);
      }
      setIsContextOpen(false);
    }
    if (e.target) e.target.value = '';
  };

  const handleNativeFileUpload = async () => {
    try {
      const picked = await ApiBridge.pickNativeFiles();
      if (picked.length > 0) {
        const formatted = picked.map(p => ({
          name: p.name,
          path: p.path,
          isImage: /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(p.name)
        }));
        setAttachedFiles(prev => [...prev, ...formatted]);
        setIsContextOpen(false);
        return;
      }
    } catch {
      // fallback to html file input
    }
    fileInputRef.current?.click();
  };

  const currentMode = MODES.find(m => m.id === agentExecutionMode) || MODES[3];
  const currentReasoning = REASONING_LEVELS.find(r => r.id === reasoningLevel) || REASONING_LEVELS[2];

  const groupedModels = useMemo(() => {
    const groups: Array<{ providerName: string; models: string[] }> = [];
    
    for (const p of providers) {
      if (p.enabled && p.models && p.models.length > 0) {
        groups.push({
          providerName: p.name,
          models: p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models
        });
      }
    }

    if (groups.length === 0) {
      groups.push(
        { providerName: 'Forge ADE', models: ['kenari/gpt-5-6-luna', 'gemini-2.5-pro', 'claude-3-7-sonnet'] },
        { providerName: 'OpenAI', models: ['gpt-4o', 'gpt-4o-mini', 'o3-mini'] },
        { providerName: 'Anthropic', models: ['claude-3-7-sonnet-20250219', 'claude-3-5-sonnet-20241022'] }
      );
    }

    return groups;
  }, [providers]);

  return (
    <div className="w-full relative select-none" ref={containerRef}>
      <input 
        type="file" 
        multiple
        ref={fileInputRef} 
        onChange={handleFileUpload} 
        className="hidden" 
      />

      {/* Main Composer Box */}
      <div className="w-full rounded-2xl border border-white/10 dark:border-white/10 bg-card p-[12px_14px] flex flex-col gap-[10px] shadow-xl/5 relative transition-colors">
        
        {headerNode}

        {/* Attached Files Pills */}
        {attachedFiles.length > 0 && (
          <div className="flex flex-wrap gap-2 pb-1">
            {attachedFiles.map((file, idx) => (
              <div 
                key={idx} 
                className="flex items-center gap-1.5 px-3 py-1 rounded-[8px] bg-surface-hover text-foreground text-xs font-mono border border-border"
                title={file.path || file.name}
              >
                {file.isImage ? (
                  <ImageIcon className="w-3.5 h-3.5 text-info shrink-0" />
                ) : (
                  <FileText className="w-3.5 h-3.5 text-success shrink-0" />
                )}
                <span className="truncate max-w-[160px]">{file.name}</span>
                <button
                  type="button"
                  onClick={() => setAttachedFiles(prev => prev.filter((_, i) => i !== idx))}
                  className="text-foreground-subtle hover:text-destructive ml-1 cursor-pointer transition-colors"
                  title="Remove attachment"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Text Input Area */}
        <textarea
          ref={textareaRef}
          rows={isCompact ? 2 : 3}
          autoFocus={autoFocus}
          value={prompt}
          onChange={handleTextChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className="w-full resize-none bg-transparent border-0 text-ui-base text-foreground placeholder-foreground-subtlest focus:outline-hidden "
        />

        {/* Bottom Control Bar */}
        <div className="flex items-center justify-between gap-3 flex-wrap text-xs">
          
          {/* Left Controls: [+] [⚠️ Full access ⌄] */}
          <div className="flex items-center gap-4 relative">
            
            {/* [+] Button */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsContextOpen(prev => !prev);
                  setActiveSubMenu(null);
                  setIsModeOpen(false);
                  setIsModelOpen(false);
                  setIsReasoningOpen(false);
                }}
                className={`w-[18px] h-[18px] flex items-center justify-center transition-colors cursor-pointer text-foreground-subtle hover:text-foreground ${isContextOpen ? 'text-foreground' : ''}`}
                title="Add context (@)"
              >
                <Plus className="w-[18px] h-[18px]" />
              </button>

              {/* Context Popover */}
              {isContextOpen && (
                <div className="absolute left-0 bottom-full mb-3 w-56 rounded-[10px] bg-card shadow-2xl border border-border py-1.5 z-50 text-xs text-foreground">
                  {/* Files Submenu */}
                  <div className="relative">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('files')}
                      onClick={() => setActiveSubMenu(prev => prev === 'files' ? null : 'files')}
                      className="w-full text-left px-3.5 py-2 hover:bg-surface-hover flex items-center justify-between cursor-pointer transition-colors"
                    >
                      <div className="flex items-center gap-2">
                        <FileText className="w-3.5 h-3.5 text-foreground-subtle" />
                        <span>Files</span>
                      </div>
                      <ChevronRight className="w-3.5 h-3.5 text-foreground-subtlest" />
                    </button>

                    {activeSubMenu === 'files' && (
                      <div className="absolute left-full top-0 ml-1.5 w-52 rounded-[10px] bg-card shadow-xl border border-border py-1.5 z-50 max-h-56 overflow-y-auto">
                        {files.filter(f => f.type === 'file').length === 0 ? (
                          <div className="px-3.5 py-2 text-xs text-foreground-subtlest">No files loaded</div>
                        ) : (
                          files.filter(f => f.type === 'file').slice(0, 15).map(f => (
                            <button
                              key={f.id}
                              type="button"
                              onClick={() => insertContextTag(`file:${f.name}`)}
                              className="w-full text-left px-3.5 py-1.5 text-xs text-foreground-subtle hover:bg-surface-hover hover:text-foreground flex items-center gap-2 truncate cursor-pointer font-mono "
                            >
                              <FileText className="w-3 h-3 text-success shrink-0" />
                              <span className="truncate">{f.name}</span>
                            </button>
                          ))
                        )}
                      </div>
                    )}
                  </div>

                  {/* Skills */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('skills')}
                    className="w-full text-left px-3.5 py-2 hover:bg-surface-hover flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Cpu className="w-3.5 h-3.5 text-foreground-subtle" />
                    <span>Skills ({skills.length})</span>
                  </button>

                  {/* MCP Tools */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('mcp')}
                    className="w-full text-left px-3.5 py-2 hover:bg-surface-hover flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Wrench className="w-3.5 h-3.5 text-foreground-subtle" />
                    <span>MCP Tools ({mcps.length})</span>
                  </button>

                  {/* Git */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('git')}
                    className="w-full text-left px-3.5 py-2 hover:bg-surface-hover flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <GitBranch className="w-3.5 h-3.5 text-foreground-subtle" />
                    <span>Git Status & Diff</span>
                  </button>

                  {/* Terminal */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('terminal')}
                    className="w-full text-left px-3.5 py-2 hover:bg-surface-hover flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Terminal className="w-3.5 h-3.5 text-foreground-subtle" />
                    <span>Terminal</span>
                  </button>

                  <div className="h-[1px] bg-border my-1" />

                  {/* Attach Local File */}
                  <button
                    type="button"
                    onClick={handleNativeFileUpload}
                    className="w-full text-left px-3.5 py-2 hover:bg-surface-hover flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Upload className="w-3.5 h-3.5 text-foreground-subtle" />
                    <span>Attach Local File</span>
                  </button>
                </div>
              )}
            </div>

            {/* Mode / Access Selector Badge */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsModeOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsModelOpen(false);
                  setIsReasoningOpen(false);
                }}
                className="flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[13px] font-medium text-foreground-subtle hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer"
              >
                <Shield className="w-3.5 h-3.5" />
                <span>{currentMode.label}</span>
                <ChevronDown className="w-3 h-3 text-foreground-subtlest" />
              </button>

              {/* Mode Dropdown */}
              {isModeOpen && (
                <div className="absolute left-0 bottom-full mb-3 w-64 rounded-[10px] bg-card shadow-2xl border border-border py-1.5 z-50 text-xs">
                  <div className="px-3.5 py-1 text-ui-xs font-semibold text-foreground-subtle dark:text-foreground-subtlest uppercase tracking-wider">
                    Execution Mode
                  </div>
                  {MODES.map(m => {
                    const isSelected = m.id === agentExecutionMode;
                    return (
                      <button
                        key={m.id}
                        type="button"
                        onClick={() => {
                          setAgentExecutionMode(m.id);
                          setIsModeOpen(false);
                        }}
                        className={`w-full text-left px-3.5 py-2 flex items-start gap-2.5 hover:bg-surface-hover cursor-pointer transition-colors ${
                          isSelected ? 'bg-surface-hover text-foreground' : 'text-foreground-subtle'
                        }`}
                      >
                        <Shield className="w-3.5 h-3.5 text-foreground-subtle shrink-0 mt-0.5" />
                        <div className="flex-1 min-w-0">
                          <div className="font-semibold text-xs flex items-center justify-between text-foreground">
                            <span>{m.label}</span>
                            {isSelected && <Check className="w-3.5 h-3.5 text-success" />}
                          </div>
                          <p className="text-ui-xs text-foreground-subtle dark:text-foreground-subtlest leading-snug mt-0.5">
                            {m.desc}
                          </p>
                        </div>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

          </div>

          {/* Right Controls: [◉ Context] [⚪ Model ⌄] [🧠 Medium ⌄] [↑ Send] */}
          <div className="flex items-center gap-3.5 relative">
            
            {/* Context Window Usage Gauge (real usage; 0 on a fresh chat) */}
            <button
              type="button"
              onClick={() => {
                setIsUsageContextOpen(prev => !prev);
                setIsModelOpen(false);
                setIsContextOpen(false);
                setIsModeOpen(false);
                setIsReasoningOpen(false);
              }}
              className="w-4 h-4 rounded-full flex items-center justify-center cursor-pointer hover:scale-110 transition-transform shrink-0"
              title={`Context: ${displayUsage.formattedUsed}/${displayUsage.formattedMax} (${displayUsage.percent.toFixed(1)}%)`}
            >
              <svg className="w-3.5 h-3.5 -rotate-90" viewBox="0 0 36 36">
                <path className="text-border" strokeWidth="5" stroke="currentColor" fill="none"
                  d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
                <path className="text-success" strokeWidth="5" strokeLinecap="round" stroke="currentColor" fill="none"
                  strokeDasharray={`${Math.max(displayUsage.percent, displayUsage.usedTokens > 0 ? 1 : 0).toFixed(1)}, 100`}
                  d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
              </svg>
            </button>

            {isUsageContextOpen && (
              <div className="absolute right-0 bottom-full mb-3 w-64 rounded-2xl bg-popover shadow-2xl border border-border p-4 z-50 text-ui-sm animate-in fade-in zoom-in-95 duration-100">
                <div className="flex flex-col gap-1 pb-3 border-b border-border">
                  <div className="text-ui-sm font-medium text-foreground-subtle">Context window</div>
                  <div className="flex items-baseline justify-between">
                    <span className="text-ui-lg font-mono font-semibold text-foreground">
                      {displayUsage.formattedUsed}<span className="text-foreground-subtlest font-normal text-ui-sm">/{displayUsage.formattedMax}</span>
                    </span>
                    <span className="text-ui-sm font-mono font-medium text-success">
                      ({displayUsage.percent.toFixed(1)}%)
                    </span>
                  </div>
                  <div className="w-full h-1.5 bg-surface-hover rounded-full overflow-hidden mt-1.5">
                    <div className="h-full bg-success rounded-full transition-all duration-300"
                      style={{ width: `${Math.min(Math.max(displayUsage.percent, displayUsage.usedTokens > 0 ? 1 : 0), 100)}%` }} />
                  </div>
                  {activeSession && typeof activeSession.cacheHitRate === 'number' && activeSession.cacheHitRate > 0 && (
                    <div className="flex items-center justify-between mt-2">
                      <span className="text-ui-xs text-foreground-subtle">Avg cache hit</span>
                      <span className="font-mono text-ui-xs font-medium text-success">
                        {(activeSession.cacheHitRate * 100).toFixed(1)}%
                      </span>
                    </div>
                  )}
                  {displayUsage.usedTokens === 0 && (
                    <p className="text-ui-xs text-foreground-subtlest mt-1">Send the first message to start filling the context.</p>
                  )}
                </div>
                {displayUsage.usedTokens > 0 && (
                  <div className="flex flex-col gap-2 pt-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="w-2 h-2 rounded-full bg-info" />
                        <span className="text-foreground-subtle">Last request</span>
                      </div>
                      <span className="font-mono text-foreground font-medium">{displayUsage.categories.messages.formattedPercent}</span>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Model Selector Dropdown */}
              <div className="relative">
                <button
                  type="button"
                  onClick={() => {
                    setIsModelOpen(prev => !prev);
                    setIsContextOpen(false);
                    setIsModeOpen(false);
                    setIsReasoningOpen(false);
                    setIsUsageContextOpen(false);
                  }}
                  className="flex items-center gap-1.5 text-[14px] font-mono text-foreground hover:text-[#000000] dark:hover:text-white transition-colors cursor-pointer"
                >
                  <span className="truncate max-w-[150px]">{currentModel || 'gpt-5-6-luna'}</span>
                  <ChevronDown className="w-3.5 h-3.5 text-foreground-subtle" />
                </button>

                {/* Model Dropdown */}
                {isModelOpen && (
                  <div className="absolute right-0 bottom-full mb-3 w-64 rounded-[10px] bg-card shadow-2xl border border-border py-1.5 z-50 text-xs max-h-72 overflow-y-auto">
                    <div className="px-3.5 py-1 text-[10px] font-semibold text-foreground-subtle dark:text-foreground-subtlest uppercase tracking-wider">
                      Select Model
                    </div>
                    {groupedModels.map(group => (
                      <div key={group.providerName} className="py-1">
                        <div className="px-3.5 py-0.5 text-[10px] font-medium text-foreground-subtle uppercase">
                          {group.providerName}
                        </div>
                        {group.models.map((mod: string) => {
                          const isSelected = mod === currentModel;
                          return (
                            <button
                              key={mod}
                              type="button"
                              onClick={() => {
                                setCurrentModel(mod);
                                setIsModelOpen(false);
                              }}
                              className={`w-full text-left px-3.5 py-1.5 flex items-center justify-between hover:bg-surface-hover cursor-pointer transition-colors font-mono ${
                                isSelected ? 'bg-surface-hover text-foreground font-semibold' : 'text-foreground-subtle'
                              }`}
                            >
                              <div className="flex items-center gap-2 truncate">
                                <span className="w-1.5 h-1.5 rounded-full bg-success" />
                                <span className="truncate">{mod}</span>
                              </div>
                              {isSelected && <Check className="w-3.5 h-3.5 text-success" />}
                            </button>
                          );
                        })}
                      </div>
                    ))}
                  </div>
                )}
              </div>

            {/* Reasoning / Effort Level Selector: [🧠 Medium ⌄] */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsReasoningOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsModeOpen(false);
                  setIsModelOpen(false);
                }}
                className="flex items-center gap-1.5 text-[14px] text-foreground hover:text-[#000000] dark:hover:text-white transition-colors cursor-pointer"
              >
                <Brain className="w-[15px] h-[15px] text-foreground-subtle" />
                <span>{currentReasoning.label}</span>
                <ChevronDown className="w-3.5 h-3.5 text-foreground-subtle" />
              </button>

              {/* Reasoning Level Dropdown */}
              {isReasoningOpen && (
                <div className="absolute right-0 bottom-full mb-3 w-48 rounded-[10px] bg-card shadow-2xl border border-border py-1.5 z-50 text-xs">
                  <div className="px-3.5 py-1 text-[10px] font-semibold text-foreground-subtle dark:text-foreground-subtlest uppercase tracking-wider">
                    Reasoning Effort
                  </div>
                  {REASONING_LEVELS.map(r => {
                    const isSelected = r.id === reasoningLevel;
                    return (
                      <button
                        key={r.id}
                        type="button"
                        onClick={() => {
                          setReasoningLevel(r.id);
                          setIsReasoningOpen(false);
                        }}
                        className={`w-full text-left px-3.5 py-2 hover:bg-surface-hover cursor-pointer transition-colors ${
                          isSelected ? 'bg-surface-hover text-foreground font-medium' : 'text-foreground-subtle'
                        }`}
                      >
                        <div className="flex items-center justify-between">
                          <span>{r.label}</span>
                          {isSelected && <Check className="w-3.5 h-3.5 text-success" />}
                        </div>
                        <p className="text-[10px] text-foreground-subtle dark:text-foreground-subtlest mt-0.5">{r.desc}</p>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Send Button ↑ (or Stop button ■ if running) */}
            {isRunning ? (
              <button
                type="button"
                onClick={stopAgentExecution}
                className="w-[34px] h-[34px] rounded-[8px] bg-destructive text-white flex items-center justify-center hover:bg-[#B91C1C] dark:hover:bg-[#DC2626] transition-colors cursor-pointer shadow-xs"
                title="Stop execution"
              >
                <Square className="w-3.5 h-3.5 fill-current" />
              </button>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={!prompt.trim() && attachedFiles.length === 0}
                className={`w-[34px] h-[34px] rounded-[8px] flex items-center justify-center transition-colors cursor-pointer shadow-xs ${
                  !prompt.trim() && attachedFiles.length === 0
                    ? 'bg-border text-foreground-subtlest opacity-60 cursor-not-allowed'
                    : 'bg-[#111827] dark:bg-surface-hover hover:bg-[#1F2937] dark:hover:bg-surface-hover text-white dark:text-foreground'
                }`}
                title="Send task"
              >
                <ArrowUp className="w-[17px] h-[17px]" />
              </button>
            )}

          </div>

        </div>

      </div>
    </div>
  );
};
