import React, { useState, useRef, useEffect, useMemo } from 'react';
import { 
  Plus, 
  Code2, 
  HelpCircle, 
  ListTree, 
  ShieldAlert, 
  ChevronDown, 
  ChevronRight, 
  FileText, 
  Folder, 
  GitBranch, 
  Cpu, 
  Terminal, 
  Upload, 
  ArrowUp, 
  Square, 
  Check, 
  Wrench, 
  Sparkles, 
  X, 
  ShieldCheck,
  Image as ImageIcon,
  Bot,
  Zap,
  Compass
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentExecutionMode, AgentReasoningLevel } from '../../types';
import { ApiBridge } from '../../services/apiBridge';
import { AcpGetAgentModels, AcpGetSlashCommands, AcpSlashCommandItem } from '../../lib/wails';


interface AgentTaskInputBarProps {
  placeholder?: string;
  autoFocus?: boolean;
  onSubmitPrompt?: (prompt: string) => void;
  isCompact?: boolean;
}

interface FlatFile {
  name: string;
  path: string;
}

function getAllFlatFiles(items: any[]): FlatFile[] {
  let list: FlatFile[] = [];
  for (const item of items) {
    if (item.type === 'file' || (!item.children && item.path)) {
      list.push({ 
        name: item.name || (item.path ? item.path.split('/').pop() : 'file'), 
        path: item.path 
      });
    }
    if (item.children && Array.isArray(item.children)) {
      list = list.concat(getAllFlatFiles(item.children));
    }
  }
  return list;
}

const MODES: { id: AgentExecutionMode; label: string; desc: string; icon: typeof Code2 }[] = [
  {
    id: 'ask',
    label: 'Ask before change',
    desc: 'Review and confirm changes before editing',
    icon: HelpCircle
  },
  {
    id: 'code',
    label: 'Edit automatically',
    desc: 'Write code and edit files automatically',
    icon: Code2
  },
  {
    id: 'plan',
    label: 'Plan mode',
    desc: 'Generate implementation plan first',
    icon: ListTree
  },
  {
    id: 'bypass',
    label: 'Full access',
    desc: 'Full system access and automatic approvals',
    icon: ShieldCheck
  }
];

const REASONING_LEVELS: { id: AgentReasoningLevel; label: string; desc: string }[] = [
  { id: 'low', label: 'Low', desc: 'Fast, lightweight reasoning' },
  { id: 'high', label: 'High', desc: 'Thorough multi-step reasoning' },
  { id: 'max', label: 'Max', desc: 'Maximum reasoning depth' }
];

export const AgentTaskInputBar: React.FC<AgentTaskInputBarProps> = ({
  placeholder = 'Ask anything, @ to mention file, / for commands...',
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
    agents,
    activeAgentId,
    setActiveAgentId,
    activeAgent,
    updateAgentConfig
  } = useWorkspace();

  const [prompt, setPrompt] = useState('');
  const [attachedFiles, setAttachedFiles] = useState<{ name: string; path?: string; content: string }[]>([]);
  const [attachedImages, setAttachedImages] = useState<{ id: string; name: string; preview: string; path?: string }[]>([]);
  
  // Dropdown states
  const [isContextOpen, setIsContextOpen] = useState(false);
  const [activeSubMenu, setActiveSubMenu] = useState<string | null>(null);
  const [isModeOpen, setIsModeOpen] = useState(false);
  const [isAgentOpen, setIsAgentOpen] = useState(false);
  const [isModelOpen, setIsModelOpen] = useState(false);
  const [isReasoningOpen, setIsReasoningOpen] = useState(false);

  // @ mention autocomplete state
  const [mentionQuery, setMentionQuery] = useState<string | null>(null);
  const [mentionStart, setMentionStart] = useState<number>(-1);
  const [selectedMentionIndex, setSelectedMentionIndex] = useState<number>(0);

  // / slash command autocomplete state
  const [slashQuery, setSlashQuery] = useState<string | null>(null);
  const [slashStart, setSlashStart] = useState<number>(-1);
  const [selectedSlashIndex, setSelectedSlashIndex] = useState<number>(0);
  const [availableSlashCommands, setAvailableSlashCommands] = useState<AcpSlashCommandItem[]>([]);

  // Discovered agent models state (from local config e.g. ~/.pi/agent/models.json or ~/.omp/agent/models.yml)
  const [agentModels, setAgentModels] = useState<string[]>([]);


  const containerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);

  const isRunning = activeSession?.status === 'running';

  const currentAgent = agents.find(a => a.id === activeAgentId) || activeAgent || agents[0];

  // Extract flat list of all workspace files for autocomplete
  const allFilesList = useMemo(() => getAllFlatFiles(files), [files]);

  // Load models and slash commands/skills when active agent changes
  useEffect(() => {
    let isMounted = true;
    if (currentAgent.type !== 'internal') {
      AcpGetAgentModels(currentAgent.id)

        .then(models => {
          if (isMounted && Array.isArray(models) && models.length > 0) {
            setAgentModels(models);
          } else if (isMounted) {
            setAgentModels([]);
          }
        })
        .catch(() => {
          if (isMounted) setAgentModels([]);
        });
    } else {
      setAgentModels([]);
    }

    AcpGetSlashCommands(currentAgent.id)
      .then(cmds => {
        if (isMounted && Array.isArray(cmds) && cmds.length > 0) {
          setAvailableSlashCommands(cmds);
        } else if (isMounted) {
          setAvailableSlashCommands([]);
        }
      })
      .catch(() => {
        if (isMounted) setAvailableSlashCommands([]);
      });

    return () => {
      isMounted = false;
    };
  }, [currentAgent.id, currentAgent.type]);

  // Filter matching files for @ mention
  const matchingMentionFiles = useMemo(() => {
    if (mentionQuery === null) return [];
    const q = mentionQuery.toLowerCase().trim();
    if (!q) return allFilesList.slice(0, 10);
    return allFilesList
      .filter(f => f.name.toLowerCase().includes(q) || f.path.toLowerCase().includes(q))
      .slice(0, 10);
  }, [allFilesList, mentionQuery]);

  // Filter matching slash commands and skills
  const matchingSlashCommands = useMemo(() => {
    if (slashQuery === null) return [];
    const q = slashQuery.toLowerCase().trim();
    if (!q) return availableSlashCommands.slice(0, 15);
    return availableSlashCommands
      .filter(c => c.name.toLowerCase().includes(q) || c.description.toLowerCase().includes(q))
      .slice(0, 15);
  }, [availableSlashCommands, slashQuery]);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsContextOpen(false);
        setActiveSubMenu(null);
        setIsModeOpen(false);
        setIsAgentOpen(false);
        setIsModelOpen(false);
        setIsReasoningOpen(false);
        setMentionQuery(null);
        setSlashQuery(null);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    // Handle keyboard navigation when @ mention popup is open
    if (mentionQuery !== null && matchingMentionFiles.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedMentionIndex(i => (i + 1) % matchingMentionFiles.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedMentionIndex(i => (i - 1 + matchingMentionFiles.length) % matchingMentionFiles.length);
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        const selected = matchingMentionFiles[selectedMentionIndex] || matchingMentionFiles[0];
        if (selected) {
          selectMentionFile(selected);
        }
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionQuery(null);
        return;
      }
    }

    // Handle keyboard navigation when / slash commands popup is open
    if (slashQuery !== null && matchingSlashCommands.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedSlashIndex(i => (i + 1) % matchingSlashCommands.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedSlashIndex(i => (i - 1 + matchingSlashCommands.length) % matchingSlashCommands.length);
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        const selected = matchingSlashCommands[selectedSlashIndex] || matchingSlashCommands[0];
        if (selected) {
          selectSlashCommand(selected);
        }
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setSlashQuery(null);
        return;
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    setPrompt(val);

    const cursor = e.target.selectionStart || val.length;
    const textBefore = val.slice(0, cursor);

    // 1. Check for @ mention trigger
    const atIdx = textBefore.lastIndexOf('@');
    if (atIdx !== -1 && (atIdx === 0 || /\s/.test(textBefore[atIdx - 1]))) {
      const query = textBefore.slice(atIdx + 1);
      if (!/\s/.test(query)) {
        setMentionQuery(query);
        setMentionStart(atIdx);
        setSelectedMentionIndex(0);
        setSlashQuery(null);
        return;
      }
    }
    setMentionQuery(null);
    setMentionStart(-1);

    // 2. Check for / slash command trigger
    const slashIdx = textBefore.lastIndexOf('/');
    if (slashIdx !== -1 && (slashIdx === 0 || /\s/.test(textBefore[slashIdx - 1]))) {
      const query = textBefore.slice(slashIdx + 1);
      if (!/\s/.test(query)) {
        setSlashQuery(query);
        setSlashStart(slashIdx);
        setSelectedSlashIndex(0);
        return;
      }
    }
    setSlashQuery(null);
    setSlashStart(-1);
  };

  const selectSlashCommand = (item: AcpSlashCommandItem) => {
    if (slashStart === -1) return;
    const before = prompt.slice(0, slashStart);
    const cursor = textareaRef.current?.selectionStart || (slashStart + (slashQuery?.length || 0) + 1);
    const after = prompt.slice(cursor);
    const newPrompt = `${before}${item.name} ${after}`;
    setPrompt(newPrompt);
    setSlashQuery(null);
    setSlashStart(-1);

    setTimeout(() => {
      if (textareaRef.current) {
        const nextPos = before.length + item.name.length + 1;
        textareaRef.current.focus();
        textareaRef.current.setSelectionRange(nextPos, nextPos);
      }
    }, 10);
  };


  const selectMentionFile = async (file: FlatFile) => {
    if (mentionStart === -1) return;
    const before = prompt.slice(0, mentionStart);
    const cursor = textareaRef.current?.selectionStart || (mentionStart + (mentionQuery?.length || 0) + 1);
    const after = prompt.slice(cursor);
    const newPrompt = `${before}@${file.name} ${after}`;
    setPrompt(newPrompt);
    setMentionQuery(null);
    setMentionStart(-1);

    // Read file contents and attach as context pill
    try {
      const content = await ApiBridge.readFile(file.path);
      setAttachedFiles(prev => {
        if (prev.some(f => f.path === file.path || f.name === file.name)) return prev;
        return [...prev, { name: file.name, path: file.path, content }];
      });
    } catch {
      setAttachedFiles(prev => {
        if (prev.some(f => f.name === file.name)) return prev;
        return [...prev, { name: file.name, path: file.path, content: `[Referenced file: ${file.path}]` }];
      });
    }

    setTimeout(() => {
      if (textareaRef.current) {
        const nextPos = mentionStart + file.name.length + 2;
        textareaRef.current.focus();
        textareaRef.current.setSelectionRange(nextPos, nextPos);
      }
    }, 10);
  };

  const handlePaste = async (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    // 1. Check for image items in clipboard (e.g. screenshot)
    const items = e.clipboardData.items;
    let foundImage = false;
    for (let i = 0; i < items.length; i++) {
      if (items[i].type.indexOf('image') !== -1) {
        foundImage = true;
        const blob = items[i].getAsFile();
        if (blob) {
          const reader = new FileReader();
          reader.onload = () => {
            const result = reader.result as string;
            setAttachedImages(prev => [
              ...prev,
              { 
                id: `paste-img-${Date.now()}-${Math.random()}`, 
                name: `screenshot-${new Date().toLocaleTimeString().replace(/:/g, '-')}.png`, 
                preview: result 
              }
            ]);
          };
          reader.readAsDataURL(blob);
        }
      }
    }
    if (foundImage) return;

    // 2. Check if pasted text is an image file path (e.g. /path/to/image.png)
    const text = e.clipboardData.getData('text').trim();
    if (/\.(png|jpe?g|webp|gif|svg)$/i.test(text)) {
      try {
        const b64 = await ApiBridge.readFileBase64(text);
        if (b64) {
          const ext = text.split('.').pop()?.toLowerCase() || 'png';
          const mime = ext === 'svg' ? 'image/svg+xml' : `image/${ext === 'jpg' ? 'jpeg' : ext}`;
          const dataUrl = `data:${mime};base64,${b64}`;
          const fileName = text.split('/').pop() || text;
          setAttachedImages(prev => [
            ...prev,
            { id: `path-img-${Date.now()}`, name: fileName, preview: dataUrl, path: text }
          ]);
        }
      } catch {
        // Not a local readable file path, allow normal text paste
      }
    }
  };

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    if (!e.dataTransfer.files) return;
    for (let i = 0; i < e.dataTransfer.files.length; i++) {
      const file = e.dataTransfer.files[i];
      if (file.type.startsWith('image/')) {
        const reader = new FileReader();
        reader.onload = () => {
          setAttachedImages(prev => [
            ...prev,
            { id: `drop-img-${Date.now()}-${Math.random()}`, name: file.name, preview: reader.result as string }
          ]);
        };
        reader.readAsDataURL(file);
      } else {
        try {
          const text = await file.text();
          setAttachedFiles(prev => [...prev, { name: file.name, path: file.name, content: text }]);
        } catch {
          // ignore
        }
      }
    }
  };

  const handleImageUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files) return;
    for (let i = 0; i < e.target.files.length; i++) {
      const file = e.target.files[i];
      if (!file.type.startsWith('image/')) continue;
      const reader = new FileReader();
      reader.onload = () => {
        setAttachedImages(prev => [
          ...prev,
          { id: `img-${Date.now()}-${Math.random()}`, name: file.name, preview: reader.result as string }
        ]);
      };
      reader.readAsDataURL(file);
    }
    e.target.value = '';
  };

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      for (let i = 0; i < e.target.files.length; i++) {
        const file = e.target.files[i];
        try {
          const text = await file.text();
          setAttachedFiles(prev => [...prev, { name: file.name, path: file.name, content: text }]);
        } catch {
          setAttachedFiles(prev => [...prev, { name: file.name, path: file.name, content: `[File: ${file.name}]` }]);
        }
      }
      setIsContextOpen(false);
    }
  };

  const handleSubmit = () => {
    let fullPrompt = prompt.trim();
    if (attachedFiles.length > 0) {
      const attachments = attachedFiles.map(f => `\n[Attached File: ${f.name}]\n${f.content}`).join('\n');
      fullPrompt = `${fullPrompt}\n${attachments}`.trim();
    }
    if (attachedImages.length > 0) {
      const imgInfo = attachedImages.map(img => `[Attached Image: ${img.name}${img.path ? ` (path: ${img.path})` : ''}]`).join('\n');
      fullPrompt = `${fullPrompt}\n\n${imgInfo}`.trim();
    }

    if (!fullPrompt && attachedImages.length === 0) return;

    if (onSubmitPrompt) {
      onSubmitPrompt(fullPrompt);
    } else if (activeSession) {
      sendAgentPrompt(fullPrompt);
    } else {
      createNewSession(fullPrompt);
    }

    setPrompt('');
    setAttachedFiles([]);
    setAttachedImages([]);
    setIsContextOpen(false);
    setIsModeOpen(false);
    setIsAgentOpen(false);
    setIsModelOpen(false);
    setIsReasoningOpen(false);
    setMentionQuery(null);
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

  const currentMode = MODES.find(m => m.id === agentExecutionMode) || MODES[3];
  const currentReasoning = REASONING_LEVELS.find(r => r.id === reasoningLevel) || REASONING_LEVELS[2];

  // Group available models by provider name

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
        { providerName: 'OpenAI', models: ['gpt-4o', 'gpt-4o-mini', 'o3-mini'] },
        { providerName: 'Anthropic', models: ['claude-3-7-sonnet-20250219', 'claude-3-5-sonnet-20241022'] },
        { providerName: 'Google', models: ['gemini-2.0-flash', 'gemini-1.5-pro'] },
        { providerName: 'Ollama', models: ['qwen2.5-coder:latest', 'deepseek-r1:latest'] }
      );
    }

    return groups;
  }, [providers]);

  // Agent icon helper
  const renderAgentIcon = (agent: any) => {
    if (agent.type === 'pi') {
      return <span className="font-serif font-bold text-amber-400 text-xs px-0.5">π</span>;
    }
    if (agent.type === 'ohmypi') {
      return <Zap className="w-3.5 h-3.5 text-purple-400" />;
    }
    if (agent.type === 'internal') {
      return <Bot className="w-3.5 h-3.5 text-blue-400" />;
    }
    return <Terminal className="w-3.5 h-3.5 text-cyan-400" />;
  };

  return (
    <div 
      className="w-full relative select-none" 
      ref={containerRef}
      onDragOver={(e) => e.preventDefault()}
      onDrop={handleDrop}
    >
      <input 
        type="file" 
        multiple
        ref={fileInputRef} 
        onChange={handleFileUpload} 
        className="hidden" 
      />
      <input 
        type="file" 
        accept="image/*"
        multiple
        ref={imageInputRef} 
        onChange={handleImageUpload} 
        className="hidden" 
      />

      {/* Floating @ Mention Autocomplete Popover */}
      {mentionQuery !== null && matchingMentionFiles.length > 0 && (
        <div className="absolute left-3 bottom-full mb-3 w-72 rounded-xl bg-white dark:bg-[#202022] shadow-2xl border border-[#e5e7eb] dark:border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs max-h-60 overflow-y-auto">
          <div className="px-3 py-1 flex items-center justify-between text-[10px] font-semibold text-[#6b7280] dark:text-[#777777] uppercase tracking-wider border-b border-[#e5e7eb] dark:border-[#2d2d30] pb-1">
            <span>Mention File (@)</span>
            <span className="font-mono text-[9px] lowercase">↑↓ navigate • ↵ insert</span>
          </div>
          <div className="py-1">
            {matchingMentionFiles.map((file, idx) => {
              const isSelected = idx === selectedMentionIndex;
              return (
                <button
                  key={file.path || file.name}
                  type="button"
                  onMouseEnter={() => setSelectedMentionIndex(idx)}
                  onClick={() => selectMentionFile(file)}
                  className={`w-full text-left px-3 py-1.5 flex items-center gap-2 cursor-pointer transition-colors ${
                    isSelected 
                      ? 'bg-[#3b82f6]/15 text-[#2563eb] dark:text-[#60a5fa] font-medium' 
                      : 'text-[#374151] dark:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e]'
                  }`}
                >
                  <FileText className={`w-3.5 h-3.5 shrink-0 ${isSelected ? 'text-[#3b82f6]' : 'text-[#9ca3af]'}`} />
                  <div className="flex-1 min-w-0">
                    <div className="truncate text-xs font-mono">{file.name}</div>
                    {file.path && file.path !== file.name && (
                      <div className="text-[10px] text-[#9ca3af] dark:text-[#666666] truncate font-mono">
                        {file.path}
                      </div>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Floating / Slash Commands & Skills Autocomplete Popover */}
      {slashQuery !== null && matchingSlashCommands.length > 0 && (
        <div className="absolute left-3 bottom-full mb-3 w-80 rounded-xl bg-white dark:bg-[#202022] shadow-2xl border border-[#e5e7eb] dark:border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs max-h-72 overflow-y-auto">
          <div className="px-3 py-1 flex items-center justify-between text-[10px] font-semibold text-[#6b7280] dark:text-[#777777] uppercase tracking-wider border-b border-[#e5e7eb] dark:border-[#2d2d30] pb-1">
            <span>Agent Slash Commands & Skills (/)</span>
            <span className="font-mono text-[9px] lowercase">↑↓ navigate • ↵ insert</span>
          </div>
          <div className="py-1">
            {matchingSlashCommands.map((item, idx) => {
              const isSelected = idx === selectedSlashIndex;
              return (
                <button
                  key={item.name}
                  type="button"
                  onMouseEnter={() => setSelectedSlashIndex(idx)}
                  onClick={() => selectSlashCommand(item)}
                  className={`w-full text-left px-3 py-1.5 flex items-start gap-2.5 cursor-pointer transition-colors ${
                    isSelected 
                      ? 'bg-[#3b82f6]/15 text-[#2563eb] dark:text-[#60a5fa] font-medium' 
                      : 'text-[#374151] dark:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e]'
                  }`}
                >
                  <div className="mt-0.5 shrink-0">
                    {item.category === 'skill' ? (
                      <Compass className={`w-3.5 h-3.5 ${isSelected ? 'text-[#3b82f6]' : 'text-amber-400'}`} />
                    ) : (
                      <Terminal className={`w-3.5 h-3.5 ${isSelected ? 'text-[#3b82f6]' : 'text-blue-400'}`} />
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-1">
                      <span className="font-mono font-semibold text-xs text-[#111827] dark:text-[#eeeeee]">
                        {item.name}
                      </span>
                      <span className={`text-[9px] px-1.5 py-0.2 rounded font-sans uppercase ${
                        item.category === 'skill' 
                          ? 'bg-amber-400/15 text-amber-500 dark:text-amber-400' 
                          : 'bg-blue-400/15 text-blue-500 dark:text-blue-400'
                      }`}>
                        {item.category}
                      </span>
                    </div>
                    {item.description && (
                      <div className="text-[11px] text-[#6b7280] dark:text-[#888888] truncate leading-tight mt-0.5">
                        {item.description}
                      </div>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      )}


      <div className={`w-full rounded-2xl border border-[#e5e7eb] dark:border-[#2d2d30] bg-[#ffffff] dark:bg-[#1a1a1c] shadow-xl transition-all duration-150 overflow-visible relative ${isCompact ? 'p-2.5' : 'p-3'}`}>
        
        {/* Attached Files & Images Pills */}
        {(attachedFiles.length > 0 || attachedImages.length > 0) && (
          <div className="flex flex-wrap gap-1.5 px-1 pb-2">
            {/* Image Pills */}
            {attachedImages.map((img) => (
              <div 
                key={img.id} 
                className="flex items-center gap-1.5 pl-1.5 pr-2 py-1 rounded-lg bg-[#f3f4f6] dark:bg-[#262628] text-[#111827] dark:text-[#e0e0e0] text-xs border border-[#e5e7eb] dark:border-[#38383a]"
              >
                <img 
                  src={img.preview} 
                  alt={img.name} 
                  className="w-5 h-5 rounded object-cover border border-[#e5e7eb] dark:border-[#38383a]" 
                />
                <span className="truncate max-w-[120px] font-mono text-[11px]">{img.name}</span>
                <button
                  type="button"
                  onClick={() => setAttachedImages(prev => prev.filter(i => i.id !== img.id))}
                  className="text-[#9ca3af] hover:text-[#ef4444] ml-0.5 cursor-pointer"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            ))}

            {/* File Pills */}
            {attachedFiles.map((file, idx) => (
              <div 
                key={idx} 
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-[#f3f4f6] dark:bg-[#262628] text-[#111827] dark:text-[#e0e0e0] text-xs font-mono border border-[#e5e7eb] dark:border-[#38383a]"
              >
                <FileText className="w-3.5 h-3.5 text-[#3b82f6] shrink-0" />
                <span className="truncate max-w-[150px]">{file.name}</span>
                <button
                  type="button"
                  onClick={() => setAttachedFiles(prev => prev.filter((_, i) => i !== idx))}
                  className="text-[#9ca3af] hover:text-[#ef4444] ml-1 cursor-pointer"
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
          onPaste={handlePaste}
          placeholder={placeholder}
          className="w-full resize-none bg-transparent border-0 text-[13px] text-[#111827] dark:text-[#eeeeee] placeholder-[#9ca3af] dark:placeholder-[#777777] focus:outline-hidden leading-relaxed font-sans px-1"
        />

        {/* Bottom Control Bar */}
        <div className="pt-2 flex items-center justify-between gap-2 flex-wrap text-xs border-t border-[#f3f4f6] dark:border-[#262628]/60 mt-1">
          
          {/* Left Controls: [+] [🖼 Image] [🛡 Mode ⌄] [🤖 Agent ⌄] */}
          <div className="flex items-center gap-1.5 relative flex-wrap">
            
            {/* [+] Context Button */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsContextOpen(prev => !prev);
                  setActiveSubMenu(null);
                  setIsModeOpen(false);
                  setIsAgentOpen(false);
                  setIsModelOpen(false);
                  setIsReasoningOpen(false);
                }}
                className={`w-6 h-6 rounded-md flex items-center justify-center transition-colors cursor-pointer text-[#888888] hover:text-white hover:bg-[#28282b] ${isContextOpen ? 'bg-[#28282b] text-white' : ''}`}
                title="Add context (@)"
              >
                <Plus className="w-4 h-4" />
              </button>

              {/* Context Popover */}
              {isContextOpen && (
                <div className="absolute left-0 bottom-full mb-2 w-56 rounded-xl bg-[#202022] shadow-2xl border border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs text-[#cccccc]">
                  {/* Files */}
                  <div className="relative">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('files')}
                      onClick={() => setActiveSubMenu(prev => prev === 'files' ? null : 'files')}
                      className="w-full text-left px-3 py-1.5 hover:bg-[#2b2b2e] hover:text-white flex items-center justify-between cursor-pointer"
                    >
                      <div className="flex items-center gap-2">
                        <FileText className="w-3.5 h-3.5 text-[#888888]" />
                        <span>Files</span>
                      </div>
                      <ChevronRight className="w-3.5 h-3.5 text-[#666666]" />
                    </button>

                    {activeSubMenu === 'files' && (
                      <div className="absolute left-full top-0 ml-1 w-52 rounded-xl bg-[#202022] shadow-xl border border-[#333336] py-1.5 z-50 max-h-56 overflow-y-auto">
                        {allFilesList.length === 0 ? (
                          <div className="px-3 py-2 text-xs text-[#777777]">No files loaded</div>
                        ) : (
                          allFilesList.slice(0, 15).map(f => (
                            <button
                              key={f.path}
                              type="button"
                              onClick={() => insertContextTag(`file:${f.name}`)}
                              className="w-full text-left px-3 py-1 text-xs text-[#bbbbbb] hover:bg-[#2b2b2e] hover:text-white flex items-center gap-2 truncate cursor-pointer"
                            >
                              <FileText className="w-3 h-3 text-[#3b82f6] shrink-0" />
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
                    className="w-full text-left px-3 py-1.5 hover:bg-[#2b2b2e] hover:text-white flex items-center gap-2 cursor-pointer"
                  >
                    <Cpu className="w-3.5 h-3.5 text-[#a855f7]" />
                    <span>Skills ({skills.length})</span>
                  </button>

                  {/* MCP Tools */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('mcp')}
                    className="w-full text-left px-3 py-1.5 hover:bg-[#2b2b2e] hover:text-white flex items-center gap-2 cursor-pointer"
                  >
                    <Wrench className="w-3.5 h-3.5 text-[#ec4899]" />
                    <span>MCP Tools ({mcps.length})</span>
                  </button>

                  {/* Git */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('git')}
                    className="w-full text-left px-3 py-1.5 hover:bg-[#2b2b2e] hover:text-white flex items-center gap-2 cursor-pointer"
                  >
                    <GitBranch className="w-3.5 h-3.5 text-[#3b82f6]" />
                    <span>Git Status & Diff</span>
                  </button>

                  {/* Terminal */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('terminal')}
                    className="w-full text-left px-3 py-1.5 hover:bg-[#2b2b2e] hover:text-white flex items-center gap-2 cursor-pointer"
                  >
                    <Terminal className="w-3.5 h-3.5 text-[#eab308]" />
                    <span>Terminal</span>
                  </button>

                  <div className="h-[1px] bg-[#2d2d30] my-1" />

                  {/* Attach Local File */}
                  <button
                    type="button"
                    onClick={() => fileInputRef.current?.click()}
                    className="w-full text-left px-3 py-1.5 hover:bg-[#2b2b2e] hover:text-white flex items-center gap-2 cursor-pointer"
                  >
                    <Upload className="w-3.5 h-3.5 text-[#10b981]" />
                    <span>Attach local file...</span>
                  </button>
                </div>
              )}
            </div>

            {/* [🖼] Image Upload Button */}
            <button
              type="button"
              onClick={() => imageInputRef.current?.click()}
              className="w-6 h-6 rounded-md flex items-center justify-center transition-colors cursor-pointer text-[#888888] hover:text-white hover:bg-[#28282b]"
              title="Upload / attach image (supports paste & drag-drop)"
            >
              <ImageIcon className="w-3.5 h-3.5" />
            </button>

            {/* Agent Selector Dropdown: [🤖 ForgeADE Internal ⌄] */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsAgentOpen(prev => !prev);
                  setIsModeOpen(false);
                  setIsContextOpen(false);
                  setIsModelOpen(false);
                  setIsReasoningOpen(false);
                }}
                className="flex items-center gap-1.5 px-2 py-1 rounded-full text-xs text-[#dddddd] hover:text-white hover:bg-[#262629] transition-colors cursor-pointer group"
                title="Select Active Agent (Internal, Pi, OMP)"
              >
                {renderAgentIcon(currentAgent)}
                <span className="font-medium text-[#dddddd] truncate max-w-[110px]">{currentAgent.name}</span>
                <span className={`w-1.5 h-1.5 rounded-full ${currentAgent.enabled ? 'bg-emerald-400' : 'bg-gray-400'}`} />
                <ChevronDown className="w-3 h-3 text-[#777777] group-hover:text-[#aaaaaa]" />
              </button>

              {/* Agent Dropdown */}
              {isAgentOpen && (
                <div className="absolute left-0 bottom-full mb-2 w-64 rounded-xl bg-white dark:bg-[#202022] shadow-2xl border border-[#e5e7eb] dark:border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs">
                  <div className="px-3 py-1 text-[10px] font-semibold text-[#6b7280] dark:text-[#777777] uppercase tracking-wider">
                    Select Agent Engine
                  </div>
                  {agents.filter(a => a.enabled).map(ag => {
                    const isSelected = ag.id === activeAgentId;
                    return (
                      <button
                        key={ag.id}
                        type="button"
                        onClick={() => {
                          setActiveAgentId(ag.id);
                          setIsAgentOpen(false);
                        }}
                        className={`w-full text-left px-3 py-2 flex items-center justify-between hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e] cursor-pointer transition-colors ${
                          isSelected ? 'bg-[#f3f4f6] dark:bg-[#2b2b2e] text-[#111827] dark:text-white font-medium' : 'text-[#374151] dark:text-[#cccccc]'
                        }`}
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <div className="w-5 h-5 rounded flex items-center justify-center bg-[#f3f4f6] dark:bg-[#262628] shrink-0">
                            {renderAgentIcon(ag)}
                          </div>
                          <div className="min-w-0 truncate">
                            <div className="truncate text-xs font-semibold">{ag.name}</div>
                            <div className="text-[10px] text-[#9ca3af] dark:text-[#777777] truncate">
                              {ag.type === 'internal' ? 'Native Engine' : `ACP stdio (${ag.command || ag.type})`}
                            </div>
                          </div>
                        </div>
                        {isSelected && <Check className="w-3.5 h-3.5 text-[#3b82f6] shrink-0" />}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Mode Selector Button */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsModeOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsAgentOpen(false);
                  setIsModelOpen(false);
                  setIsReasoningOpen(false);
                }}
                className="flex items-center gap-1.5 px-2 py-1 rounded-full text-xs text-[#dddddd] hover:text-white hover:bg-[#262629] transition-colors cursor-pointer group"
              >
                <currentMode.icon className="w-3.5 h-3.5 text-[#f59e0b]" />
                <span className="font-medium text-[#dddddd]">{currentMode.label}</span>
                <ChevronDown className="w-3 h-3 text-[#777777] group-hover:text-[#aaaaaa]" />
              </button>

              {/* Mode Dropdown */}
              {isModeOpen && (
                <div className="absolute left-0 bottom-full mb-2 w-64 rounded-xl bg-[#202022] shadow-2xl border border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs">
                  <div className="px-3 py-1 text-[10px] font-semibold text-[#777777] uppercase tracking-wider">
                    Select Mode
                  </div>
                  {MODES.map(m => {
                    const isSelected = m.id === agentExecutionMode;
                    const Icon = m.icon;
                    return (
                      <button
                        key={m.id}
                        type="button"
                        onClick={() => {
                          setAgentExecutionMode(m.id);
                          setIsModeOpen(false);
                        }}
                        className={`w-full text-left px-3 py-2 flex items-start gap-2.5 hover:bg-[#2b2b2e] cursor-pointer transition-colors ${
                          isSelected ? 'bg-[#2b2b2e]/70 text-white' : 'text-[#cccccc]'
                        }`}
                      >
                        <Icon className="w-4 h-4 text-[#f59e0b] shrink-0 mt-0.5" />
                        <div className="flex-1 min-w-0">
                          <div className="font-medium text-xs flex items-center justify-between">
                            <span>{m.label}</span>
                            {isSelected && <Check className="w-3.5 h-3.5 text-[#3b82f6]" />}
                          </div>
                          <p className="text-[11px] text-[#777777] leading-snug mt-0.5">
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

          {/* Right Controls: [⚪ Model ⌄] [⚙ Max ⌄] [↑ Send] */}
          <div className="flex items-center gap-2 relative">
            
            {/* Model Selector Dropdown */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsModelOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsAgentOpen(false);
                  setIsModeOpen(false);
                  setIsReasoningOpen(false);
                }}
                className="flex items-center gap-1.5 px-2 py-1 rounded-full text-xs text-[#cccccc] hover:text-white hover:bg-[#262629] transition-colors cursor-pointer group"
                title="Model selection for active agent"
              >
                <span className="w-2 h-2 rounded-full bg-[#10b981] ring-2 ring-[#10b981]/20" />
                <span className="font-medium text-[#dddddd] truncate max-w-[120px]">
                  {currentAgent.type !== 'internal' 
                    ? (currentAgent.model || 'Agent Default') 
                    : (currentModel || 'GLM-5.3-Flash')}
                </span>
                <ChevronDown className="w-3 h-3 text-[#777777] group-hover:text-[#aaaaaa]" />
              </button>

              {/* Model Dropdown */}
              {isModelOpen && (
                <div className="absolute right-0 bottom-full mb-2 w-64 rounded-xl bg-white dark:bg-[#202022] shadow-2xl border border-[#e5e7eb] dark:border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs max-h-72 overflow-y-auto">
                  <div className="px-3 py-1 text-[10px] font-semibold text-[#6b7280] dark:text-[#777777] uppercase tracking-wider">
                    {currentAgent.type !== 'internal' ? `${currentAgent.name} Models` : 'Select LLM Model'}
                  </div>

                  {currentAgent.type !== 'internal' ? (
                    <div className="p-2 space-y-1">
                      <div className="text-[11px] text-[#6b7280] dark:text-[#9ca3af] px-1 pb-1 flex items-center justify-between">
                        <span>Configured for {currentAgent.name}:</span>
                        {agentModels.length > 0 && (
                          <span className="text-[10px] text-emerald-500 font-mono">
                            {agentModels.length} detected
                          </span>
                        )}
                      </div>

                      {/* Default Agent Model Option */}
                      <button
                        type="button"
                        onClick={() => {
                          updateAgentConfig(currentAgent.id, { model: undefined });
                          setIsModelOpen(false);
                        }}
                        className="w-full text-left px-2 py-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e] cursor-pointer text-xs font-mono flex items-center justify-between"
                      >
                        <div className="flex items-center gap-1.5">
                          <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                          <span className="italic text-[#888888]">agent-default</span>
                        </div>
                        {!currentAgent.model && (
                          <Check className="w-3 h-3 text-emerald-400" />
                        )}
                      </button>

                      {/* Detected Agent Models from local config files */}
                      {agentModels.length > 0 ? (
                        agentModels.map(m => {
                          const isSelected = currentAgent.model === m;
                          return (
                            <button
                              key={m}
                              type="button"
                              onClick={() => {
                                updateAgentConfig(currentAgent.id, { model: m });
                                setIsModelOpen(false);
                              }}
                              className={`w-full text-left px-2 py-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e] cursor-pointer text-xs font-mono flex items-center justify-between transition-colors ${
                                isSelected ? 'bg-[#3b82f6]/15 text-[#2563eb] dark:text-[#60a5fa] font-medium' : ''
                              }`}
                            >
                              <span className="truncate">{m}</span>
                              {isSelected && (
                                <Check className="w-3 h-3 text-emerald-400 shrink-0" />
                              )}
                            </button>
                          );
                        })
                      ) : (
                        ['claude-3-7-sonnet', 'claude-3-5-sonnet', 'gpt-4o', 'deepseek-r1', 'qwen2.5-coder'].map(m => {
                          const isSelected = currentAgent.model === m;
                          return (
                            <button
                              key={m}
                              type="button"
                              onClick={() => {
                                updateAgentConfig(currentAgent.id, { model: m });
                                setIsModelOpen(false);
                              }}
                              className="w-full text-left px-2 py-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e] cursor-pointer text-xs font-mono flex items-center justify-between"
                            >
                              <span className="truncate">{m}</span>
                              {isSelected && (
                                <Check className="w-3 h-3 text-emerald-400 shrink-0" />
                              )}
                            </button>
                          );
                        })
                      )}
                    </div>
                  ) : (
                    groupedModels.map(group => (
                      <div key={group.providerName} className="py-1">
                        <div className="px-3 py-0.5 text-[10px] font-bold text-[#9ca3af] dark:text-[#666666] uppercase">
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
                              className={`w-full text-left px-3 py-1.5 flex items-center justify-between hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e] cursor-pointer transition-colors ${
                                isSelected ? 'bg-[#f3f4f6] dark:bg-[#2b2b2e] text-[#111827] dark:text-white font-medium' : 'text-[#374151] dark:text-[#cccccc]'
                              }`}
                            >
                              <div className="flex items-center gap-2 truncate">
                                <span className="w-1.5 h-1.5 rounded-full bg-[#10b981]" />
                                <span className="truncate font-mono">{mod}</span>
                              </div>
                              {isSelected && <Check className="w-3.5 h-3.5 text-[#3b82f6]" />}
                            </button>
                          );
                        })}
                      </div>
                    ))
                  )}
                </div>
              )}
            </div>

            {/* Reasoning Level Selector: ⚙ Max ⌄ */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsReasoningOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsAgentOpen(false);
                  setIsModeOpen(false);
                  setIsModelOpen(false);
                }}
                className="flex items-center gap-1.5 px-2 py-1 rounded-full text-xs text-[#cccccc] hover:text-white hover:bg-[#262629] transition-colors cursor-pointer group"
              >
                <Sparkles className="w-3.5 h-3.5 text-[#8b5cf6]" />
                <span className="font-medium text-[#dddddd]">{currentReasoning.label}</span>
                <ChevronDown className="w-3 h-3 text-[#777777] group-hover:text-[#aaaaaa]" />
              </button>

              {/* Reasoning Level Dropdown */}
              {isReasoningOpen && (
                <div className="absolute right-0 bottom-full mb-2 w-52 rounded-xl bg-[#202022] shadow-2xl border border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs">
                  <div className="px-3 py-1 text-[10px] font-semibold text-[#777777] uppercase tracking-wider">
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
                        className={`w-full text-left px-3 py-1.5 hover:bg-[#2b2b2e] cursor-pointer transition-colors ${
                          isSelected ? 'bg-[#2b2b2e] text-white font-medium' : 'text-[#cccccc]'
                        }`}
                      >
                        <div className="flex items-center justify-between">
                          <span className="font-medium">{r.label}</span>
                          {isSelected && <Check className="w-3.5 h-3.5 text-[#3b82f6]" />}
                        </div>
                        <p className="text-[10px] text-[#777777]">{r.desc}</p>
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
                className="w-7 h-7 rounded-full bg-[#ef4444] text-white flex items-center justify-center hover:bg-[#dc2626] transition-transform hover:scale-105 shadow-md cursor-pointer"
                title="Stop execution"
              >
                <Square className="w-3 h-3 fill-current" />
              </button>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={!prompt.trim() && attachedFiles.length === 0 && attachedImages.length === 0}
                className={`w-7 h-7 rounded-full flex items-center justify-center transition-all cursor-pointer shadow-md ${
                  !prompt.trim() && attachedFiles.length === 0 && attachedImages.length === 0
                    ? 'bg-[#333336] text-[#777777] opacity-60 cursor-not-allowed'
                    : 'bg-white text-black hover:scale-105'
                }`}
                title="Send task"
              >
                <ArrowUp className="w-4 h-4 stroke-[2.5]" />
              </button>
            )}

          </div>

        </div>

      </div>
    </div>
  );
};
