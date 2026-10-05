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
  Folder
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AtMentionMenu, flattenFileTree, extractAtQuery, type AtMentionItem } from './AtMentionMenu';
import { AcpGetSlashCommands, SetActiveModel, GetClipboardFiles, type AcpSlashCommandItem } from '../../lib/wails';
import { ProviderIcon } from './ProviderIcon';
import { AgentExecutionMode, AgentReasoningLevel } from '../../types';

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
    createNewSession,
    activeSession,
    stopAgentExecution,
    files,
    skills,
    mcps,
    contextUsage,
    agents,
    activeAgentId,
    setActiveAgentId,
    refreshAgentModels
  } = useWorkspace();

  const [isReloadingModels, setIsReloadingModels] = useState(false);
  const [prompt, setPrompt] = useState('');
  const [attachedFiles, setAttachedFiles] = useState<{ name: string; path?: string; isImage?: boolean }[]>([]);
  const [isContextOpen, setIsContextOpen] = useState(false);
  const [activeSubMenu, setActiveSubMenu] = useState<string | null>(null);
  const [isModeOpen, setIsModeOpen] = useState(false);
  const [isModelPickerOpen, setIsModelPickerOpen] = useState(false);
  const [activeModelSubMenu, setActiveModelSubMenu] = useState<'model' | 'agent' | null>(null);
  const [isUsageContextOpen, setIsUsageContextOpen] = useState(false);

  // @-mention: opens while the text before the caret has "@<query>".
  const [atOpen, setAtOpen] = useState(false);
  const [atItems, setAtItems] = useState<AtMentionItem[]>([]);
  const [atIndex, setAtIndex] = useState(0);

  // Slash menu: opens while the text before the caret starts with "/".
  const [slashItems, setSlashItems] = useState<AcpSlashCommandItem[]>([]);
  const [slashOpen, setSlashOpen] = useState(false);
  const [slashQuery, setSlashQuery] = useState('');
  const [slashIndex, setSlashIndex] = useState(0);
  const slashLoadedRef = useRef(false);

  const containerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const isRunning = activeSession?.status === 'running';

  const currentAgent = useMemo(() => {
    return agents.find(a => a.id === activeAgentId) || agents[0] || {
      id: 'default',
      name: 'Default Agent',
      type: 'internal',
      provider: 'forge-ade'
    };
  }, [agents, activeAgentId]);

  const isAcpAgent = Boolean(currentAgent && currentAgent.type !== 'internal');

  const currentMode = useMemo(() => {
    return MODES.find(m => m.id === agentExecutionMode) || MODES[0];
  }, [agentExecutionMode]);

  const harnessReasoningLevels = useMemo(() => {
    if (currentAgent.supportedEfforts && currentAgent.supportedEfforts.length > 0) {
      return ALL_REASONING_LEVELS.filter(r => currentAgent.supportedEfforts!.includes(r.id));
    }
    return ALL_REASONING_LEVELS.filter(r => ['low', 'medium', 'high'].includes(r.id));
  }, [currentAgent]);

  const currentReasoning = useMemo(() => {
    return harnessReasoningLevels.find(r => r.id === reasoningLevel) || harnessReasoningLevels[0] || ALL_REASONING_LEVELS[3];
  }, [harnessReasoningLevels, reasoningLevel]);

  // Group models from current agent or providers.
  // External agents advertise their own model list; the internal harness lists
  // every model selected in Settings → Model settings (enabled providers) —
  // never the static supportedModels baked into the agent record.
  const availableModels = useMemo(() => {
    if (currentAgent.type && currentAgent.type !== 'internal') {
      if (currentAgent.supportedModels && currentAgent.supportedModels.length > 0) {
        return currentAgent.supportedModels;
      }
    }
    const list: string[] = [];
    const seen = new Set<string>();
    for (const p of providers) {
      if (!p.enabled) continue;
      const models = (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models) || [];
      for (const m of models) {
        if (!seen.has(m)) {
          seen.add(m);
          list.push(m);
        }
      }
    }
    if (list.length > 0) return list;
    return ['claude-3-7-sonnet', 'gpt-4o', 'gemini-2.5-pro'];
  }, [currentAgent, providers]);

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

  const handleSubmit = () => {
    if (!prompt.trim() && attachedFiles.length === 0) return;
    if (isRunning) return;

    if (onSubmitPrompt) {
      onSubmitPrompt(prompt);
    } else if (activeSession) {
      sendAgentPrompt(prompt);
    } else {
      createNewSession(prompt);
    }

    setPrompt('');
    setAttachedFiles([]);
  };

  // Load the slash catalog once per agent (external: backend catalog;
  // internal: skills + MCP servers/tools from the workspace store).
  useEffect(() => {
    if (!isAcpAgent) return;
    if (slashLoadedRef.current) return;
    slashLoadedRef.current = true;
    AcpGetSlashCommands(activeAgentId)
      .then((items) => setSlashItems(items || []))
      .catch(() => setSlashItems([]));
  }, [isAcpAgent, activeAgentId]);

  const internalSlashItems = useMemo(() => {
    const items: AcpSlashCommandItem[] = [{ name: '/skill', description: 'Invoke a skill (/skill <name>)', category: 'command' }];
    for (const s of skills || []) {
      items.push({ name: `/skill:${s.name}`, description: s.description || `Run ${s.name} skill`, category: 'skill' });
    }
    for (const m of mcps || []) {
      const name = typeof m === 'string' ? m : m?.name;
      if (name) items.push({ name: `/mcp:${name}`, description: `MCP server: ${name}`, category: 'mcp' });
    }
    return items;
  }, [skills, mcps]);

  const slashCatalog = useMemo(
    () => (isAcpAgent ? slashItems : internalSlashItems),
    [isAcpAgent, slashItems, internalSlashItems]
  );

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

  const applySlash = (item: AcpSlashCommandItem) => {
    const el = textareaRef.current;
    if (!el) return;
    const pos = el.selectionStart ?? el.value.length;
    const upto = el.value.slice(0, pos);
    const startIdx = upto.lastIndexOf('/');
    if (startIdx === -1) return;
    const next = el.value.slice(0, startIdx) + item.name + ' ' + el.value.slice(pos);
    setPrompt(next);
    setSlashOpen(false);
    requestAnimationFrame(() => {
      el.focus();
      const caret = startIdx + item.name.length + 1;
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
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleNativeFileUpload = () => {
    fileInputRef.current?.click();
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const filesList = e.target.files;
    if (!filesList) return;
    for (let i = 0; i < filesList.length; i++) {
      const file = filesList[i];
      setAttachedFiles(prev => [
        ...prev,
        {
          name: file.name,
          path: (file as any).path || file.name,
          isImage: file.type.startsWith('image/')
        }
      ]);
    }
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
      const rect = el.getBoundingClientRect();
      const inside =
        detail.x >= rect.left &&
        detail.x <= rect.right &&
        detail.y >= rect.top &&
        detail.y <= rect.bottom;
      if (!inside) return;

      ev.preventDefault();
      ev.stopPropagation();
      const quoted = detail.files
        .map((p: string) => `'${p}'`)
        .join(' ');
      setPrompt((prev) => (prev ? `${prev} ${quoted}` : quoted));

      const newAttachments = detail.files.map((p: string) => {
        const name = p.split(/[/\\]/).pop() || p;
        const isImage = /\.(png|jpe?g|gif|webp|svg|ico|bmp)$/i.test(p);
        return { name, path: p, isImage };
      });
      setAttachedFiles((prev) => [...prev, ...newAttachments]);
      textareaRef.current?.focus();
    };

    window.addEventListener('forge:file-drop', handleForgeDrop);
    return () => window.removeEventListener('forge:file-drop', handleForgeDrop);
  }, []);

  const isFullAccess = agentExecutionMode === 'bypass';

  return (
    <div
      className={`w-full relative select-none mx-auto @container/composer ${fillWidth ? 'max-w-none px-1' : 'max-w-[var(--app-chat-max-width,48rem)]'}`}
      ref={containerRef}
      data-file-drop-target="true"
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

      {/* Main Kybern Composer Squircle Container */}
      <div className="chat-composer-surface border border-[color:var(--surface-border)] p-2.5 sm:p-3 flex flex-col gap-2 relative">
        
        {/* Attached Files Badges */}
        {attachedFiles.length > 0 && (
          <div className="flex flex-wrap gap-1.5 pb-1">
            {attachedFiles.map((file, idx) => (
              <div 
                key={idx} 
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-[var(--color-background-button-secondary)] text-foreground text-ui-xs font-mono border border-border"
                title={file.path || file.name}
              >
                {file.isImage ? (
                  <ImageIcon className="size-3.5 text-info shrink-0" />
                ) : (
                  <FileText className="size-3.5 text-success shrink-0" />
                )}
                <span className="truncate max-w-[160px]">{file.name}</span>
                <button
                  type="button"
                  onClick={() => setAttachedFiles(prev => prev.filter((_, i) => i !== idx))}
                  className="text-foreground-subtle hover:text-destructive ml-0.5 cursor-pointer transition-colors"
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
          <div className="absolute bottom-full left-3 right-3 mb-1 max-h-56 overflow-y-auto rounded-xl border border-border bg-popover shadow-2xl z-40">
            {filteredSlash.map((it, i) => (
              <button
                key={it.name}
                type="button"
                className={`flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-xs transition-colors ${
                  i === slashIndex
                    ? 'bg-primary/10 dark:bg-card/60'
                    : 'hover:bg-surface dark:hover:bg-surface-hover'
                }`}
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
                  className={`shrink-0 rounded px-1.5 py-0.5 text-ui-xs font-medium uppercase ${
                    it.category === 'skill'
                      ? 'bg-sky-500/15 text-sky-500'
                      : it.category === 'mcp' || it.category === 'mcp-tool'
                        ? 'bg-violet-500/15 text-violet-500'
                        : 'bg-surface text-foreground-subtle'
                  }`}
                >
                  {it.category}
                </span>
              </button>
            ))}
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
            try {
              const files = await GetClipboardFiles().catch(() => []);
              if (files && files.length > 0) {
                e.preventDefault();
                const quoted = files.map((p) => `'${p}'`).join(' ');
                const el = textareaRef.current;
                if (el) {
                  const start = el.selectionStart ?? el.value.length;
                  const end = el.selectionEnd ?? el.value.length;
                  const next = el.value.slice(0, start) + quoted + ' ' + el.value.slice(end);
                  setPrompt(next);
                }
              }
            } catch {}
          }}
          placeholder={placeholder}
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
                aria-label={`Model: ${currentModel || currentAgent.name}, effort ${currentReasoning.label}, agent ${currentAgent.name}`}
                className="flex h-7 min-w-0 items-center gap-1.5 rounded-lg px-2 text-[length:var(--app-font-size-ui,12px)] font-normal text-foreground hover:bg-[var(--color-background-button-secondary-hover)] transition-colors cursor-pointer"
                title={`${currentAgent.name} · ${currentModel || 'Model'} · ${currentReasoning.label}`}
              >
                <ProviderIcon provider={currentAgent.id || currentAgent.name} size={14} className="size-3.5 shrink-0 text-foreground" />
                <span className="min-w-0 truncate max-w-[110px] @[40rem]:max-w-[150px] font-normal leading-none">{currentModel || currentAgent.name}</span>
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
                        <ProviderIcon provider={currentAgent.id} size={14} className="size-3.5 shrink-0" />
                        <span className="truncate">{currentModel || 'Model'}</span>
                      </div>
                      <ChevronRight className="size-3 text-foreground-subtlest" />
                    </button>

                    {activeModelSubMenu === 'model' && (
                      <div className="absolute right-full bottom-0 mr-1 w-64 rounded-xl bg-popover shadow-xl border border-border p-1 z-50 max-h-80 overflow-y-auto">
                        <div className="px-2.5 py-1 text-ui-xs font-semibold text-muted-foreground uppercase tracking-wider">
                          Select Model
                        </div>
                        {availableModels.map((m: string) => {
                          const isSelected = m === currentModel;
                          return (
                            <button
                              key={m}
                              type="button"
                              onClick={() => {
                                setCurrentModel(m);
                                // Point the Go harness at the provider owning
                                // this model so the turn authenticates with it.
                                if (!currentAgent.type || currentAgent.type === 'internal') {
                                  const owner = providers.find(p => p.enabled && ((p.selectedModels || p.models) || []).includes(m));
                                  if (owner) SetActiveModel(owner.id, m).catch(() => {});
                                }
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
                    )}
                  </div>

                  {/* Section 3: Agent Harness Submenu */}
                  <div className="relative">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveModelSubMenu('agent')}
                      onClick={() => setActiveModelSubMenu(prev => prev === 'agent' ? null : 'agent')}
                      className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] flex items-center justify-between cursor-pointer transition-colors text-foreground"
                    >
                      <div className="flex items-center gap-2 truncate">
                        <Bot className="size-3.5 shrink-0 text-muted-foreground" />
                        <span className="truncate">Agent: {currentAgent.name}</span>
                      </div>
                      <ChevronRight className="size-3 text-foreground-subtlest" />
                    </button>

                    {activeModelSubMenu === 'agent' && (
                      <div className="absolute right-full bottom-0 mr-1 w-64 rounded-xl bg-popover shadow-xl border border-border p-1 z-50 max-h-80 overflow-y-auto">
                        <div className="px-2.5 py-1 text-ui-xs font-semibold text-muted-foreground uppercase tracking-wider">
                          Agent Harness
                        </div>
                        {agents.map(ag => {
                          const isSelected = ag.id === currentAgent.id;
                          return (
                            <button
                              key={ag.id}
                              type="button"
                              onClick={() => {
                                setActiveAgentId(ag.id);
                                // External agents pin their advertised models;
                                // internal uses the Settings provider list.
                                if (ag.type && ag.type !== 'internal' && ag.supportedModels && ag.supportedModels.length > 0) {
                                  if (!ag.supportedModels.includes(currentModel)) {
                                    setCurrentModel(ag.supportedModels[0]);
                                  }
                                }
                                if (ag.defaultEffort) {
                                  setReasoningLevel(ag.defaultEffort);
                                }
                                setActiveModelSubMenu(null);
                              }}
                              className={`w-full text-left px-2.5 py-1.5 rounded-lg flex items-center justify-between hover:bg-[var(--color-background-button-secondary-hover)] cursor-pointer transition-colors ${
                                isSelected ? 'bg-[var(--color-background-button-secondary)] text-foreground font-medium' : 'text-foreground/80'
                              }`}
                            >
                              <div className="flex items-center gap-2 truncate">
                                <ProviderIcon provider={ag.id} size={14} className="size-3.5 shrink-0" />
                                <span className="truncate">{ag.name}</span>
                              </div>
                              {isSelected && <Check className="size-3.5 text-success" />}
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </div>

                  <div className="h-px bg-border my-1" />

                  {/* Section 4: Reload Models Action */}
                  <button
                    type="button"
                    disabled={isReloadingModels}
                    onClick={async (e) => {
                      e.stopPropagation();
                      setIsReloadingModels(true);
                      try {
                        await refreshAgentModels(currentAgent.id);
                      } finally {
                        setIsReloadingModels(false);
                      }
                    }}
                    className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[var(--color-background-button-secondary-hover)] cursor-pointer flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {isReloadingModels ? (
                      <Loader2 className="size-3.5 animate-spin" />
                    ) : (
                      <RefreshCw className="size-3.5" />
                    )}
                    <span>{isReloadingModels ? 'Reloading…' : 'Reload models'}</span>
                  </button>

                </div>
              )}
            </div>

            {/* Kybern Round 32px Send Circle Button */}
            {isRunning ? (
              <button
                type="button"
                onClick={stopAgentExecution}
                aria-label="Stop execution"
                className="flex size-8 shrink-0 items-center justify-center rounded-full bg-destructive text-white hover:opacity-90 transition-opacity cursor-pointer shadow-sm"
                title="Stop execution"
              >
                <Square className="size-3.5 fill-current" aria-hidden="true" />
              </button>
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
