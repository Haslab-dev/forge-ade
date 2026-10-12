import React, { useState, useRef, useEffect, useMemo } from 'react';
import { 
  Plus, 
  ChevronDown, 
  ChevronRight, 
  FileText, 
  GitBranch, 
  Terminal, 
  Upload, 
  ArrowUp, 
  Square, 
  Check,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Hand,
  Pencil,
  Wrench,
  X,
  Brain,
  Bot,
  Image as ImageIcon,
  RefreshCw,
  Loader2,
  Folder,
  Compass,
  CornerDownLeft
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { cn } from '../../lib/utils';
import { AtMentionMenu, flattenFileTree, extractAtQuery, type AtMentionItem } from './AtMentionMenu';
import { SetActiveModel, GetClipboardFiles } from '../../lib/wails';
import { ApiBridge } from '../../services/apiBridge';
import { ProviderIcon } from './ProviderIcon';
import { AgentExecutionMode, AgentReasoningLevel } from '../../types';

export interface SlashCommandItem {
  name: string;
  description: string;
  category: string;
}

interface AgentTaskInputBarProps {
  placeholder?: string;
  headerNode?: React.ReactNode;
  autoFocus?: boolean;
  onSubmitPrompt?: (prompt: string) => void;
  isCompact?: boolean;
  /** Stretch to the full column width (session view) instead of the capped chat measure. */
  fillWidth?: boolean;
}

const MODES: { id: AgentExecutionMode; label: string; desc: string; icon: any }[] = [
  {
    id: 'ask',
    label: 'Ask for approval',
    desc: 'Always ask before editing files or running commands',
    icon: Hand
  },
  {
    id: 'code',
    label: 'Approve edits',
    desc: 'Edit files freely, ask before running commands',
    icon: Pencil
  },
  {
    id: 'plan',
    label: 'Approve for me',
    desc: 'Only ask for actions detected as potentially unsafe',
    icon: ShieldCheck
  },
  {
    id: 'bypass',
    label: 'Full access',
    desc: 'Unrestricted access to the filesystem and commands',
    icon: ShieldAlert
  }
];

const ALL_REASONING_LEVELS: { id: AgentReasoningLevel; label: string; desc: string }[] = [
  { id: 'off', label: 'Off', desc: 'No reasoning' },
  { id: 'minimal', label: 'Minimal', desc: 'Lowest reasoning effort' },
  { id: 'low', label: 'Low', desc: 'Fast, lightweight reasoning' },
  { id: 'medium', label: 'Medium', desc: 'Balanced reasoning depth' },
  { id: 'high', label: 'High', desc: 'Thorough multi-step reasoning' },
  { id: 'xhigh', label: 'Extra High', desc: 'Extended deep reasoning' },
  { id: 'max', label: 'Max', desc: 'Maximum available reasoning budget' }
];

export const AgentTaskInputBar: React.FC<AgentTaskInputBarProps> = ({
  placeholder = 'Ask anything, @ threads or files, $ skills, or / commands',
  headerNode,
  autoFocus = false,
  onSubmitPrompt,
  isCompact = false,
  fillWidth = false
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
    steerAgentPrompt,
    createNewSession,
    activeSession,
    stopAgentExecution,
    files,
    skills,
    mcps,
    contextUsage,
    setMode,
    clearTasks,
    activeWorkspacePath
  } = useWorkspace();

  const [prompt, setPrompt] = useState('');
  const [steerQueue, setSteerQueue] = useState<string[]>([]);
  const [attachedFiles, setAttachedFiles] = useState<{
    name: string;
    path?: string;
    isImage?: boolean;
    previewUrl?: string;
    size?: number;
  }[]>([]);
  const [isDraggingOver, setIsDraggingOver] = useState(false);
  const [isContextOpen, setIsContextOpen] = useState(false);
  const [activeSubMenu, setActiveSubMenu] = useState<string | null>(null);
  const [isModeOpen, setIsModeOpen] = useState(false);
  const [isModelPickerOpen, setIsModelPickerOpen] = useState(false);
  const [activeModelSubMenu, setActiveModelSubMenu] = useState<'model' | null>(null);
  const [isUsageContextOpen, setIsUsageContextOpen] = useState(false);

  // @-mention: opens while the text before the caret has "@<query>".
  const [atOpen, setAtOpen] = useState(false);
  const [atItems, setAtItems] = useState<AtMentionItem[]>([]);
  const [atIndex, setAtIndex] = useState(0);

  // Slash menu: opens while the text before the caret starts with "/".
  const [slashOpen, setSlashOpen] = useState(false);
  const [slashQuery, setSlashQuery] = useState('');
  const [slashIndex, setSlashIndex] = useState(0);

  const containerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const isRunning = activeSession?.status === 'running';

  const currentMode = useMemo(() => {
    return MODES.find(m => m.id === agentExecutionMode) || MODES[0];
  }, [agentExecutionMode]);

  // The internal harness exposes the standard effort ladder.
  const harnessReasoningLevels = useMemo(
    () => ALL_REASONING_LEVELS.filter(r => ['low', 'medium', 'high'].includes(r.id)),
    []
  );

  const currentReasoning = useMemo(() => {
    return harnessReasoningLevels.find(r => r.id === reasoningLevel) || harnessReasoningLevels[0] || ALL_REASONING_LEVELS[3];
  }, [harnessReasoningLevels, reasoningLevel]);

  // Live input tokens
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
      };
    }
    const total = contextUsage.usedTokens + liveInputTokens;
    const pct = Math.min(100, Math.max(0, (total / contextUsage.maxTokens) * 100));
    const formatted = total >= 1000000
      ? `${(total / 1000000).toFixed(2)}M`
      : total >= 1000
      ? `${(total / 1000).toFixed(1)}K`
      : `${total}`;

    return {
      usedTokens: total,
      maxTokens: contextUsage.maxTokens,
      percent: pct,
      formattedUsed: formatted,
      formattedMax: contextUsage.formattedMax || '1M'
    };
  }, [contextUsage, liveInputTokens]);

  // Adjust textarea height automatically
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      const scrollH = textareaRef.current.scrollHeight;
      const minH = isCompact ? 36 : 48;
      const maxH = 200;
      textareaRef.current.style.height = `${Math.min(Math.max(scrollH, minH), maxH)}px`;
    }
  }, [prompt, isCompact]);

  // Click outside to close menus
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsContextOpen(false);
        setIsModeOpen(false);
        setIsModelPickerOpen(false);
        setActiveModelSubMenu(null);
        setIsUsageContextOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Element picker (Browser viewer "select element for chat") appends the
  // picked element reference into the prompt.
  useEffect(() => {
    const onInsert = (e: Event) => {
      const detail = (e as CustomEvent<string>).detail;
      if (!detail) return;
      setPrompt(prev => (prev ? `${prev} ${detail}` : `${detail} `));
      textareaRef.current?.focus();
    };
    window.addEventListener('forge:composer-insert', onInsert);
    return () => window.removeEventListener('forge:composer-insert', onInsert);
  }, []);

  const handleSteer = (instructionToSteer?: string) => {
    const text = (instructionToSteer ?? prompt).trim();
    if (!text) return;
    void steerAgentPrompt(text);
    if (!instructionToSteer) {
      setPrompt('');
      setAttachedFiles([]);
    }
  };

  const handleQueueSteer = () => {
    const text = prompt.trim();
    if (!text) return;
    setSteerQueue(prev => [...prev, text]);
    setPrompt('');
  };

  const handleSubmit = () => {
    if (!prompt.trim() && attachedFiles.length === 0) return;
    if (isRunning) {
      // If agent is already running, Enter queues instruction into stack above
      handleQueueSteer();
      return;
    }

    let effectivePrompt = prompt.trim();
    if (attachedFiles.length > 0) {
      const fileRefs = attachedFiles
        .map(f => f.path || f.name)
        .filter(Boolean)
        .filter(p => !effectivePrompt.includes(p));
      if (fileRefs.length > 0) {
        const fileList = fileRefs.map(p => `@${p}`).join('\n');
        effectivePrompt = effectivePrompt
          ? `${effectivePrompt}\n\nAttached files:\n${fileList}`
          : `Please review the following attached files:\n${fileList}`;
      }
    }

    if (onSubmitPrompt) {
      onSubmitPrompt(effectivePrompt);
    } else if (activeSession) {
      sendAgentPrompt(effectivePrompt);
    } else {
      createNewSession(effectivePrompt);
    }

    setPrompt('');
    setAttachedFiles([]);
  };

  // Slash catalog: skills + plan + goal + security + model + settings + vibe + MCPs
  const slashCatalog = useMemo(() => {
    const skillCount = (skills || []).length;
    const currentGoal = activeSession?.progress?.current_goal || '';
    const hasGoal = !!currentGoal;

    const isPlanMode = (activeSession?.executionMode as string) === 'plan' || currentGoal.toLowerCase().startsWith('plan:');

    const items: SlashCommandItem[] = [
      {
        name: '/skill:',
        description: `${skillCount} skills`,
        category: 'skill'
      },
      {
        name: '/model',
        description: `Model: ${currentModel || 'openrouter/dots-studio/dots-3-note-preview:free'}`,
        category: 'system'
      },
      {
        name: '/resume',
        description: 'Resume a different session',
        category: 'session'
      },
      {
        name: '/security',
        description: 'Plan, run, inspect, import, and compare OMP-native security scans',
        category: 'security'
      },
      {
        name: '/settings',
        description: 'Open settings menu',
        category: 'settings'
      },
      {
        name: '/setup',
        description: 'Open provider setup',
        category: 'settings'
      },
      {
        name: '/plan',
        description: isPlanMode ? 'Plan: on' : 'Plan: off',
        category: 'plan'
      },
      {
        name: '/plan-review',
        description: isPlanMode ? 'Plan review: ready' : 'Plan review: plan mode inactive',
        category: 'plan'
      },
      {
        name: '/vibe',
        description: 'Vibe: off',
        category: 'mode'
      },
      {
        name: '/goal',
        description: hasGoal ? `Goal: ${currentGoal}` : 'Goal: off',
        category: 'goal'
      },
      {
        name: '/clear',
        description: 'Clear active tasks and reset objective checklist',
        category: 'command'
      },
      ...(skills || []).map((s) => ({
        name: `/skill:${s.name}`,
        description: s.description || `Run ${s.name} skill`,
        category: 'skill'
      })),
      ...(mcps || []).map((m) => {
        const name = typeof m === 'string' ? m : m?.name;
        return { name: `/mcp:${name}`, description: `MCP server: ${name}`, category: 'mcp' };
      }).filter((it) => !!it.name)
    ];
    return items;
  }, [skills, mcps, currentModel, activeSession?.progress?.current_goal, activeSession?.executionMode]);

  // Text before the caret decides whether the menu shows and what it filters.
  const updateSlashState = () => {
    const el = textareaRef.current;
    if (!el) return;
    const upto = el.value.slice(0, el.selectionStart ?? 0);
    const match = /(?:^|\s)\/([^\s]*)$/.exec(upto);
    if (match) {
      setSlashQuery(match[1].toLowerCase());
      setSlashIndex(0);
      setSlashOpen(true);
    } else {
      setSlashOpen(false);
    }
  };

  const filteredSlash = useMemo(
    () =>
      slashCatalog.filter((it) =>
        slashQuery ? it.name.toLowerCase().includes(slashQuery) : true
      ),
    [slashCatalog, slashQuery]
  );

  const applySlash = (item: SlashCommandItem) => {
    if (item.name === '/settings' || item.name === '/setup') {
      setSlashOpen(false);
      setPrompt('');
      setMode('settings');
      return;
    }
    if (item.name === '/clear') {
      setSlashOpen(false);
      setPrompt('');
      void clearTasks();
      return;
    }
    if (item.name === '/model') {
      setSlashOpen(false);
      setIsModelPickerOpen(true);
      return;
    }
    const el = textareaRef.current;
    if (!el) return;
    const pos = el.selectionStart ?? el.value.length;
    const upto = el.value.slice(0, pos);
    const startIdx = upto.lastIndexOf('/');
    if (startIdx === -1) return;
    const insertText = item.name === '/skill:' ? '/skill:' : item.name + ' ';
    const next = el.value.slice(0, startIdx) + insertText + el.value.slice(pos);
    setPrompt(next);
    if (item.name === '/skill:') {
      setSlashQuery('skill:');
      setSlashOpen(true);
    } else {
      setSlashOpen(false);
    }
    requestAnimationFrame(() => {
      el.focus();
      const caret = startIdx + insertText.length;
      el.setSelectionRange(caret, caret);
    });
  };

  // @-mention candidates from the workspace tree (files + folders).
  const atCandidates = useMemo(() => flattenFileTree(files || []), [files]);

  const updateAtState = () => {
    const el = textareaRef.current;
    if (!el) return;
    const upto = el.value.slice(0, el.selectionStart ?? 0);
    const query = extractAtQuery(upto);
    if (query === null) {
      setAtOpen(false);
      return;
    }
    const q = query.toLowerCase();
    const matches = atCandidates
      .filter((c) => q === '' || c.path.toLowerCase().includes(q))
      .slice(0, 8);
    setAtItems(matches);
    setAtIndex(0);
    setAtOpen(matches.length > 0);
  };

  const applyAtMention = (item: AtMentionItem) => {
    const el = textareaRef.current;
    if (!el) return;
    const pos = el.selectionStart ?? el.value.length;
    const upto = el.value.slice(0, pos);
    const startIdx = upto.lastIndexOf('@');
    if (startIdx === -1) return;
    const token = '@' + item.path;
    const next = el.value.slice(0, startIdx) + token + ' ' + el.value.slice(pos);
    setPrompt(next);
    setAtOpen(false);
    requestAnimationFrame(() => {
      el.focus();
      const caret = startIdx + token.length + 1;
      el.setSelectionRange(caret, caret);
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (atOpen && atItems.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setAtIndex((i) => (i + 1) % atItems.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setAtIndex((i) => (i - 1 + atItems.length) % atItems.length);
        return;
      }
      if (e.key === 'Tab' || (e.key === 'Enter' && !e.shiftKey)) {
        e.preventDefault();
        applyAtMention(atItems[Math.min(atIndex, atItems.length - 1)]);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setAtOpen(false);
        return;
      }
    }
    if (slashOpen && filteredSlash.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSlashIndex((i) => (i + 1) % filteredSlash.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSlashIndex((i) => (i - 1 + filteredSlash.length) % filteredSlash.length);
        return;
      }
      if (e.key === 'Tab' || (e.key === 'Enter' && !e.shiftKey)) {
        e.preventDefault();
        applySlash(filteredSlash[Math.min(slashIndex, filteredSlash.length - 1)]);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setSlashOpen(false);
        return;
      }
    }
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      if (isRunning && prompt.trim()) {
        e.preventDefault();
        handleSteer();
        return;
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (isRunning) {
        handleQueueSteer();
      } else {
        handleSubmit();
      }
      return;
    }
  };

  const handleNativeFileUpload = () => {
    fileInputRef.current?.click();
  };

  const processFilesForAttachment = async (filesToProcess: (File | string)[]) => {
    const newAttachments: {
      name: string;
      path?: string;
      isImage?: boolean;
      previewUrl?: string;
      size?: number;
    }[] = [];

    for (const item of filesToProcess) {
      if (typeof item === 'string') {
        const p = item;
        const name = p.split(/[/\\]/).pop() || p;
        const isImage = /\.(png|jpe?g|gif|webp|svg|ico|bmp)$/i.test(p);
        let previewUrl: string | undefined;
        if (isImage) {
          try {
            const b64 = await ApiBridge.readFileBase64(p);
            if (b64) {
              const ext = name.split('.').pop()?.toLowerCase() || 'png';
              previewUrl = `data:image/${ext};base64,${b64}`;
            }
          } catch {}
        }
        newAttachments.push({ name, path: p, isImage, previewUrl });
      } else {
        const file = item;
        const isImage = file.type.startsWith('image/') || /\.(png|jpe?g|gif|webp|svg|ico|bmp)$/i.test(file.name);
        let previewUrl: string | undefined;
        let targetPath = (file as any).path;

        if (isImage) {
          try {
            previewUrl = await new Promise<string>((resolve) => {
              const reader = new FileReader();
              reader.onload = () => resolve(reader.result as string);
              reader.readAsDataURL(file);
            });
            if (!targetPath && activeWorkspacePath && previewUrl) {
              const comma = previewUrl.indexOf(',');
              const b64 = comma !== -1 ? previewUrl.slice(comma + 1) : previewUrl;
              const sanitizedName = file.name.replace(/[^a-zA-Z0-9._-]/g, '_');
              const fileName = `screenshot_${Date.now()}_${sanitizedName}`;
              const fullPath = `${activeWorkspacePath}/.forge/attachments/${fileName}`;
              await ApiBridge.writeFileBase64(fullPath, b64);
              targetPath = fullPath;
            }
          } catch {}
        }
        newAttachments.push({
          name: file.name,
          path: targetPath,
          isImage,
          previewUrl,
          size: file.size
        });
      }
    }

    setAttachedFiles(prev => [...prev, ...newAttachments]);
    textareaRef.current?.focus();
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const filesList = e.target.files;
    if (!filesList) return;
    void processFilesForAttachment(Array.from(filesList));
    e.target.value = '';
    setIsContextOpen(false);
  };

  const insertContextTag = (tag: string) => {
    setPrompt(prev => prev ? `${prev} @${tag} ` : `@${tag} `);
    setIsContextOpen(false);
    textareaRef.current?.focus();
  };

  useEffect(() => {
    const handleForgeDrop = (ev: Event) => {
      const detail = (ev as CustomEvent)?.detail;
      if (!detail?.files || detail.files.length === 0) return;
      const el = containerRef.current;
      if (!el) return;

      const target = detail.target as HTMLElement | null;
      if (target?.closest('[data-editor-drop-target="true"]') || target?.closest('[data-tree-drop-target="true"]')) {
        return;
      }

      const rect = el.getBoundingClientRect();
      const chatPane = el.closest('[data-agent-chat="true"]') || document.querySelector('[data-agent-chat="true"]');
      const chatRect = chatPane?.getBoundingClientRect();

      const insideComposer =
        detail.x >= rect.left &&
        detail.x <= rect.right &&
        detail.y >= rect.top &&
        detail.y <= rect.bottom;

      const insideChat =
        Boolean(
          chatRect &&
          detail.x >= chatRect.left &&
          detail.x <= chatRect.right &&
          detail.y >= chatRect.top &&
          detail.y <= chatRect.bottom
        );

      if (!insideComposer && !insideChat) return;

      ev.preventDefault();
      ev.stopPropagation();
      setIsDraggingOver(false);

      void processFilesForAttachment(detail.files);
    };

    window.addEventListener('forge:file-drop', handleForgeDrop);
    return () => window.removeEventListener('forge:file-drop', handleForgeDrop);
  }, [activeWorkspacePath]);

  const isFullAccess = agentExecutionMode === 'bypass';

  return (
    <div
      className={`w-full relative select-none mx-auto @container/composer ${fillWidth ? 'max-w-none px-1' : 'max-w-[var(--app-chat-max-width,48rem)]'}`}
      ref={containerRef}
      data-file-drop-target="true"
      onDragEnter={(e) => {
        e.preventDefault();
        setIsDraggingOver(true);
      }}
      onDragOver={(e) => {
        e.preventDefault();
        setIsDraggingOver(true);
      }}
      onDragLeave={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node)) {
          setIsDraggingOver(false);
        }
      }}
      onDrop={(e) => {
        e.preventDefault();
        setIsDraggingOver(false);
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
          void processFilesForAttachment(Array.from(e.dataTransfer.files));
        }
      }}
    >
      <input 
        type="file" 
        multiple
        ref={fileInputRef} 
        onChange={handleFileUpload} 
        className="hidden" 
      />

      {/* Stacked header slot (Landing context tray or diffs) */}
      {headerNode && (
        <div className="chat-composer-stacked-top mx-auto -mb-px w-14/15 min-w-0 border border-b-0 border-[color:var(--composer-stacked-border)]">
          {headerNode}
        </div>
      )}

      {/* Stacked Mid-Turn Steering Queue Tray ABOVE input area */}
      {steerQueue.length > 0 && (
        <div className="mb-2 w-full rounded-2xl bg-panel/95 backdrop-blur-xs border border-border/80 shadow-md p-2.5 sm:p-3 flex flex-col gap-2 animate-in fade-in slide-in-from-bottom-2 duration-150">
          <div className="flex items-center justify-between text-ui-xs font-semibold text-muted-foreground uppercase tracking-wider px-1">
            <span className="flex items-center gap-1.5">
              <Compass className="size-3.5 text-accent-primary animate-spin" style={{ animationDuration: '6s' }} />
              <span>Stacked Steering Instructions ({steerQueue.length})</span>
            </span>
            <button
              type="button"
              onClick={() => setSteerQueue([])}
              className="text-ui-xs text-muted-foreground hover:text-destructive cursor-pointer transition-colors"
            >
              Clear all
            </button>
          </div>
          <div className="flex flex-col gap-1.5 max-h-48 overflow-y-auto pr-0.5">
            {steerQueue.map((item, idx) => (
              <div
                key={idx}
                className="flex items-center justify-between gap-3 px-3 py-2 rounded-xl bg-[var(--color-background-button-secondary)] text-foreground border border-border/80 text-ui-xs group shadow-2xs"
              >
                <div className="flex items-center gap-2.5 min-w-0 flex-1">
                  <span className="size-5 shrink-0 rounded-full bg-accent-primary/15 text-accent-primary font-mono text-ui-xs flex items-center justify-center font-bold">
                    {idx + 1}
                  </span>
                  <span className="text-ui-xs text-foreground font-normal break-words">{item}</span>
                </div>
                <div className="flex items-center gap-1.5 shrink-0">
                  <button
                    type="button"
                    onClick={() => {
                      handleSteer(item);
                      setSteerQueue(prev => prev.filter((_, i) => i !== idx));
                    }}
                    className="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-accent-primary text-white hover:opacity-90 cursor-pointer font-medium text-ui-xs transition-all shadow-2xs"
                    title="Inject this steering instruction immediately into the running agent loop"
                  >
                    <Compass className="size-3" />
                    <span>Steer</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setSteerQueue(prev => prev.filter((_, i) => i !== idx))}
                    className="size-6 rounded-full hover:bg-surface flex items-center justify-center text-foreground-subtle hover:text-destructive cursor-pointer transition-colors"
                    title="Remove instruction"
                  >
                    <X className="size-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Main Kybern Composer Squircle Container */}
      <div className="chat-composer-surface border border-[color:var(--surface-border)] p-2.5 sm:p-3 flex flex-col gap-2 relative">
        
        {/* Drag and Drop Visual Overlay */}
        {isDraggingOver && (
          <div className="absolute inset-0 bg-background/90 backdrop-blur-xs border-2 border-dashed border-accent-primary rounded-xl z-50 flex items-center justify-center gap-3 text-fg-primary pointer-events-none select-none animate-in fade-in duration-150">
            <div className="p-2.5 rounded-full bg-accent-primary/10 text-accent-primary animate-bounce">
              <ImageIcon className="w-5 h-5" />
            </div>
            <div className="flex flex-col">
              <span className="text-ui-sm font-semibold text-foreground">Drop images or files here</span>
              <span className="text-ui-xs text-muted-foreground">Attach directly to your agent prompt</span>
            </div>
          </div>
        )}

        {/* Attached Files Badges */}
        {attachedFiles.length > 0 && (
          <div className="flex flex-wrap gap-2 pb-1.5">
            {attachedFiles.map((file, idx) => (
              <div 
                key={idx} 
                className="flex items-center gap-2 p-1 pl-1.5 pr-2 rounded-lg bg-[var(--color-background-button-secondary)] text-foreground text-ui-xs border border-border group shadow-2xs"
                title={file.path || file.name}
              >
                {file.isImage ? (
                  file.previewUrl ? (
                    <img
                      src={file.previewUrl}
                      alt={file.name}
                      className="size-7 rounded object-cover border border-border/60 shrink-0 bg-panel"
                    />
                  ) : (
                    <div className="size-7 rounded bg-accent-primary/10 flex items-center justify-center shrink-0">
                      <ImageIcon className="size-3.5 text-accent-primary" />
                    </div>
                  )
                ) : (
                  <div className="size-7 rounded bg-success/10 flex items-center justify-center shrink-0">
                    <FileText className="size-3.5 text-success" />
                  </div>
                )}
                <div className="flex flex-col min-w-0 max-w-[150px]">
                  <span className="truncate font-medium text-ui-xs text-foreground leading-tight">{file.name}</span>
                  <span className="text-[10px] text-muted-foreground leading-tight">
                    {file.isImage ? 'Image attachment' : 'File'}
                  </span>
                </div>
                <button
                  type="button"
                  onClick={() => setAttachedFiles(prev => prev.filter((_, i) => i !== idx))}
                  className="size-5 rounded-full hover:bg-surface flex items-center justify-center text-foreground-subtle hover:text-destructive cursor-pointer transition-colors shrink-0 ml-0.5"
                  title="Remove attachment"
                >
                  <X className="size-3" />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* @-mention menu */}
        {atOpen && atItems.length > 0 && (
          <AtMentionMenu items={atItems} index={atIndex} onSelect={applyAtMention} />
        )}

        {/* Slash command menu */}
        {slashOpen && filteredSlash.length > 0 && (
          <div className="absolute bottom-full left-3 right-3 mb-1 max-h-60 overflow-y-auto rounded-xl border border-border bg-popover shadow-2xl z-40 p-1 divide-y divide-border/40">
            <div className="px-3 py-1.5 text-[11px] font-semibold text-foreground-subtle flex items-center justify-between select-none">
              <span>Slash commands & skills</span>
              <span className="font-mono text-[10px]">{filteredSlash.length} available</span>
            </div>
            <div className="py-0.5">
              {filteredSlash.map((it, i) => (
                <button
                  key={it.name}
                  type="button"
                  className={cn(
                    'flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-xs rounded-lg transition-colors cursor-pointer select-none',
                    i === slashIndex
                      ? 'bg-primary/10 text-primary dark:bg-card/60'
                      : 'hover:bg-surface text-foreground'
                  )}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    applySlash(it);
                  }}
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="font-mono font-semibold text-foreground">{it.name}</span>
                    <span className="min-w-0 truncate text-foreground-subtle">{it.description}</span>
                  </span>
                  <span
                    className={cn(
                      'shrink-0 rounded px-1.5 py-0.5 text-[10px] font-mono font-medium uppercase',
                      it.category === 'skill'
                        ? 'bg-sky-500/15 text-sky-500 border border-sky-500/20'
                        : it.category === 'goal'
                          ? 'bg-emerald-500/15 text-emerald-500 border border-emerald-500/20'
                          : it.category === 'plan'
                            ? 'bg-purple-500/15 text-purple-500 border border-purple-500/20'
                            : it.category === 'security'
                              ? 'bg-amber-500/15 text-amber-500 border border-amber-500/20'
                              : it.category === 'mode'
                                ? 'bg-pink-500/15 text-pink-500 border border-pink-500/20'
                                : it.category === 'mcp' || it.category === 'mcp-tool'
                                  ? 'bg-violet-500/15 text-violet-500 border border-violet-500/20'
                                  : 'bg-surface text-foreground-subtle border border-border'
                    )}
                  >
                    {it.category}
                  </span>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Text Input Area */}
        <textarea
          ref={textareaRef}
          rows={isCompact ? 1 : 2}
          autoFocus={autoFocus}
          value={prompt}
          aria-label={placeholder === 'Ask anything, @ threads or files, $ skills, or / commands' ? 'Message the agent' : placeholder}
          onChange={(e) => {
            setPrompt(e.target.value);
            updateSlashState();
            updateAtState();
          }}
          onKeyDown={handleKeyDown}
          onPaste={async (e) => {
            const items = e.clipboardData?.items;
            let hasImage = false;
            if (items) {
              const imageFiles: File[] = [];
              for (let i = 0; i < items.length; i++) {
                const item = items[i];
                if (item.type.startsWith('image/')) {
                  const blob = item.getAsFile();
                  if (blob) {
                    hasImage = true;
                    imageFiles.push(blob);
                  }
                }
              }
              if (imageFiles.length > 0) {
                e.preventDefault();
                void processFilesForAttachment(imageFiles);
                return;
              }
            }

            if (!hasImage) {
              try {
                const files = await GetClipboardFiles().catch(() => []);
                if (files && files.length > 0) {
                  e.preventDefault();
                  void processFilesForAttachment(files);
                }
              } catch {}
            }
          }}
          placeholder={isRunning ? "Type instruction to steer or queue for running agent..." : placeholder}
          className="w-full resize-none bg-transparent border-0 font-sans text-[length:var(--app-font-size-chat,13px)] leading-[1.4] text-foreground placeholder:text-muted-foreground/45 focus:outline-hidden px-1"
        />

        {/* Bottom Bar: [+] [Mode] ────── [Context] [Provider + Model + Effort ⌄] [↑] */}
        <div className="flex min-w-0 items-center justify-between gap-1.5 pt-0.5">

          {/* Left Controls: [+] [Permission Mode] */}
          <div className="flex min-w-0 shrink items-center gap-1 relative">
            
            {/* [+] Button */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsContextOpen(prev => !prev);
                  setActiveSubMenu(null);
                  setIsModeOpen(false);
                  setIsModelPickerOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isContextOpen}
                aria-label="Add context"
                className="flex size-7 shrink-0 items-center justify-center rounded-lg text-muted-foreground hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground transition-colors cursor-pointer"
                title="Add context (@)"
              >
                <Plus className="size-4" aria-hidden="true" />
              </button>

              {/* Context Popover */}
              {isContextOpen && (
                <div role="menu" aria-label="Add context" className="absolute left-0 bottom-full mb-2 w-56 rounded-xl bg-popover shadow-xl border border-border p-1 z-50 text-xs text-foreground animate-in fade-in zoom-in-95 duration-100">
                  {/* Files Submenu */}
                  <div className="relative">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('files')}
                      onClick={() => setActiveSubMenu(prev => prev === 'files' ? null : 'files')}
                      className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center justify-between cursor-pointer transition-colors"
                    >
                      <div className="flex items-center gap-2">
                        <FileText className="size-3.5 text-foreground-subtle" />
                        <span>Files</span>
                      </div>
                      <ChevronRight className="size-3 text-foreground-subtlest" />
                    </button>

                    {activeSubMenu === 'files' && (
                      <div className="absolute left-full top-0 ml-1 w-52 rounded-xl bg-popover shadow-xl border border-border p-1 z-50 max-h-56 overflow-y-auto">
                        {files.filter(f => f.type === 'file').length === 0 ? (
                          <div className="px-2.5 py-2 text-xs text-foreground-subtlest">No files loaded</div>
                        ) : (
                          files.filter(f => f.type === 'file').slice(0, 15).map(f => (
                            <button
                              key={f.id}
                              type="button"
                              onClick={() => insertContextTag(`file:${f.name}`)}
                              className="w-full text-left px-2.5 py-1.5 rounded-lg text-xs text-foreground-subtle hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground flex items-center gap-2 truncate cursor-pointer font-mono"
                            >
                              <FileText className="size-3 text-success shrink-0" />
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
                    className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Bot className="size-3.5 text-foreground-subtle" />
                    <span>Skills ({skills.length})</span>
                  </button>

                  {/* MCP Tools */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('mcp')}
                    className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Wrench className="size-3.5 text-foreground-subtle" />
                    <span>MCP Tools ({mcps.length})</span>
                  </button>

                  {/* Git */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('git')}
                    className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <GitBranch className="size-3.5 text-foreground-subtle" />
                    <span>Git Status & Diff</span>
                  </button>

                  {/* Terminal */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('terminal')}
                    className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Terminal className="size-3.5 text-foreground-subtle" />
                    <span>Terminal</span>
                  </button>

                  <div className="h-px bg-border my-1" />

                  {/* Attach Local File */}
                  <button
                    type="button"
                    onClick={handleNativeFileUpload}
                    className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center gap-2 cursor-pointer transition-colors"
                  >
                    <Upload className="size-3.5 text-foreground-subtle" />
                    <span>Attach Local File</span>
                  </button>
                </div>
              )}
            </div>

            {/* Permission Mode Selector Badge */}
            <div className="relative flex min-w-0 items-center">
              <button
                type="button"
                onClick={() => {
                  setIsModeOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsModelPickerOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isModeOpen}
                aria-label={`Permission mode: ${currentMode.label}`}
                className={`flex h-7 min-w-0 items-center gap-1.5 rounded-lg px-2 text-[length:var(--app-font-size-ui,12px)] font-normal transition-colors cursor-pointer hover:bg-[var(--color-background-button-secondary-hover)] ${
                  isFullAccess ? 'text-[var(--runtime-full-access-accent)]' : 'text-foreground-secondary hover:text-foreground'
                }`}
              >
                <currentMode.icon className="size-3.5 shrink-0" aria-hidden="true" />
                <span className="min-w-0 truncate">{currentMode.label}</span>
                <ChevronDown className={`size-3 shrink-0 opacity-60 ${isFullAccess ? 'text-[var(--runtime-full-access-accent)]' : ''}`} aria-hidden="true" />
              </button>

              {/* Mode Dropdown Menu */}
              {isModeOpen && (
                <div role="menu" aria-label="Permission mode" className="absolute left-0 bottom-full mb-2 w-64 rounded-xl bg-popover shadow-xl border border-border p-1 z-50 text-xs animate-in fade-in zoom-in-95 duration-100">
                  <div className="px-2.5 py-1 text-ui-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    Permission mode
                  </div>
                  {MODES.map(m => {
                    const isSelected = m.id === agentExecutionMode;
                    const isModeFull = m.id === 'bypass';
                    return (
                      <button
                        key={m.id}
                        type="button"
                        role="menuitemradio"
                        aria-checked={isSelected}
                        onClick={() => {
                          setAgentExecutionMode(m.id);
                          setIsModeOpen(false);
                        }}
                        className={`w-full text-left px-2.5 py-1.5 rounded-lg flex items-start gap-2.5 hover:bg-[var(--color-background-button-secondary-hover)] cursor-pointer transition-colors ${
                          isSelected ? 'bg-[var(--color-background-button-secondary)]' : ''
                        } ${isModeFull ? 'text-[var(--runtime-full-access-accent)]' : 'text-foreground'}`}
                      >
                        <m.icon className="size-4 shrink-0 mt-0.5 opacity-80" aria-hidden="true" />
                        <div className="flex-1 min-w-0">
                          <div className="font-medium text-xs flex items-center justify-between">
                            <span>{m.label}</span>
                            {isSelected && <Check className="size-3.5 text-success" aria-hidden="true" />}
                          </div>
                          <p className={`text-ui-xs leading-snug mt-0.5 ${isModeFull ? 'text-current opacity-80' : 'text-muted-foreground'}`}>
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

          {/* Right Controls: [Context Usage Gauge] [Provider + Model + Effort ⌄] [Send] */}
          <div className="flex min-w-0 shrink items-center gap-1.5 relative">

            {/* Context Window Usage Gauge */}
            <button
              type="button"
              onClick={() => {
                setIsUsageContextOpen(prev => !prev);
                setIsModelPickerOpen(false);
                setIsModeOpen(false);
                setIsContextOpen(false);
              }}
              aria-label={`Context window usage: ${displayUsage.formattedUsed} of ${displayUsage.formattedMax} tokens, ${displayUsage.percent.toFixed(1)} percent`}
              aria-expanded={isUsageContextOpen}
              className="size-6 rounded-full flex items-center justify-center cursor-pointer hover:bg-[var(--color-background-button-secondary-hover)] transition-colors shrink-0"
              title={`Context: ${displayUsage.formattedUsed}/${displayUsage.formattedMax} (${displayUsage.percent.toFixed(1)}%)`}
            >
              <svg className="size-3.5 -rotate-90" viewBox="0 0 36 36" aria-hidden="true">
                <path className="text-border" strokeWidth="5" stroke="currentColor" fill="none"
                  d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
                <path className="text-success" strokeWidth="5" strokeLinecap="round" stroke="currentColor" fill="none"
                  strokeDasharray={`${Math.max(displayUsage.percent, displayUsage.usedTokens > 0 ? 1 : 0).toFixed(1)}, 100`}
                  d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
              </svg>
            </button>

            {isUsageContextOpen && (
              <div className="absolute right-0 bottom-full mb-2 w-60 rounded-xl bg-popover shadow-xl border border-border p-3 z-50 text-xs animate-in fade-in zoom-in-95 duration-100">
                <div className="flex flex-col gap-1">
                  <div className="text-ui-xs font-medium text-muted-foreground">Context window</div>
                  <div className="flex items-baseline justify-between">
                    <span className="text-sm font-mono font-semibold text-foreground">
                      {displayUsage.formattedUsed}<span className="text-muted-foreground font-normal text-xs">/{displayUsage.formattedMax}</span>
                    </span>
                    <span className="text-xs font-mono font-medium text-success">
                      ({displayUsage.percent.toFixed(1)}%)
                    </span>
                  </div>
                  <div className="w-full h-1.5 bg-[var(--color-background-button-secondary)] rounded-full overflow-hidden mt-1">
                    <div className="h-full bg-success rounded-full transition-all duration-300"
                      style={{ width: `${Math.min(Math.max(displayUsage.percent, displayUsage.usedTokens > 0 ? 1 : 0), 100)}%` }} />
                  </div>
                </div>
              </div>
            )}

            {/* Provider + Model + Effort Compound Picker (Kybern Exact Style) */}
            <div className="relative flex min-w-0 items-center" onKeyDown={(e) => { if (e.key === 'Escape') { setIsModelPickerOpen(false); setActiveModelSubMenu(null); } }}>
              <button
                type="button"
                onClick={() => {
                  setIsModelPickerOpen(prev => !prev);
                  setActiveModelSubMenu(null);
                  setIsUsageContextOpen(false);
                  setIsModeOpen(false);
                  setIsContextOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isModelPickerOpen}
                aria-label={`Model: ${currentModel || 'ForgeADE'}, effort ${currentReasoning.label}, agent ForgeADE`}
                className="flex h-7 min-w-0 items-center gap-1.5 rounded-lg px-2 text-[length:var(--app-font-size-ui,12px)] font-normal text-foreground hover:bg-[var(--color-background-button-secondary-hover)] transition-colors cursor-pointer"
                title={`ForgeADE · ${currentModel || 'Model'} · ${currentReasoning.label}`}
              >
                <ProviderIcon provider="agent-internal" size={14} className="size-3.5 shrink-0 text-foreground" />
                <span className="min-w-0 truncate max-w-[110px] @[40rem]:max-w-[150px] font-normal leading-none">{currentModel || 'ForgeADE'}</span>
                <span className="shrink-0 capitalize leading-none text-muted-foreground/60 hidden @[32rem]:inline-block">
                  {currentReasoning.label}
                </span>
                <ChevronDown className="size-3 shrink-0 opacity-60" aria-hidden="true" />
              </button>

              {/* Kybern Model/Effort/Provider Picker Popup */}
              {isModelPickerOpen && (
                <div role="menu" aria-label="Model, effort and agent" className="absolute right-0 bottom-full mb-2 w-64 rounded-xl bg-popover shadow-xl border border-border p-1 z-50 text-xs animate-in fade-in zoom-in-95 duration-100">

                  {/* Section 1: Effort Radio Group */}
                  <div className="px-2.5 py-1 text-ui-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    Effort
                  </div>
                  <div className="flex flex-col gap-0.5">
                    {harnessReasoningLevels.map(r => {
                      const isSelected = r.id === currentReasoning.id;
                      return (
                        <button
                          key={r.id}
                          type="button"
                          role="menuitemradio"
                          aria-checked={isSelected}
                          onClick={() => {
                            setReasoningLevel(r.id);
                          }}
                          className={`w-full text-left px-2.5 py-1.5 rounded-lg flex items-center justify-between hover:bg-[var(--color-background-button-secondary-hover)] cursor-pointer transition-colors ${
                            isSelected ? 'bg-[var(--color-background-button-secondary)] text-foreground font-medium' : 'text-foreground/80'
                          }`}
                        >
                          <span className="capitalize">{r.label}</span>
                          {isSelected && <Check className="size-3.5 text-success" aria-hidden="true" />}
                        </button>
                      );
                    })}
                  </div>

                  <div className="h-px bg-border my-1" />

                  {/* Section 2: Model Submenu */}
                  <div className="relative">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveModelSubMenu('model')}
                      onClick={() => setActiveModelSubMenu(prev => prev === 'model' ? null : 'model')}
                      className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center justify-between cursor-pointer transition-colors text-foreground"
                    >
                      <div className="flex items-center gap-2 truncate">
                        <ProviderIcon provider="agent-internal" size={14} className="size-3.5 shrink-0" />
                        <span className="truncate">{currentModel || 'Model'}</span>
                      </div>
                      <ChevronRight className="size-3 text-foreground-subtlest" />
                    </button>

                    {activeModelSubMenu === 'model' && (
                      <div className="absolute right-full bottom-0 mr-1 w-64 rounded-xl bg-popover shadow-xl border border-border p-1 z-50 max-h-80 overflow-y-auto">
                        <div className="px-2.5 py-1 text-ui-xs font-semibold text-muted-foreground uppercase tracking-wider">
                          Select Model
                        </div>
                        {/* Grouped by provider — mirror of the header model
                            picker, so models never float without context. */}
                        {providers.filter(p => p.enabled).map(p => {
                          const models = (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models) || [];
                          if (models.length === 0) return null;
                          return (
                            <div key={p.id} className="mb-0.5">
                              <div className="px-2.5 py-1 text-ui-xs font-semibold uppercase tracking-wider text-muted-foreground">
                                {p.name}
                              </div>
                              {models.map((m: string) => {
                                const isSelected = m === currentModel;
                                return (
                                  <button
                                    key={`${p.id}-${m}`}
                                    type="button"
                                    onClick={() => {
                                      setCurrentModel(m);
                                      // Point the Go harness at this provider
                                      // so the turn authenticates with it.
                                      SetActiveModel(p.id, m).catch(() => {});
                                      setActiveModelSubMenu(null);
                                    }}
                                    className={`w-full text-left px-2.5 py-1.5 rounded-lg flex items-center justify-between hover:bg-[var(--color-background-button-secondary-hover)] cursor-pointer font-mono text-xs transition-colors ${
                                      isSelected ? 'bg-[var(--color-background-button-secondary)] text-foreground font-medium' : 'text-foreground/80'
                                    }`}
                                  >
                                    <span className="truncate">{m}</span>
                                    {isSelected && <Check className="size-3.5 text-success" />}
                                  </button>
                                );
                              })}
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>

                  {/* Section 3: Agent — fixed to the internal ForgeADE harness */}
                  <div className="flex items-center justify-between px-2.5 py-1.5">
                    <div className="flex items-center gap-2 truncate">
                      <Bot className="size-3.5 shrink-0 text-muted-foreground" />
                      <span className="truncate">Agent: ForgeADE</span>
                    </div>
                    <span className="shrink-0 rounded bg-[var(--color-background-button-secondary)] px-1.5 py-0.5 text-ui-xs text-muted-foreground">internal</span>
                  </div>

                </div>
              )}
            </div>

            {/* Kybern Action Buttons: Send / Steer / Queue / Stop */}
            {isRunning ? (
              <div className="flex items-center gap-1.5 shrink-0">
                {prompt.trim() && (
                  <>
                    <button
                      type="button"
                      onClick={handleQueueSteer}
                      aria-label="Queue steering instruction"
                      className="flex h-8 items-center gap-1 px-2.5 rounded-full bg-[var(--color-background-button-secondary)] text-foreground hover:bg-[var(--color-background-button-secondary-hover)] transition-colors cursor-pointer text-ui-xs font-medium border border-border"
                      title="Add to steering stack (Enter)"
                    >
                      <Plus className="size-3.5" />
                      <span>Queue</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleSteer()}
                      aria-label="Steer agent now"
                      className="flex h-8 items-center gap-1.5 px-3 rounded-full bg-accent-primary text-white hover:opacity-90 transition-opacity cursor-pointer text-ui-xs font-medium shadow-sm"
                      title="Steer running agent immediately (Cmd+Enter)"
                    >
                      <Compass className="size-3.5" />
                      <span>Steer</span>
                    </button>
                  </>
                )}
                <button
                  type="button"
                  onClick={stopAgentExecution}
                  aria-label="Stop execution"
                  className="flex size-8 shrink-0 items-center justify-center rounded-full bg-destructive text-white hover:opacity-90 transition-opacity cursor-pointer shadow-sm"
                  title="Stop execution"
                >
                  <Square className="size-3.5 fill-current" aria-hidden="true" />
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={!prompt.trim() && attachedFiles.length === 0}
                aria-label="Send message"
                className={`flex size-8 shrink-0 items-center justify-center rounded-full transition-colors cursor-pointer shadow-sm ${
                  !prompt.trim() && attachedFiles.length === 0
                    ? 'bg-[var(--color-background-button-secondary)] text-muted-foreground/35 cursor-not-allowed'
                    : 'bg-foreground text-background hover:opacity-90'
                }`}
                title="Send"
              >
                <ArrowUp className="size-4" strokeWidth={2.5} aria-hidden="true" />
              </button>
            )}

          </div>

        </div>

      </div>
    </div>
  );
};

export default AgentTaskInputBar;
