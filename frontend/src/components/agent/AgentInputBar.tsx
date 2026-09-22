import React, { useState, useRef, useEffect } from 'react';
import { 
  Plus, 
  Code2, 
  HelpCircle, 
  ListTree, 
  ShieldAlert, 
  ChevronRight, 
  FileText, 
  Folder, 
  GitBranch, 
  BookOpen, 
  Cpu, 
  MessageSquare, 
  Terminal, 
  Network, 
  Upload, 
  Slash, 
  Mic, 
  ArrowUp, 
  Square, 
  Check, 
  ChevronDown,
  Activity,
  X,
  Wrench,
  Laptop,
  ExternalLink,
  HardDrive,
  Image as ImageIcon
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { AgentExecutionMode } from '../../types';
import { ModelSwitcherDropdown } from './ModelSwitcherDropdown';

interface AgentInputBarProps {
  initialPrompt?: string;
  onSendPrompt?: (prompt: string) => void;
  placeholder?: string;
  autoFocus?: boolean;
}

const AGENT_MODES: { id: AgentExecutionMode; label: string; desc: string; icon: typeof Code2 }[] = [
  {
    id: 'code',
    label: 'Code',
    desc: 'Write and edit code',
    icon: Code2
  },
  {
    id: 'ask',
    label: 'Ask',
    desc: 'Answer question without code changes',
    icon: HelpCircle
  },
  {
    id: 'plan',
    label: 'Plan',
    desc: 'Plan changes before implementing',
    icon: ListTree
  },
  {
    id: 'bypass',
    label: 'Bypass Permissions',
    desc: 'Auto-approve all tool calls',
    icon: ShieldAlert
  }
];

export const AgentInputBar: React.FC<AgentInputBarProps> = ({
  initialPrompt = '',
  onSendPrompt,
  placeholder = 'Ask My-ADE to write code, test an endpoint, or explain files...',
  autoFocus = false
}) => {
  const { 
    sendAgentPrompt, 
    createNewSession, 
    activeSession, 
    stopAgentExecution,
    agentExecutionMode,
    setAgentExecutionMode,
    agents,
    activeAgentId,
    setActiveAgentId,
    activeAgent,
    contextUsage,
    files,
    skills,
    mcps,
    activeWorkspacePath,
    openFolder,
    setIsFolderModalOpen,
    recentWorkspaces,
    openSettingsTab
  } = useWorkspace();

  const [prompt, setPrompt] = useState(initialPrompt);
  const [attachedFiles, setAttachedFiles] = useState<{ name: string; path?: string; isImage?: boolean }[]>([]);
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [activeSubMenu, setActiveSubMenu] = useState<string | null>(null);
  const [isModeDropdownOpen, setIsModeDropdownOpen] = useState(false);
  const [isEngineDropdownOpen, setIsEngineDropdownOpen] = useState(false);
  const [isFolderDropdownOpen, setIsFolderDropdownOpen] = useState(false);
  const [isContextTooltipOpen, setIsContextTooltipOpen] = useState(false);
  const [isRecording, setIsRecording] = useState(false);

  const containerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Sync external initialPrompt
  useEffect(() => {
    if (initialPrompt) {
      setPrompt(initialPrompt);
    }
  }, [initialPrompt]);

  // Click outside listener for all popovers
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsMenuOpen(false);
        setActiveSubMenu(null);
        setIsModeDropdownOpen(false);
        setIsEngineDropdownOpen(false);
        setIsFolderDropdownOpen(false);
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

    // If user types '@' as the last character, open the context popover!
    if (val.endsWith('@')) {
      setIsMenuOpen(true);
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

    if (onSendPrompt) {
      onSendPrompt(fullPrompt);
    } else if (activeSession) {
      sendAgentPrompt(fullPrompt);
    } else {
      createNewSession(fullPrompt);
    }

    setPrompt('');
    setAttachedFiles([]);
    setIsMenuOpen(false);
    setActiveSubMenu(null);
  };

  const handleVoiceInput = () => {
    if (typeof window !== 'undefined' && ('webkitSpeechRecognition' in window || 'SpeechRecognition' in window)) {
      const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
      const recognition = new SpeechRecognition();
      recognition.lang = 'en-US';
      recognition.onstart = () => setIsRecording(true);
      recognition.onresult = (event: any) => {
        const transcript = event.results[0][0].transcript;
        setPrompt(prev => (prev ? `${prev} ${transcript}` : transcript));
        setIsRecording(false);
      };
      recognition.onerror = () => setIsRecording(false);
      recognition.onend = () => setIsRecording(false);
      recognition.start();
    } else {
      setIsRecording(true);
      setTimeout(() => {
        setPrompt(prev => (prev ? `${prev} Analyze workspace structure and optimize code` : 'Analyze workspace structure and optimize code'));
        setIsRecording(false);
        textareaRef.current?.focus();
      }, 1000);
    }
  };

  const insertContextTag = (tag: string) => {
    setPrompt(prev => {
      const cleanPrev = prev.endsWith('@') ? prev.slice(0, -1) : prev;
      return `${cleanPrev} @${tag} `.trimStart();
    });
    setIsMenuOpen(false);
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
      setIsMenuOpen(false);
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
        setIsMenuOpen(false);
        return;
      }
    } catch {
      // fallback to html file input
    }
    fileInputRef.current?.click();
  };

  const currentModeConfig = AGENT_MODES.find(m => m.id === agentExecutionMode) || AGENT_MODES[0];
  const isRunning = activeSession?.status === 'running';

  return (
    <div className="w-full relative" ref={containerRef}>
      {/* Hidden file input for upload */}
      <input 
        type="file" 
        multiple
        ref={fileInputRef} 
        onChange={handleFileUpload} 
        className="hidden" 
      />

      {/* Main Card Container */}
      <div className="w-full rounded-2xl border border-border dark:border-border bg-white dark:bg-card shadow-sm hover:shadow-md transition-shadow duration-200 overflow-visible relative">
        
        {/* Tip Header */}
        <div className="px-4 pt-3 pb-1 text-[13px] text-foreground-subtle dark:text-[#737373] flex items-center justify-between">
          <div>
            Tip: Type <span className="text-foreground-subtle dark:text-foreground-subtle font-mono font-medium">@</span> to bring in files, rules, or skills
          </div>
          {contextUsage.percent > 0 && (
            <div className="text-ui-xs font-mono text-foreground-subtle dark:text-[#737373] hidden sm:block">
              {contextUsage.percent}% context ({contextUsage.usedTokens.toLocaleString()} tokens)
            </div>
          )}
        </div>

        {/* Attached Files Chips */}
        {attachedFiles.length > 0 && (
          <div className="flex flex-wrap gap-1.5 px-4 pt-2">
            {attachedFiles.map((file, idx) => (
              <div 
                key={idx} 
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-primary/10 dark:bg-card text-primary dark:text-info text-xs font-mono border border-[#bfdbfe] dark:border-[#1e3a8a]"
                title={file.path || file.name}
              >
                {file.isImage ? (
                  <ImageIcon className="w-3.5 h-3.5 shrink-0" />
                ) : (
                  <FileText className="w-3.5 h-3.5 shrink-0" />
                )}
                <span className="truncate max-w-[150px]">{file.name}</span>
                <button
                  type="button"
                  onClick={() => setAttachedFiles(prev => prev.filter((_, i) => i !== idx))}
                  className="text-foreground-subtlest hover:text-destructive ml-1 cursor-pointer"
                  title="Remove attachment"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Prompt Textarea */}
        <div className="px-4 py-2">
          <textarea
            ref={textareaRef}
            rows={2}
            autoFocus={autoFocus}
            value={prompt}
            onChange={handleTextChange}
            onKeyDown={handleKeyDown}
            placeholder={!activeWorkspacePath ? 'Choose a workspace folder to start chatting...' : placeholder}
            className="w-full resize-none bg-transparent border-0 text-[14px] text-foreground dark:text-[#f3f4f6] placeholder-foreground-subtle dark:placeholder-foreground-subtle focus:outline-hidden leading-relaxed font-sans"
          />
        </div>

        {/* Bottom Control Bar inside Input Card */}
        <div className="px-3 pb-3 flex items-center justify-between gap-2 flex-wrap">
          
          {/* Left Controls: [+] [Code Mode] [Model Switcher] */}
          <div className="flex items-center gap-2 relative">
            
            {/* 1. [+] Context / Attachment Trigger Button */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsMenuOpen(prev => !prev);
                  setActiveSubMenu(null);
                }}
                className={`w-7 h-7 rounded-lg border border-border dark:border-border flex items-center justify-center transition-colors cursor-pointer ${
                  isMenuOpen 
                    ? 'bg-primary/10 dark:bg-card text-primary dark:text-info border-primary' 
                    : 'bg-white dark:bg-[#252526] hover:bg-surface dark:hover:bg-[#2e2e2e] text-foreground-subtle dark:text-foreground-subtle hover:text-foreground dark:hover:text-white'
                }`}
                title="Add context (files, directories, skills, MCPs, rules)"
              >
                <Plus className="w-4 h-4" />
              </button>

              {/* Context Popover Menu */}
              {isMenuOpen && (
                <div className="absolute left-0 bottom-full mb-2 w-56 rounded-2xl bg-white dark:bg-card shadow-2xl border border-border dark:border-border py-2 z-50 animate-in fade-in zoom-in-95 duration-100 font-sans text-xs">
                  
                  {/* Files > */}
                  <div className="relative group">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('files')}
                      onClick={() => setActiveSubMenu(prev => prev === 'files' ? null : 'files')}
                      className="w-full text-left px-3.5 py-2 text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center justify-between transition-colors cursor-pointer"
                    >
                      <div className="flex items-center gap-2.5">
                        <FileText className="w-4 h-4 text-foreground-subtle dark:text-foreground-subtle" />
                        <span className="font-medium">Files</span>
                      </div>
                      <ChevronRight className="w-3.5 h-3.5 text-foreground-subtle" />
                    </button>

                    {/* Files Submenu */}
                    {activeSubMenu === 'files' && (
                      <div className="absolute left-full top-0 ml-1 w-56 rounded-xl bg-white dark:bg-card shadow-xl border border-border dark:border-border py-1.5 z-50 max-h-56 overflow-y-auto">
                        {files.filter(f => f.type === 'file').length === 0 ? (
                          <div className="px-3 py-2 text-xs text-foreground-subtle">No files found</div>
                        ) : (
                          files.filter(f => f.type === 'file').map(f => (
                            <button
                              key={f.id || f.path}
                              type="button"
                              onClick={() => insertContextTag(`file:${f.path}`)}
                              className="w-full text-left px-3 py-1.5 text-xs text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover truncate font-mono cursor-pointer"
                            >
                              {f.name}
                            </button>
                          ))
                        )}
                      </div>
                    )}
                  </div>

                  {/* Directories > */}
                  <div className="relative group">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('directories')}
                      onClick={() => setActiveSubMenu(prev => prev === 'directories' ? null : 'directories')}
                      className="w-full text-left px-3.5 py-2 text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center justify-between transition-colors cursor-pointer"
                    >
                      <div className="flex items-center gap-2.5">
                        <Folder className="w-4 h-4 text-[#dcb67a]" />
                        <span className="font-medium">Directories</span>
                      </div>
                      <ChevronRight className="w-3.5 h-3.5 text-foreground-subtle" />
                    </button>

                    {/* Directories Submenu */}
                    {activeSubMenu === 'directories' && (
                      <div className="absolute left-full top-0 ml-1 w-48 rounded-xl bg-white dark:bg-card shadow-xl border border-border dark:border-border py-1.5 z-50">
                        {files.filter(f => f.type === 'folder').length === 0 ? (
                          <div className="px-3 py-2 text-xs text-foreground-subtle">No directories</div>
                        ) : (
                          files.filter(f => f.type === 'folder').map(d => (
                            <button
                              key={d.id || d.name}
                              type="button"
                              onClick={() => insertContextTag(`dir:${d.name}`)}
                              className="w-full text-left px-3 py-1.5 text-xs text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center gap-1.5 cursor-pointer"
                            >
                              <Folder className="w-3.5 h-3.5 text-[#dcb67a]" />
                              <span>{d.name}/</span>
                            </button>
                          ))
                        )}
                      </div>
                    )}
                  </div>

                  {/* Skills > */}
                  <div className="relative group">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('skills')}
                      onClick={() => setActiveSubMenu(prev => prev === 'skills' ? null : 'skills')}
                      className="w-full text-left px-3.5 py-2 text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center justify-between transition-colors cursor-pointer"
                    >
                      <div className="flex items-center gap-2.5">
                        <Cpu className="w-4 h-4 text-primary" />
                        <span className="font-medium">Skills ({skills.length})</span>
                      </div>
                      <ChevronRight className="w-3.5 h-3.5 text-foreground-subtle" />
                    </button>

                    {activeSubMenu === 'skills' && (
                      <div className="absolute left-full top-0 ml-1 w-52 rounded-xl bg-white dark:bg-card shadow-xl border border-border dark:border-border py-1.5 z-50">
                        {skills.length === 0 ? (
                          <div className="px-3 py-2 text-xs text-foreground-subtle">No skills loaded</div>
                        ) : (
                          skills.map(s => (
                            <button
                              key={s.id}
                              type="button"
                              onClick={() => insertContextTag(`skill:${s.name}`)}
                              className="w-full text-left px-3 py-1.5 text-xs text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center gap-1.5 truncate cursor-pointer"
                            >
                              <Cpu className="w-3.5 h-3.5 text-primary shrink-0" />
                              <span className="truncate">{s.name}</span>
                            </button>
                          ))
                        )}
                      </div>
                    )}
                  </div>

                  {/* MCP Servers > */}
                  <div className="relative group">
                    <button
                      type="button"
                      onMouseEnter={() => setActiveSubMenu('mcps')}
                      onClick={() => setActiveSubMenu(prev => prev === 'mcps' ? null : 'mcps')}
                      className="w-full text-left px-3.5 py-2 text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center justify-between transition-colors cursor-pointer"
                    >
                      <div className="flex items-center gap-2.5">
                        <Wrench className="w-4 h-4 text-[#ec4899]" />
                        <span className="font-medium">MCP Tools ({mcps.length})</span>
                      </div>
                      <ChevronRight className="w-3.5 h-3.5 text-foreground-subtle" />
                    </button>

                    {activeSubMenu === 'mcps' && (
                      <div className="absolute left-full top-0 ml-1 w-52 rounded-xl bg-white dark:bg-card shadow-xl border border-border dark:border-border py-1.5 z-50">
                        {mcps.length === 0 ? (
                          <div className="px-3 py-2 text-xs text-foreground-subtle">No MCPs configured</div>
                        ) : (
                          mcps.map(m => (
                            <button
                              key={m.id}
                              type="button"
                              onClick={() => insertContextTag(`mcp:${m.name}`)}
                              className="w-full text-left px-3 py-1.5 text-xs text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center gap-1.5 truncate cursor-pointer"
                            >
                              <Wrench className="w-3.5 h-3.5 text-[#ec4899] shrink-0" />
                              <span className="truncate">{m.name}</span>
                            </button>
                          ))
                        )}
                      </div>
                    )}
                  </div>

                  {/* Git */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('git:diff')}
                    className="w-full text-left px-3.5 py-2 text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center gap-2.5 transition-colors cursor-pointer"
                  >
                    <GitBranch className="w-4 h-4 text-info" />
                    <span className="font-medium">Git (Branch & Diffs)</span>
                  </button>

                  {/* Terminal */}
                  <button
                    type="button"
                    onClick={() => insertContextTag('terminal:stdout')}
                    className="w-full text-left px-3.5 py-2 text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center gap-2.5 transition-colors cursor-pointer"
                  >
                    <Terminal className="w-4 h-4 text-warning" />
                    <span className="font-medium">Terminal Output</span>
                  </button>

                  <div className="h-[1px] bg-surface-hover dark:bg-surface-hover my-1" />

                  {/* Upload files */}
                  <button
                    type="button"
                    onClick={handleNativeFileUpload}
                    className="w-full text-left px-3.5 py-2 text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center gap-2.5 transition-colors cursor-pointer"
                  >
                    <Upload className="w-4 h-4 text-foreground-subtle" />
                    <span className="font-medium">Attach Local File(s)</span>
                  </button>
                </div>
              )}
            </div>

            {/* 2. [<> Code] Agent Mode Selector Button */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setIsModeDropdownOpen(prev => !prev)}
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border dark:border-border bg-white dark:bg-[#252526] hover:bg-surface dark:hover:bg-[#2e2e2e] text-xs text-foreground-subtle dark:text-foreground-secondary font-medium transition-all shadow-2xs group cursor-pointer"
                title={`Agent Execution Mode: ${currentModeConfig.label}`}
              >
                <currentModeConfig.icon className="w-3.5 h-3.5 text-info" />
                <span className="font-semibold text-foreground dark:text-white">{currentModeConfig.label}</span>
                <ChevronDown className="w-3 h-3 text-foreground-subtle group-hover:text-foreground-subtle" />
              </button>

              {/* Mode Selector Popover Dropdown */}
              {isModeDropdownOpen && (
                <div className="absolute left-0 bottom-full mb-2 w-72 rounded-2xl bg-white dark:bg-[#222224] shadow-2xl border border-border dark:border-border py-2 z-50 animate-in fade-in zoom-in-95 duration-100 font-sans text-xs">
                  <div className="px-3.5 py-1.5 border-b border-border dark:border-border flex items-center justify-between">
                    <span className="text-ui-xs font-semibold text-foreground-subtle dark:text-foreground-subtle uppercase tracking-wider">
                      Execution Mode
                    </span>
                  </div>

                  <div className="py-1">
                    {AGENT_MODES.map(m => {
                      const IconComp = m.icon;
                      const isSelected = m.id === agentExecutionMode;
                      return (
                        <button
                          key={m.id}
                          type="button"
                          onClick={() => {
                            setAgentExecutionMode(m.id);
                            setIsModeDropdownOpen(false);
                          }}
                          className={`w-full text-left px-3.5 py-2 flex items-start gap-2.5 transition-colors cursor-pointer ${
                            isSelected
                              ? 'bg-primary/10 dark:bg-card/60 text-[#1e40af] dark:text-[#93c5fd]'
                              : 'text-foreground-subtle dark:text-foreground-secondary hover:bg-surface dark:hover:bg-surface-hover'
                          }`}
                        >
                          <IconComp className="w-4 h-4 text-info shrink-0 mt-0.5" />
                          <div className="flex-1">
                            <div className="font-semibold text-foreground dark:text-white flex items-center justify-between">
                              <span>{m.label}</span>
                              {isSelected && <Check className="w-3.5 h-3.5 text-primary" />}
                            </div>
                            <p className="text-ui-xs text-foreground-subtle dark:text-foreground-subtle leading-tight mt-0.5">
                              {m.desc}
                            </p>
                          </div>
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>

            {/* 3. Real Synchronized Model Selector Dropdown */}
            <ModelSwitcherDropdown />

          </div>

          {/* Right Controls: [Round Progress] [Forge Local] [Mic] [Submit ↑] */}
          <div className="flex items-center gap-2.5">
            
            {/* 4. [Round Progress Ring] (% context used) */}
            <div 
              className="relative cursor-pointer"
              onMouseEnter={() => setIsContextTooltipOpen(true)}
              onMouseLeave={() => setIsContextTooltipOpen(false)}
            >
              <div className="w-6 h-6 flex items-center justify-center relative">
                <svg className="w-5 h-5 -rotate-90" viewBox="0 0 36 36">
                  <path
                    className="text-foreground-secondary dark:text-foreground-subtle"
                    strokeWidth="4"
                    stroke="currentColor"
                    fill="none"
                    d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                  />
                  <path
                    className="text-info"
                    strokeDasharray={`${contextUsage.percent}, 100`}
                    strokeWidth="4"
                    strokeLinecap="round"
                    stroke="currentColor"
                    fill="none"
                    d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                  />
                </svg>
              </div>

              {/* Context Tooltip Popover */}
              {isContextTooltipOpen && (
                <div className="absolute right-0 bottom-full mb-2 w-64 rounded-xl bg-[#0f172a] text-white p-3 shadow-xl z-50 text-xs font-sans space-y-1.5 animate-in fade-in">
                  <div className="font-semibold border-b border-[#334155] pb-1 flex justify-between">
                    <span>Context Window Usage</span>
                    <span className="text-info">{contextUsage.percent}%</span>
                  </div>
                  <div className="text-ui-xs space-y-1">
                    <div className="flex justify-between text-foreground-subtlest">
                       <span>Used Tokens:</span>
                       <span className="text-white font-medium">{contextUsage.usedTokens.toLocaleString()}</span>
                    </div>
                    <div className="flex justify-between text-foreground-subtlest">
                       <span>Max Tokens:</span>
                       <span className="text-white font-medium">{contextUsage.maxTokens.toLocaleString()}</span>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* 5. Agent Engine Selector */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setIsEngineDropdownOpen(prev => !prev)}
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border dark:border-border bg-white dark:bg-[#252526] hover:bg-surface dark:hover:bg-[#2e2e2e] text-xs text-foreground-subtle dark:text-foreground-secondary font-medium transition-all shadow-2xs group cursor-pointer"
                title={`Agent: ${activeAgent.name}`}
              >
                <div className="flex items-center gap-1">
                  <Activity className="w-3.5 h-3.5 text-[#10b981]" />
                  <span className="font-semibold text-foreground dark:text-white">{activeAgent.name}</span>
                </div>
                <ChevronDown className="w-3 h-3 text-foreground-subtle group-hover:text-foreground-subtle" />
              </button>

              {/* Engine Selector Dropdown */}
              {isEngineDropdownOpen && (
                <div className="absolute right-0 bottom-full mb-2 w-64 rounded-2xl bg-white dark:bg-[#222224] shadow-2xl border border-border dark:border-border py-2 z-50 animate-in fade-in zoom-in-95 duration-100 font-sans text-xs">
                  <div className="px-3.5 py-1.5 border-b border-border dark:border-border flex items-center justify-between">
                    <span className="text-ui-xs font-semibold text-foreground-subtle dark:text-foreground-subtle uppercase tracking-wider">
                      Connected Agents
                    </span>
                  </div>

                  <div className="py-1">
                    {agents.filter(a => a.id === 'agent-internal' || a.status === 'connected').map(agentItem => {
                      const isSelected = agentItem.id === activeAgentId;
                      return (
                        <button
                          key={agentItem.id}
                          type="button"
                          onClick={() => {
                            setActiveAgentId(agentItem.id);
                            setIsEngineDropdownOpen(false);
                          }}
                          className={`w-full text-left px-3.5 py-2 flex items-center justify-between transition-colors cursor-pointer ${
                            isSelected
                              ? 'bg-primary/10 dark:bg-card/60 text-[#1e40af] dark:text-[#93c5fd] font-semibold'
                              : 'text-foreground-subtle dark:text-foreground-secondary hover:bg-surface dark:hover:bg-surface-hover'
                          }`}
                        >
                          <div className="flex items-center gap-2">
                            <span className="w-2 h-2 rounded-full bg-[#10b981]" />
                            <span className="font-semibold">{agentItem.name}</span>
                          </div>
                          {isSelected && <Check className="w-4 h-4 text-primary shrink-0" />}
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>

            {/* 6. Voice / Mic Trigger Button */}
            <button
              type="button"
              onClick={handleVoiceInput}
              className={`w-7 h-7 rounded-lg flex items-center justify-center transition-colors cursor-pointer ${
                isRecording
                  ? 'bg-destructive/10 text-destructive animate-pulse border border-[#ef4444]'
                  : 'bg-white dark:bg-[#252526] hover:bg-surface dark:hover:bg-[#2e2e2e] text-foreground-subtle dark:text-foreground-subtle hover:text-foreground dark:hover:text-white border border-border dark:border-border'
              }`}
              title="Voice dictation"
            >
              <Mic className="w-3.5 h-3.5" />
            </button>

            {/* 7. [↑ Submit] or [■ Stop] Button */}
            {isRunning ? (
              <button
                type="button"
                onClick={stopAgentExecution}
                className="w-7 h-7 rounded-lg bg-[#ef4444] text-white flex items-center justify-center hover:bg-[#dc2626] transition-colors shadow-2xs cursor-pointer"
                title="Stop generation"
              >
                <Square className="w-3 h-3 fill-current" />
              </button>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={!prompt.trim() && attachedFiles.length === 0}
                className={`w-7 h-7 rounded-lg flex items-center justify-center transition-opacity shadow-2xs cursor-pointer ${
                  !prompt.trim() && attachedFiles.length === 0
                    ? 'bg-[#111827] dark:bg-white text-white dark:text-foreground opacity-30 cursor-not-allowed'
                    : 'bg-[#111827] dark:bg-white text-white dark:text-foreground hover:opacity-90'
                }`}
                title="Send Prompt (Enter)"
              >
                <ArrowUp className="w-3.5 h-3.5" />
              </button>
            )}

          </div>

        </div>

        {/* Integrated Sub-bar: [💻 Local] [📁 Choose folder / folder-name ▾] [Go to agent manager ↗] */}
        <div className="px-3.5 py-2.5 bg-surface dark:bg-[#18181a] border-t border-border dark:border-border rounded-b-2xl flex items-center justify-between text-xs text-foreground-subtle dark:text-foreground-subtle">
          <div className="flex items-center gap-2 relative">
            {/* 💻 Local badge */}
            <div className="flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-background dark:bg-surface-hover text-ui-xs font-medium text-[#475569] dark:text-foreground-secondary border border-border dark:border-border">
              <Laptop className="w-3 h-3 text-info" />
              <span>Local</span>
            </div>

            {/* 📁 Folder Picker / Dropdown */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setIsFolderDropdownOpen(prev => !prev)}
                className={`flex items-center gap-1.5 px-2 py-0.5 rounded-md text-ui-xs font-medium transition-colors cursor-pointer border ${
                  activeWorkspacePath
                    ? 'bg-white dark:bg-surface-hover text-foreground dark:text-white border-border dark:border-border hover:border-primary'
                    : 'bg-primary/10 dark:bg-card text-primary dark:text-info border-[#bfdbfe] dark:border-[#1e3a5f] hover:bg-primary/10 font-semibold'
                }`}
                title={activeWorkspacePath || 'Select a workspace folder'}
              >
                <Folder className={`w-3 h-3 ${activeWorkspacePath ? 'text-[#eab308]' : 'text-primary dark:text-info'}`} />
                <span className="truncate max-w-[130px] sm:max-w-[200px]">
                  {activeWorkspacePath ? activeWorkspacePath.split('/').pop() : 'Choose folder'}
                </span>
                <ChevronDown className="w-3 h-3 text-foreground-subtle" />
              </button>

              {/* Folder Quick Dropdown Menu */}
              {isFolderDropdownOpen && (
                <div className="absolute left-0 bottom-full mb-1.5 w-64 rounded-xl bg-white dark:bg-[#222224] shadow-2xl border border-border dark:border-border py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 font-sans text-xs">
                  <div className="px-3 py-1 text-[10px] font-semibold text-foreground-subtle uppercase tracking-wider border-b border-border dark:border-border">
                    Workspace Folder
                  </div>
                  
                  {/* Pick new folder directly */}
                  <button
                    type="button"
                    onClick={async () => {
                      setIsFolderDropdownOpen(false);
                      try {
                        const picked = await ApiBridge.pickNativeDirectory();
                        if (picked && picked.path) {
                          await openFolder(picked.path);
                        }
                      } catch {
                        openFolder();
                      }
                    }}
                    className="w-full text-left px-3 py-2 text-primary dark:text-info hover:bg-primary/10 dark:hover:bg-card flex items-center gap-2 font-medium cursor-pointer"
                  >
                    <HardDrive className="w-3.5 h-3.5" />
                    <span>Choose folder from computer...</span>
                  </button>

                  {/* Recent Workspaces History */}
                  {recentWorkspaces.length > 0 && (
                    <>
                      <div className="px-3 pt-1.5 pb-0.5 text-[10px] font-semibold text-foreground-subtle uppercase tracking-wider border-t border-border dark:border-border">
                        Recent History
                      </div>
                      <div className="max-h-36 overflow-y-auto py-0.5">
                        {recentWorkspaces.slice(0, 6).map((rPath, idx) => {
                          const fName = rPath.split('/').pop() || rPath;
                          const isCur = rPath === activeWorkspacePath;
                          return (
                            <button
                              key={idx}
                              type="button"
                              onClick={async () => {
                                setIsFolderDropdownOpen(false);
                                await openFolder(rPath);
                              }}
                              className={`w-full text-left px-3 py-1.5 text-xs flex items-center justify-between cursor-pointer ${
                                isCur
                                  ? 'bg-primary/10 dark:bg-card/70 text-primary dark:text-info font-medium'
                                  : 'text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover'
                              }`}
                            >
                              <div className="flex items-center gap-2 truncate">
                                <Folder className="w-3 h-3 text-[#eab308] shrink-0" />
                                <span className="truncate">{fName}</span>
                              </div>
                              {isCur && <Check className="w-3 h-3 text-primary shrink-0" />}
                            </button>
                          );
                        })}
                      </div>
                    </>
                  )}

                  <div className="border-t border-border dark:border-border pt-1">
                    <button
                      type="button"
                      onClick={() => {
                        setIsFolderDropdownOpen(false);
                        setIsFolderModalOpen(true);
                      }}
                      className="w-full text-left px-3 py-1.5 text-ui-xs text-foreground-subtle dark:text-foreground-subtle hover:bg-surface-hover dark:hover:bg-surface-hover flex items-center justify-between cursor-pointer"
                    >
                      <span>Manage all workspaces...</span>
                      <ExternalLink className="w-3 h-3 text-foreground-subtle" />
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Right: Go to agent manager ↗ */}
          <button
            type="button"
            onClick={() => openSettingsTab('agents')}
            className="flex items-center gap-1 text-ui-xs text-foreground-subtle dark:text-foreground-subtle hover:text-foreground dark:hover:text-white transition-colors cursor-pointer group"
          >
            <span>Go to agent manager</span>
            <ExternalLink className="w-3 h-3 group-hover:translate-x-0.5 group-hover:-translate-y-0.5 transition-transform" />
          </button>
        </div>

      </div>
    </div>
  );
};
