import React, { useState, useRef, useEffect, useMemo } from 'react';
import {
  Plus,
  ChevronRight,
  FileText,
  GitBranch,
  Terminal,
  Upload,
  ArrowUp,
  Square,
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
  CornerDownLeft,
  GripVertical,
  Trash2,
  Minimize2,
  Archive
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { useComposerDraftStore } from '../../stores/composerDraftStore';
import { usePromptStashStore } from '../../stores/promptStashStore';
import { cn } from '../../lib/utils';
import { AtMentionMenu, flattenFileTree, extractAtQuery, type AtMentionItem } from './AtMentionMenu';
import { SetActiveModel, SetActiveReasoningEffort, GetClipboardFiles } from '../../lib/wails';
import { ApiBridge } from '../../services/apiBridge';
import { ProviderIcon } from './ProviderIcon';
import { WorkspaceSwitcherDropdown } from './WorkspaceSwitcherDropdown';
import { BranchSelector } from './BranchSelector';
import {
  ComposerChevron,
  ComposerControl,
  ComposerKbd,
  ComposerMenu,
  ComposerMenuCheck,
  ComposerMenuItem,
  ComposerMenuLabel,
} from '../common/composer-ui';
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
  /**
   * Attach the t3 context strip (workspace + branch chips) below the input.
   * The hero view opts out — its landing tray already carries those chips.
   */
  showContextStrip?: boolean;
}

const MODES: { id: AgentExecutionMode; label: string; short: string; desc: string; icon: any }[] = [
  {
    id: 'ask',
    label: 'Ask for approval',
    short: 'Ask',
    desc: 'Always ask before editing files or running commands',
    icon: Hand
  },
  {
    id: 'code',
    label: 'Approve edits',
    short: 'Edits',
    desc: 'Edit files freely, ask before running commands',
    icon: Pencil
  },
  {
    id: 'plan',
    label: 'Approve for me',
    short: 'Auto',
    desc: 'Only ask for actions detected as potentially unsafe',
    icon: ShieldCheck
  },
  {
    id: 'bypass',
    label: 'Full access',
    short: 'Full',
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
  fillWidth = false,
  showContextStrip = true
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
    compactCurrentSession,
    activeWorkspacePath,
    gitBranch,
    agents,
    activeAgentId,
    setActiveAgentId,
    activeAgent,
    refreshAgentModels,
    openSideFile
  } = useWorkspace();

  const [prompt, setPrompt] = useState('');
  const [steerQueue, setSteerQueue] = useState<string[]>([]);
  // ZCode-style queue rows: inline edit (pencil) and drag-to-reorder (grip).
  const [editingIdx, setEditingIdx] = useState<number | null>(null);
  const [editText, setEditText] = useState('');
  const dragFromIdx = useRef<number | null>(null);
  const [dragOverIdx, setDragOverIdx] = useState<number | null>(null);
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
  const [isAgentPickerOpen, setIsAgentPickerOpen] = useState(false);
  const [isModelPickerOpen, setIsModelPickerOpen] = useState(false);
  const [customModelInput, setCustomModelInput] = useState('');
  const [isEffortPickerOpen, setIsEffortPickerOpen] = useState(false);
  const [isUsageContextOpen, setIsUsageContextOpen] = useState(false);

  const draftKey = useMemo(() => {
    return activeSession?.id ? `session:${activeSession.id}` : `workspace:${activeWorkspacePath || 'default'}`;
  }, [activeSession?.id, activeWorkspacePath]);

  const [isStashMenuOpen, setIsStashMenuOpen] = useState(false);
  const stashEntries = usePromptStashStore(s => s.entries);
  const stashPrompt = usePromptStashStore(s => s.stashPrompt);
  const popPrompt = usePromptStashStore(s => s.popPrompt);
  const removePrompt = usePromptStashStore(s => s.removePrompt);
  const clearStash = usePromptStashStore(s => s.clearStash);

  const initialLoadedKey = useRef<string | null>(null);

  // Hydrate draft when switching session/workspace
  useEffect(() => {
    if (initialLoadedKey.current !== draftKey) {
      initialLoadedKey.current = draftKey;
      const draft = useComposerDraftStore.getState().getDraft(draftKey);
      setPrompt(draft.prompt || '');
      setAttachedFiles(draft.attachedFiles || []);
    }
  }, [draftKey]);

  // Persist draft on edit
  useEffect(() => {
    if (initialLoadedKey.current === draftKey) {
      useComposerDraftStore.getState().setDraft(draftKey, {
        prompt,
        attachedFiles: attachedFiles.map((f, i) => ({
          id: f.path || f.name || String(i),
          name: f.name,
          path: f.path,
          isImage: f.isImage,
          previewUrl: f.previewUrl,
          size: f.size
        }))
      });
    }
  }, [draftKey, prompt, attachedFiles]);

  const handleStashCurrent = () => {
    if (!prompt.trim() && attachedFiles.length === 0) return;
    stashPrompt(
      prompt,
      attachedFiles.map((f, i) => ({
        id: f.path || f.name || String(i),
        name: f.name,
        path: f.path,
        isImage: f.isImage,
        previewUrl: f.previewUrl,
        size: f.size
      }))
    );
    setPrompt('');
    setAttachedFiles([]);
    useComposerDraftStore.getState().clearDraft(draftKey);
  };

  const handleRestoreStash = (id: string) => {
    const item = popPrompt(id);
    if (item) {
      setPrompt(item.prompt || '');
      setAttachedFiles(item.attachments || []);
      setIsStashMenuOpen(false);
    }
  };

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

  // The internal harness exposes the standard effort ladder including off and max.
  const harnessReasoningLevels = useMemo(
    () => ALL_REASONING_LEVELS.filter(r => ['off', 'low', 'medium', 'high', 'max'].includes(r.id)),
    []
  );

  const currentReasoning = useMemo(() => {
    return harnessReasoningLevels.find(r => r.id === reasoningLevel) || harnessReasoningLevels.find(r => r.id === 'medium') || harnessReasoningLevels[0];
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
        setIsStashMenuOpen(false);
        setIsAgentPickerOpen(false);
        setIsModelPickerOpen(false);
        setIsEffortPickerOpen(false);
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

  // Sync the persisted reasoning effort to the harness whenever it changes — the
  // LLM request never asks for reasoning without it, so thinking rows would
  // never appear no matter what the model supports.
  useEffect(() => {
    SetActiveReasoningEffort(reasoningLevel).catch(() => {});
  }, [reasoningLevel]);

  const handleSteer = (instructionToSteer?: string) => {
    const text = (instructionToSteer ?? prompt).trim();
    if (!text) return;
    void steerAgentPrompt(text);
    if (!instructionToSteer) {
      setPrompt('');
      setAttachedFiles([]);
      useComposerDraftStore.getState().clearDraft(draftKey);
    }
  };

  const handleQueueSteer = () => {
    const text = prompt.trim();
    if (!text) return;
    setSteerQueue(prev => [...prev, text]);
    setPrompt('');
  };

  // --- Steering queue row helpers (ZCode-style tray) ------------------------

  const removeQueuedSteer = (idx: number) => {
    setSteerQueue(prev => prev.filter((_, i) => i !== idx));
    if (editingIdx === idx) setEditingIdx(null);
  };

  const startEditQueuedSteer = (idx: number) => {
    setEditingIdx(idx);
    setEditText(steerQueue[idx]);
  };

  const commitEditQueuedSteer = () => {
    if (editingIdx === null) return;
    const text = editText.trim();
    if (text) {
      setSteerQueue(prev => prev.map((item, i) => (i === editingIdx ? text : item)));
    }
    setEditingIdx(null);
  };

  const moveQueuedSteer = (from: number, to: number) => {
    if (from === to || to < 0 || to >= steerQueue.length) return;
    setSteerQueue(prev => {
      const next = [...prev];
      const [moved] = next.splice(from, 1);
      next.splice(to, 0, moved);
      return next;
    });
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
        const fileList = fileRefs.map(p => {
          let ref = p;
          if (activeWorkspacePath && activeWorkspacePath !== 'general' && p.startsWith(activeWorkspacePath)) {
            ref = p.slice(activeWorkspacePath.length).replace(/^[/\\]+/, '');
          }
          return `@${ref}`;
        }).join('\n');
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
    useComposerDraftStore.getState().clearDraft(draftKey);
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
      ...(skills || []).map((s) => {
        const slug = (s.trigger || s.name || '').trim().toLowerCase().replace(/\s+/g, '-');
        return {
          name: `/skill:${slug}`,
          description: s.description || (s.name !== slug ? `${s.name} (${slug})` : `Run ${slug} skill`),
          category: 'skill'
        };
      }),
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
        slashQuery ? (it?.name || '').toLowerCase().includes(slashQuery) : true
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
    if (e.altKey && e.key.toLowerCase() === 's') {
      e.preventDefault();
      handleStashCurrent();
      return;
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

  const handleNativeFileUpload = async () => {
    try {
      const picked = await ApiBridge.pickNativeFiles();
      if (picked && picked.length > 0) {
        await processFilesForAttachment(picked.map(p => p.path));
        setIsContextOpen(false);
        return;
      }
    } catch (e) {
      console.warn('Native file picker failed or cancelled:', e);
    }
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
        let finalPath = p;

        if (isImage) {
          try {
            const b64 = await ApiBridge.readFileBase64(p);
            if (b64) {
              const ext = name.split('.').pop()?.toLowerCase() || 'png';
              previewUrl = `data:image/${ext};base64,${b64}`;

              // Ensure a copy exists in workspace or general attachments folder
              const sanitizedName = name.replace(/[^a-zA-Z0-9._-]/g, '_');
              const fileName = `img_${Date.now()}_${sanitizedName}`;
              const saved = await ApiBridge.saveAttachment(fileName, b64);
              if (saved) {
                finalPath = saved;
              }
            }
          } catch (e) {
            console.warn('Error reading native image attachment:', e);
          }
        }
        newAttachments.push({ name, path: finalPath, isImage, previewUrl });
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
            if (previewUrl) {
              const comma = previewUrl.indexOf(',');
              const b64 = comma !== -1 ? previewUrl.slice(comma + 1) : previewUrl;
              const sanitizedName = file.name.replace(/[^a-zA-Z0-9._-]/g, '_');
              const fileName = `img_${Date.now()}_${sanitizedName}`;

              // Save to workspace or General attachments
              let savedPath = await ApiBridge.saveAttachment(fileName, b64);
              if (!savedPath && activeWorkspacePath && activeWorkspacePath !== 'general') {
                const fullPath = `${activeWorkspacePath}/.forge/attachments/${fileName}`;
                await ApiBridge.writeFileBase64(fullPath, b64);
                savedPath = fullPath;
              }
              targetPath = savedPath || targetPath || fileName;
            }
          } catch (err) {
            console.error('Failed to process image attachment:', err);
          }
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
  const chipSize = isCompact ? 'xs' : 'sm';

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

      {/* Stacked Mid-Turn Steering Queue Tray ABOVE input area (ZCode-style:
          grip rows — steer now, edit inline, delete, drag to reorder) */}
      {steerQueue.length > 0 && (
        <div className="mb-2 w-full rounded-2xl bg-panel/95 backdrop-blur-xs border border-border/80 shadow-md p-1.5 sm:p-2 flex flex-col gap-1 animate-in fade-in slide-in-from-bottom-2 duration-150">
          <div className="flex flex-col gap-1 max-h-52 overflow-y-auto pr-0.5">
            {steerQueue.map((item, idx) => (
              <div
                key={`${idx}-${item.slice(0, 12)}`}
                draggable={editingIdx !== idx}
                onDragStart={(e) => {
                  dragFromIdx.current = idx;
                  e.dataTransfer.effectAllowed = 'move';
                }}
                onDragOver={(e) => {
                  e.preventDefault();
                  e.dataTransfer.dropEffect = 'move';
                  if (dragFromIdx.current !== null && dragFromIdx.current !== idx) {
                    moveQueuedSteer(dragFromIdx.current, idx);
                    dragFromIdx.current = idx;
                  }
                  setDragOverIdx(idx);
                }}
                onDragEnd={() => {
                  dragFromIdx.current = null;
                  setDragOverIdx(null);
                }}
                onDrop={(e) => e.preventDefault()}
                className={cn(
                  'flex items-center justify-between gap-2.5 px-2.5 py-2 rounded-xl bg-[var(--color-background-button-secondary)] text-foreground border text-ui-xs group shadow-2xs',
                  dragOverIdx === idx && dragFromIdx.current !== null
                    ? 'border-accent-primary/60'
                    : 'border-transparent'
                )}
              >
                {/* Drag grip */}
                <span
                  className="shrink-0 cursor-grab active:cursor-grabbing text-foreground-subtlest group-hover:text-foreground-subtle transition-colors"
                  title="Drag to reorder"
                >
                  <GripVertical className="size-3.5" />
                </span>

                {/* Instruction text / inline editor */}
                {editingIdx === idx ? (
                  <input
                    type="text"
                    value={editText}
                    onChange={(e) => setEditText(e.target.value)}
                    onBlur={commitEditQueuedSteer}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault();
                        commitEditQueuedSteer();
                      } else if (e.key === 'Escape') {
                        e.preventDefault();
                        setEditingIdx(null);
                      }
                    }}
                    autoFocus
                    className="min-w-0 flex-1 bg-transparent border-0 text-ui-xs text-foreground focus:outline-none px-0.5"
                  />
                ) : (
                  <span
                    className="min-w-0 flex-1 text-ui-xs text-foreground font-normal break-words cursor-default"
                    onDoubleClick={() => startEditQueuedSteer(idx)}
                  >
                    {item}
                  </span>
                )}

                <div className="flex items-center gap-1 shrink-0">
                  {/* Steer now */}
                  <button
                    type="button"
                    onClick={() => {
                      handleSteer(item);
                      removeQueuedSteer(idx);
                    }}
                    className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-[var(--color-background-button-secondary)] border border-border text-foreground hover:bg-[var(--color-background-button-secondary-hover)] cursor-pointer font-medium text-ui-xs transition-colors"
                    title="Steer the running agent with this instruction now"
                  >
                    <ArrowUp className="size-3" strokeWidth={2.5} />
                    <span>Steer</span>
                  </button>
                  {/* Edit */}
                  <button
                    type="button"
                    onClick={() => startEditQueuedSteer(idx)}
                    className="size-6 rounded-full hover:bg-surface flex items-center justify-center text-foreground-subtle hover:text-foreground cursor-pointer transition-colors"
                    title="Edit instruction"
                  >
                    <Pencil className="size-3.5" />
                  </button>
                  {/* Delete */}
                  <button
                    type="button"
                    onClick={() => removeQueuedSteer(idx)}
                    className="size-6 rounded-full hover:bg-surface flex items-center justify-center text-foreground-subtle hover:text-destructive cursor-pointer transition-colors"
                    title="Remove instruction"
                  >
                    <Trash2 className="size-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Main composer surface (t3: glass card + hairline outline + soft shadow,
          with the workspace/branch context strip attached below the input) */}
      <div className="chat-composer-surface relative flex flex-col">
        <div className={cn(
          "flex min-w-0 flex-1 flex-col px-3 sm:px-3.5",
          isCompact ? "gap-1 pt-2.5 pb-1 sm:pt-2.5" : "gap-1.5 pt-3.5 pb-2 sm:px-4 sm:pt-4"
        )}>

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
                onClick={() => {
                  if (file.path) {
                    void openSideFile(file.path);
                  }
                }}
                className="flex items-center gap-1.5 rounded-lg border border-border-subtle bg-accent/40 px-1.5 py-1 text-ui-xs text-foreground group transition-colors hover:bg-accent/70 cursor-pointer"
                title={`Click to view ${file.name} in right sidebar`}
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
                  onClick={(e) => {
                    e.stopPropagation();
                    setAttachedFiles(prev => prev.filter((_, i) => i !== idx));
                  }}
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
          placeholder={isRunning ? 'Keep typing to queue follow-up changes' : placeholder}
          className={cn(
            "w-full resize-none bg-transparent border-0 font-sans leading-relaxed text-foreground placeholder:text-placeholder/75 focus:outline-hidden",
            isCompact
              ? "text-ui-sm min-h-[30px]"
              : "text-[length:var(--font-size-prompt,0.9375rem)]"
          )}
        />

        </div>

        {/* Bottom bar: left tray … right actions (t3 footer: spacing, no border) */}
        <div className={cn(
          "flex min-w-0 flex-nowrap items-center justify-between px-3 sm:px-3.5",
          isCompact ? "gap-1.5 pt-0.5 pb-1.5 sm:pb-2" : "gap-2 pt-1 pb-2.5 sm:px-4 sm:pb-3"
        )}>

          {/* Left Controls tray (t3 layout): pickers cluster left, send sits right */}
          <div className={cn(
            "relative -m-1 flex min-w-0 flex-1 items-center overflow-visible p-1",
            isCompact ? "gap-1 -ms-3 ps-3 sm:-ms-3.5 sm:ps-3.5" : "gap-1 -ms-3.5 ps-3.5 sm:-ms-4 sm:ps-4"
          )}>
            
            {/* [+] Button */}
            <div className="relative">
              <ComposerControl
                size={chipSize}
                onClick={() => {
                  setIsContextOpen(prev => !prev);
                  setActiveSubMenu(null);
                  setIsModeOpen(false);
                  setIsModelPickerOpen(false);
                  setIsAgentPickerOpen(false);
                  setIsEffortPickerOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isContextOpen}
                aria-label="Add context"
                pressed={isContextOpen}
                title="Add context (@)"
                className={isCompact ? "w-6.5 justify-center px-0" : "w-7 justify-center px-0"}
              >
                <Plus aria-hidden="true" />
              </ComposerControl>

              {/* Context Popover */}
              {isContextOpen && (
                <ComposerMenu label="Add context" className="absolute bottom-full left-0 mb-2 w-56">
                  {/* Files Submenu */}
                  <div className="relative">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('files')}
                      onClick={() => setActiveSubMenu(prev => prev === 'files' ? null : 'files')}
                      className="flex min-h-7 w-full cursor-pointer items-center justify-between rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary outline-none transition-colors hover:bg-accent"
                    >
                      <span className="flex items-center gap-2">
                        <FileText className="size-3.5 text-muted-foreground" />
                        <span>Files</span>
                      </span>
                      <ChevronRight className="size-3 text-fg-tertiary" />
                    </button>

                    {activeSubMenu === 'files' && (
                      <div className="dropdown-glass absolute left-full top-0 z-50 ml-1 max-h-56 w-52 overflow-y-auto rounded-lg p-1 shadow-[0_16px_40px_-18px_rgb(0_0_0/55%)]">
                        {files.filter(f => f.type === 'file').length === 0 ? (
                          <div className="px-2 py-1.5 text-ui-xs text-fg-tertiary">No files loaded</div>
                        ) : (
                          files.filter(f => f.type === 'file').slice(0, 15).map(f => (
                            <button
                              key={f.id}
                              type="button"
                              onClick={() => insertContextTag(`file:${f.name}`)}
                              className="flex min-h-7 w-full cursor-pointer items-center gap-2 truncate rounded-sm px-2 py-1 text-left font-mono text-ui-xs text-fg-secondary outline-none transition-colors hover:bg-accent hover:text-fg-primary"
                            >
                              <FileText className="size-3 shrink-0 text-success" />
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
                    className="flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary outline-none transition-colors hover:bg-accent"
                  >
                    <Bot className="size-3.5 text-muted-foreground" />
                    <span>Skills ({skills.length})</span>
                  </button>

                  {/* MCP Tools */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('mcp')}
                    className="flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary outline-none transition-colors hover:bg-accent"
                  >
                    <Wrench className="size-3.5 text-muted-foreground" />
                    <span>MCP Tools ({mcps.length})</span>
                  </button>

                  {/* Git */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('git')}
                    className="flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary outline-none transition-colors hover:bg-accent"
                  >
                    <GitBranch className="size-3.5 text-muted-foreground" />
                    <span>Git Status & Diff</span>
                  </button>

                  {/* Terminal */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('terminal')}
                    className="flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary outline-none transition-colors hover:bg-accent"
                  >
                    <Terminal className="size-3.5 text-muted-foreground" />
                    <span>Terminal</span>
                  </button>

                  <div className="mx-1 my-1 h-px bg-border-subtle" />

                  {/* Attach Local File */}
                  <button
                    type="button"
                    onClick={handleNativeFileUpload}
                    className="flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary outline-none transition-colors hover:bg-accent"
                  >
                    <Upload className="size-3.5 text-muted-foreground" />
                    <span>Attach Local File</span>
                  </button>

                  <div className="mx-1 my-1 h-px bg-border-subtle" />

                  {/* Stash draft / view stashes */}
                  <button
                    type="button"
                    onClick={() => {
                      setIsContextOpen(false);
                      if (prompt.trim() || attachedFiles.length > 0) {
                        handleStashCurrent();
                      } else {
                        setIsStashMenuOpen(true);
                      }
                    }}
                    className="flex min-h-7 w-full cursor-pointer items-center justify-between rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary outline-none transition-colors hover:bg-accent"
                  >
                    <span className="flex items-center gap-2">
                      <Archive className="size-3.5 text-muted-foreground" />
                      <span>{prompt.trim() || attachedFiles.length > 0 ? 'Stash current prompt' : 'Prompt Stash'}</span>
                    </span>
                    <ComposerKbd>Alt+S</ComposerKbd>
                  </button>
                </ComposerMenu>
              )}
            </div>

            {/* Permission Mode Selector Chip */}
            <div className="relative flex shrink-0 items-center">
              <ComposerControl
                size={chipSize}
                onClick={() => {
                  setIsModeOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsModelPickerOpen(false);
                  setIsAgentPickerOpen(false);
                  setIsEffortPickerOpen(false);
                  setIsStashMenuOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isModeOpen}
                aria-label={`Permission mode: ${currentMode.label}`}
                pressed={isModeOpen}
                className={cn('min-w-0', isFullAccess && 'text-full-access [&_svg:not([class*="opacity-"])]:text-full-access')}
                title={`Permission mode: ${currentMode.label} (${currentMode.desc})`}
              >
                <currentMode.icon aria-hidden="true" />
                <span className="min-w-0 truncate text-ui-xs leading-none">{currentMode.short}</span>
                <ComposerChevron size={chipSize} className={cn(isFullAccess && 'text-full-access')} />
              </ComposerControl>

              {/* Mode Dropdown Menu */}
              {isModeOpen && (
                <ComposerMenu label="Permission mode" className="absolute bottom-full left-0 mb-2 w-64">
                  <ComposerMenuLabel>Permission mode</ComposerMenuLabel>
                  {MODES.map(m => {
                    const isSelected = m.id === agentExecutionMode;
                    const isModeFull = m.id === 'bypass';
                    return (
                      <ComposerMenuItem
                        key={m.id}
                        role="menuitemradio"
                        selected={isSelected}
                        onClick={() => {
                          setAgentExecutionMode(m.id);
                          setIsModeOpen(false);
                        }}
                        className={cn('items-start gap-2.5 py-1.5', isModeFull && 'text-full-access')}
                      >
                        <m.icon className="mt-0.5 size-4 shrink-0 opacity-80" aria-hidden="true" />
                        <span className="min-w-0 flex-1">
                          <span className="flex items-center justify-between text-ui-sm font-medium">
                            <span>{m.label}</span>
                            {isSelected && <ComposerMenuCheck />}
                          </span>
                          <span className={cn('mt-0.5 block text-ui-xs leading-snug', isModeFull ? 'text-current opacity-80' : 'text-fg-tertiary')}>
                            {m.desc}
                          </span>
                        </span>
                      </ComposerMenuItem>
                    );
                  })}
                </ComposerMenu>
              )}
            </div>

            {/* Stash Button and Popover (only shown when stashed drafts exist, or popover is opened from + menu) */}
            {(stashEntries.length > 0 || isStashMenuOpen) && (
              <div className="relative flex shrink-0 items-center">
                {stashEntries.length > 0 && (
                  <ComposerControl
                    size={chipSize}
                    onClick={() => {
                      setIsStashMenuOpen(prev => !prev);
                      setIsModeOpen(false);
                      setIsContextOpen(false);
                      setIsModelPickerOpen(false);
                      setIsAgentPickerOpen(false);
                      setIsEffortPickerOpen(false);
                    }}
                    aria-haspopup="menu"
                    aria-expanded={isStashMenuOpen}
                    aria-label={`Prompt Stash (${stashEntries.length})`}
                    pressed={isStashMenuOpen}
                    className="min-w-0"
                    title={`${stashEntries.length} stashed prompt${stashEntries.length > 1 ? 's' : ''} (Alt+S to stash)`}
                  >
                    <Archive aria-hidden="true" className="size-3.5" />
                    <span className="rounded-full bg-primary/20 px-1 py-0.2 text-3xs font-bold text-fg-primary">
                      {stashEntries.length}
                    </span>
                  </ComposerControl>
                )}

                {isStashMenuOpen && (
                  <ComposerMenu label="Prompt Stash" className="absolute bottom-full left-0 mb-2 w-72">
                    <div className="flex items-center justify-between border-b border-border-subtle px-2 pb-1.5 pt-1">
                      <span className="text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">Prompt Stash</span>
                      {prompt.trim() && (
                        <button
                          type="button"
                          onClick={handleStashCurrent}
                          className="cursor-pointer text-2xs font-medium text-accent-primary hover:underline"
                        >
                          + Stash current
                        </button>
                      )}
                    </div>

                    <div className="max-h-60 space-y-0.5 overflow-y-auto pt-1.5 scrollbar-thin">
                      {stashEntries.length === 0 ? (
                        <div className="py-4 text-center text-ui-xs text-fg-tertiary">
                          No stashed prompts.<br />Press <ComposerKbd>Alt+S</ComposerKbd> to stash current draft.
                        </div>
                      ) : (
                        stashEntries.map(entry => (
                          <div key={entry.id} className="group flex items-start justify-between gap-1.5 rounded-md p-1.5 transition-colors hover:bg-accent">
                            <button
                              type="button"
                              onClick={() => handleRestoreStash(entry.id)}
                              className="min-w-0 flex-1 cursor-pointer text-left"
                            >
                              <div className="truncate text-ui-xs font-medium text-fg-primary">{entry.title}</div>
                              <div className="mt-0.5 line-clamp-2 text-3xs text-fg-tertiary">{entry.prompt}</div>
                            </button>
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                removePrompt(entry.id);
                              }}
                              className="cursor-pointer rounded p-1 text-fg-tertiary opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
                              title="Delete stash"
                            >
                              <Trash2 className="size-3" />
                            </button>
                          </div>
                        ))
                      )}
                    </div>

                    {stashEntries.length > 0 && (
                      <div className="mt-1 flex justify-end border-t border-border-subtle pt-1.5">
                        <button
                          type="button"
                          onClick={clearStash}
                          className="cursor-pointer px-1 text-2xs text-fg-tertiary transition-colors hover:text-destructive"
                        >
                          Clear all
                        </button>
                      </div>
                    )}
                  </ComposerMenu>
                )}
              </div>
            )}

                        {/* 1. Agent Orchestrator Picker (ACP vs Native) */}
            <div className="relative flex shrink-0 items-center">
              <ComposerControl
                size={chipSize}
                onClick={() => {
                  setIsAgentPickerOpen(prev => !prev);
                  setIsModelPickerOpen(false);
                  setIsEffortPickerOpen(false);
                  setIsUsageContextOpen(false);
                  setIsModeOpen(false);
                  setIsContextOpen(false);
                  setIsStashMenuOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isAgentPickerOpen}
                aria-label={`Agent: ${activeAgent?.name || 'Agent'}`}
                pressed={isAgentPickerOpen}
                className="min-w-0"
                title={`Agent Orchestrator: ${activeAgent?.name || 'Agent'} (${activeAgent?.type !== 'internal' ? 'ACP external agent' : 'internal engine'})`}
              >
                <ProviderIcon provider={activeAgent?.icon || activeAgent?.provider || 'agent-internal'} size={14} className="size-3.5 shrink-0" />
                <span className="min-w-0 max-w-[130px] sm:max-w-[170px] truncate leading-none text-ui-xs">
                  {activeAgent?.name?.replace(/\s*\([^)]*\)/, '') || activeAgent?.name || 'Agent'}
                </span>
                <ComposerChevron size={chipSize} />
              </ComposerControl>

              {isAgentPickerOpen && (
                <ComposerMenu label="Select Agent Orchestrator" className="absolute bottom-full left-0 mb-2 max-h-96 w-72 overflow-y-auto">
                  <ComposerMenuLabel className="flex items-center justify-between">
                    <span>Agent Orchestrators</span>
                    <span className="text-3xs font-normal normal-case tracking-normal text-fg-tertiary">ACP / Native</span>
                  </ComposerMenuLabel>
                  <div className="flex flex-col gap-0.5">
                    {agents.map(ag => {
                      const isSelected = ag.id === activeAgentId;
                      const isACP = ag.type !== 'internal';
                      return (
                        <ComposerMenuItem
                          key={ag.id}
                          selected={isSelected}
                          onClick={() => {
                            setActiveAgentId(ag.id);
                            setIsAgentPickerOpen(false);
                          }}
                          className="items-start justify-between gap-2 p-2"
                        >
                          <span className="flex min-w-0 items-start gap-2.5">
                            <ProviderIcon provider={ag.icon || ag.provider || 'agent-internal'} size={16} className="mt-0.5 size-4 shrink-0" />
                            <span className="flex min-w-0 flex-col">
                              <span className="flex items-center gap-1.5">
                                <span className="truncate text-ui-xs font-medium leading-tight">{ag.name}</span>
                                {isACP ? (
                                  <span className="rounded border border-accent-primary/30 bg-accent-primary/20 px-1 py-0.2 text-4xs font-bold uppercase leading-none tracking-wider text-accent-primary">ACP</span>
                                ) : (
                                  <span className="rounded bg-accent px-1 py-0.2 text-4xs leading-none text-fg-tertiary">native</span>
                                )}
                              </span>
                              <span className="mt-0.5 line-clamp-2 text-3xs leading-tight text-fg-tertiary">
                                {ag.description}
                              </span>
                            </span>
                          </span>
                          {isSelected && <ComposerMenuCheck className="ml-2" />}
                        </ComposerMenuItem>
                      );
                    })}
                  </div>
                </ComposerMenu>
              )}
            </div>

            {/* 2. Model Selector */}
            <div className="relative flex shrink-0 items-center">
              <ComposerControl
                size={chipSize}
                onClick={() => {
                  const willOpen = !isModelPickerOpen;
                  setIsModelPickerOpen(willOpen);
                  if (willOpen && activeAgent?.id) {
                    void refreshAgentModels(activeAgent.id);
                  }
                  setIsAgentPickerOpen(false);
                  setIsEffortPickerOpen(false);
                  setIsUsageContextOpen(false);
                  setIsModeOpen(false);
                  setIsContextOpen(false);
                  setIsStashMenuOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isModelPickerOpen}
                aria-label={`Model: ${currentModel || activeAgent?.model || 'Model'}`}
                pressed={isModelPickerOpen}
                className="min-w-0"
                title={`Model: ${currentModel || activeAgent?.model || 'Select Model'}`}
              >
                <span className="min-w-0 max-w-[130px] sm:max-w-[180px] truncate font-mono text-ui-xs leading-none">
                  {currentModel || activeAgent?.model || 'Select Model'}
                </span>
                <ComposerChevron size={chipSize} />
              </ComposerControl>

              {isModelPickerOpen && (
                <ComposerMenu label="Select Model" className="absolute bottom-full left-0 mb-2 max-h-80 w-72 overflow-y-auto">
                  {activeAgent?.type !== 'internal' ? (
                    <div>
                      <ComposerMenuLabel className="flex items-center justify-between normal-case text-accent-primary">
                        <span>{activeAgent.name} Models</span>
                        <span className="rounded bg-accent-primary/15 px-1 py-0.2 text-4xs font-bold uppercase text-accent-primary">ACP Native</span>
                      </ComposerMenuLabel>
                      <div className="flex flex-col gap-0.5">
                        {(activeAgent?.supportedModels || []).map((m: string) => {
                          const isSelected = m === currentModel || (!currentModel && m === activeAgent?.model);
                          return (
                            <ComposerMenuItem
                              key={`acp-model-${m}`}
                              selected={isSelected}
                              onClick={() => {
                                setCurrentModel(m);
                                setIsModelPickerOpen(false);
                              }}
                              className="justify-between"
                            >
                              <span className="truncate">{m}</span>
                              {isSelected && <ComposerMenuCheck />}
                            </ComposerMenuItem>
                          );
                        })}
                      </div>
                      <div className="mt-2 border-t border-border-subtle px-1 pb-1 pt-2">
                        <form
                          onSubmit={(e) => {
                            e.preventDefault();
                            const val = customModelInput.trim();
                            if (val) {
                              setCurrentModel(val);
                              setCustomModelInput('');
                              setIsModelPickerOpen(false);
                            }
                          }}
                          className="flex items-center gap-1.5"
                        >
                          <input
                            type="text"
                            value={customModelInput}
                            onChange={(e) => setCustomModelInput(e.target.value)}
                            placeholder="Custom model..."
                            className="min-w-0 flex-1 rounded-md border border-border-default bg-surface px-2 py-1 font-mono text-ui-xs text-fg-primary focus:border-accent-primary focus:outline-none"
                          />
                          <button
                            type="submit"
                            disabled={!customModelInput.trim()}
                            className="rounded-md bg-message-action px-2 py-1 text-2xs font-medium text-message-action-foreground transition-opacity hover:bg-message-action-hover disabled:opacity-40 cursor-pointer"
                          >
                            Set
                          </button>
                        </form>
                      </div>
                    </div>
                  ) : (
                    <div>
                      <ComposerMenuLabel>Native Provider Models</ComposerMenuLabel>
                      {providers.filter(p => p.enabled).map(p => {
                        const models = (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models) || [];
                        if (models.length === 0) return null;
                        return (
                          <div key={p.id} className="mt-0.5 mb-1">
                            <ComposerMenuLabel className="normal-case tracking-normal">{p.name}</ComposerMenuLabel>
                            {models.map((m: string) => {
                              const isSelected = m === currentModel;
                              return (
                                <ComposerMenuItem
                                  key={`${p.id}-${m}`}
                                  selected={isSelected}
                                  onClick={() => {
                                    setCurrentModel(m);
                                    SetActiveModel(p.id, m).catch(() => {});
                                    setIsModelPickerOpen(false);
                                  }}
                                  className="justify-between"
                                >
                                  <span className="truncate">{m}</span>
                                  {isSelected && <ComposerMenuCheck />}
                                </ComposerMenuItem>
                              );
                            })}
                          </div>
                        );
                      })}
                    </div>
                  )}
                </ComposerMenu>
              )}
            </div>

            {/* 3. Thinking Mode & Effort Selector */}
            <div className="relative flex shrink-0 items-center">
              <ComposerControl
                size={chipSize}
                onClick={() => {
                  setIsEffortPickerOpen(prev => !prev);
                  setIsAgentPickerOpen(false);
                  setIsModelPickerOpen(false);
                  setIsUsageContextOpen(false);
                  setIsModeOpen(false);
                  setIsContextOpen(false);
                  setIsStashMenuOpen(false);
                }}
                aria-haspopup="menu"
                aria-expanded={isEffortPickerOpen}
                aria-label={`Thinking effort: ${currentReasoning.label}`}
                pressed={isEffortPickerOpen}
                className={cn(
                  "min-w-0 font-normal",
                  currentReasoning.id !== 'off' ? 'text-fg-primary' : 'text-secondary-label'
                )}
                title={`Thinking effort: ${currentReasoning.label}`}
              >
                <Brain className={cn("size-3.5", currentReasoning.id !== 'off' && "text-brand")} aria-hidden="true" />
                <span className="leading-none text-ui-xs">
                  {currentReasoning.id === 'off' ? 'Off' : currentReasoning.label}
                </span>
                <ComposerChevron size={chipSize} />
              </ComposerControl>

              {isEffortPickerOpen && (
                <ComposerMenu label="Thinking Effort" className="absolute bottom-full right-0 mb-2 w-64">
                  <ComposerMenuLabel className="flex items-center justify-between">
                    <span>Thinking Effort</span>
                    <span className="text-3xs font-normal normal-case tracking-normal text-fg-tertiary">
                      {currentReasoning.id === 'off' ? 'Disabled' : currentReasoning.label}
                    </span>
                  </ComposerMenuLabel>
                  <div className="flex flex-col gap-0.5">
                    {harnessReasoningLevels.map(r => {
                      const isSelected = r.id === currentReasoning.id;
                      return (
                        <ComposerMenuItem
                          key={r.id}
                          role="menuitemradio"
                          selected={isSelected}
                          onClick={() => {
                            setReasoningLevel(r.id);
                            SetActiveReasoningEffort(r.id).catch(() => {});
                            setIsEffortPickerOpen(false);
                          }}
                          className="justify-between"
                        >
                          <span className="flex min-w-0 flex-col">
                            <span className="text-ui-sm font-medium capitalize leading-tight">{r.label}</span>
                            <span className="mt-0.5 truncate text-3xs leading-tight text-fg-tertiary">{r.desc}</span>
                          </span>
                          {isSelected && <ComposerMenuCheck className="ml-2" />}
                        </ComposerMenuItem>
                      );
                    })}
                  </div>
                </ComposerMenu>
              )}
            </div>

          </div>

          {/* Right Controls: [Context Usage Gauge] [Send ↑] */}
          <div className="relative flex min-w-0 shrink items-center gap-1">

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
              className="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-full transition-colors hover:bg-accent"
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
              <ComposerMenu label="Context window" className="absolute bottom-full right-0 mb-2 w-60 p-3">
                <div className="flex flex-col gap-1">
                  <div className="text-ui-xs font-medium text-fg-tertiary">Context window</div>
                  <div className="flex items-baseline justify-between">
                    <span className="font-mono text-ui-sm font-semibold text-fg-primary">
                      {displayUsage.formattedUsed}<span className="font-normal text-ui-xs text-fg-tertiary">/{displayUsage.formattedMax}</span>
                    </span>
                    <span className="font-mono text-ui-xs font-medium text-success">
                      ({displayUsage.percent.toFixed(1)}%)
                    </span>
                  </div>
                  <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-accent">
                    <div className="h-full rounded-full bg-success transition-all duration-300"
                      style={{ width: `${Math.min(Math.max(displayUsage.percent, displayUsage.usedTokens > 0 ? 1 : 0), 100)}%` }} />
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      setIsUsageContextOpen(false);
                      void compactCurrentSession();
                    }}
                    className="mt-2.5 flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-md border border-border-default bg-surface px-2.5 py-1.5 text-ui-xs font-medium text-fg-primary transition-colors hover:bg-surface-hover"
                  >
                    <Minimize2 className="size-3 text-brand" />
                    <span>Force Compact Chat</span>
                  </button>
                </div>
              </ComposerMenu>
            )}

            {/* Force Compact lives inside the usage popover — no standalone chip (t3 density). */}

            {/* Send / Steer / Queue / Stop (t3 message-action recipe) */}
            {isRunning ? (
              <div className="flex shrink-0 items-center gap-1.5">
                {prompt.trim() && (
                  <>
                    <button
                      type="button"
                      onClick={handleQueueSteer}
                      aria-label="Queue steering instruction"
                      className={cn(
                        "inline-flex cursor-pointer items-center gap-1 rounded-full border border-border-default bg-accent text-ui-xs font-medium text-secondary-label transition-colors hover:bg-accent/70 hover:text-foreground",
                        isCompact ? "h-7 px-2" : "h-8 px-2.5"
                      )}
                      title="Add to steering stack (Enter)"
                    >
                      <Plus className="size-3" />
                      <span>Queue</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleSteer()}
                      aria-label="Steer agent now"
                      className={cn(
                        "inline-flex cursor-pointer items-center gap-1.5 rounded-full bg-message-action text-ui-xs font-medium text-message-action-foreground shadow-xs transition-colors hover:bg-message-action-hover",
                        isCompact ? "h-7 px-2.5" : "h-8 px-3.5"
                      )}
                      title="Steer running agent immediately (Cmd+Enter)"
                    >
                      <Compass className="size-3" />
                      <span>Steer</span>
                    </button>
                  </>
                )}
                <button
                  type="button"
                  onClick={stopAgentExecution}
                  aria-label="Stop execution"
                  className={cn(
                    "flex shrink-0 cursor-pointer items-center justify-center rounded-full bg-destructive/90 text-white shadow-xs transition-colors hover:bg-destructive",
                    isCompact ? "size-7" : "size-8"
                  )}
                  title="Stop execution"
                >
                  <Square className={isCompact ? "size-2.5 fill-current" : "size-3 fill-current"} aria-hidden="true" />
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={!prompt.trim() && attachedFiles.length === 0}
                aria-label="Send message"
                className={cn(
                  "relative flex shrink-0 cursor-pointer items-center justify-center overflow-hidden rounded-full bg-message-action text-message-action-foreground shadow-xs transition-all duration-150 inset-shadow-[0_1px_rgb(255_255_255/16%)] hover:scale-105 hover:bg-message-action-hover active:scale-100 active:inset-shadow-[0_1px_rgb(0_0_0/8%)] disabled:pointer-events-none disabled:opacity-60 disabled:shadow-none disabled:hover:scale-100",
                  isCompact ? "size-7" : "size-8"
                )}
                title="Send"
              >
                <ArrowUp className={isCompact ? "size-3.5" : "size-4"} strokeWidth={2} aria-hidden="true" />
              </button>
            )}

          </div>

        </div>

      </div>

      {/* Stacked bottom context tray for active chat: workspace (left) & branch (right) */}
      {showContextStrip && (
        <div className="chat-composer-stacked-bottom mx-auto -mt-px w-14/15 min-w-0 border border-t-0 border-[color:var(--composer-stacked-border)]">
          <div className="flex min-h-8 w-full items-center justify-between px-3 py-1 sm:px-4 text-xs text-foreground-secondary">
            <div className="flex items-center min-w-0">
              <WorkspaceSwitcherDropdown variant="composer" />
            </div>
            {gitBranch && !activeWorkspacePath?.startsWith('ssh://') && (
              <div className="flex items-center shrink-0">
                <BranchSelector variant="composer" align="end" />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default AgentTaskInputBar;
