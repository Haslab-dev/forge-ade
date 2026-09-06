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
  Settings2,
  X,
  ShieldCheck
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentExecutionMode, AgentReasoningLevel } from '../../types';
import { ApiBridge } from '../../services/apiBridge';

interface AgentTaskInputBarProps {
  placeholder?: string;
  autoFocus?: boolean;
  onSubmitPrompt?: (prompt: string) => void;
  isCompact?: boolean;
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
  placeholder = 'Ask anything, @ to add context, / for commands or capabilities',
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
    mcps
  } = useWorkspace();

  const [prompt, setPrompt] = useState('');
  const [attachedFiles, setAttachedFiles] = useState<{ name: string; content: string }[]>([]);
  const [isContextOpen, setIsContextOpen] = useState(false);
  const [activeSubMenu, setActiveSubMenu] = useState<string | null>(null);
  const [isModeOpen, setIsModeOpen] = useState(false);
  const [isModelOpen, setIsModelOpen] = useState(false);
  const [isReasoningOpen, setIsReasoningOpen] = useState(false);

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
      const attachments = attachedFiles.map(f => `\n[Attached File: ${f.name}]\n${f.content}`).join('\n');
      fullPrompt = `${fullPrompt}\n${attachments}`.trim();
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

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      for (let i = 0; i < e.target.files.length; i++) {
        const file = e.target.files[i];
        try {
          const text = await file.text();
          setAttachedFiles(prev => [...prev, { name: file.name, content: text }]);
        } catch {
          setAttachedFiles(prev => [...prev, { name: file.name, content: `[File: ${file.name}]` }]);
        }
      }
      setIsContextOpen(false);
    }
  };

  const currentMode = MODES.find(m => m.id === agentExecutionMode) || MODES[3];
  const currentReasoning = REASONING_LEVELS.find(r => r.id === reasoningLevel) || REASONING_LEVELS[2];

  // Group available models by provider name
  const groupedModels = useMemo(() => {
    const groups: Array<{ providerName: string; models: string[] }> = [];
    
    // Add enabled providers' models
    for (const p of providers) {
      if (p.enabled && p.models && p.models.length > 0) {
        groups.push({
          providerName: p.name,
          models: p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models
        });
      }
    }

    // Default fallback groups if providers are empty
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

  return (
    <div className="w-full relative select-none" ref={containerRef}>
      <input 
        type="file" 
        multiple
        ref={fileInputRef} 
        onChange={handleFileUpload} 
        className="hidden" 
      />

      <div className={`w-full rounded-2xl border border-[#e5e7eb] dark:border-[#2d2d30] bg-[#ffffff] dark:bg-[#1a1a1c] shadow-xl transition-all duration-150 overflow-visible relative ${isCompact ? 'p-2.5' : 'p-3'}`}>
        
        {/* Attached Files Pills */}
        {attachedFiles.length > 0 && (
          <div className="flex flex-wrap gap-1.5 px-1 pb-2">
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
          placeholder={placeholder}
          className="w-full resize-none bg-transparent border-0 text-[13px] text-[#111827] dark:text-[#eeeeee] placeholder-[#9ca3af] dark:placeholder-[#777777] focus:outline-hidden leading-relaxed font-sans px-1"
        />

        {/* Bottom Control Bar */}
        <div className="pt-2 flex items-center justify-between gap-2 flex-wrap text-xs">
          
          {/* Left Controls: [+] [🛡 Full access ⌄] */}
          <div className="flex items-center gap-2 relative">
            
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
                        {files.filter(f => f.type === 'file').length === 0 ? (
                          <div className="px-3 py-2 text-xs text-[#777777]">No files loaded</div>
                        ) : (
                          files.filter(f => f.type === 'file').slice(0, 15).map(f => (
                            <button
                              key={f.id}
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
                    <Upload className="w-3.5 h-3.5 text-[#888888]" />
                    <span>Attach Local File</span>
                  </button>
                </div>
              )}
            </div>

            {/* Mode Selector Pill: 🛡 Full access ⌄ */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsModeOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsModelOpen(false);
                  setIsReasoningOpen(false);
                }}
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs text-[#dddddd] hover:text-white hover:bg-[#262629] transition-colors cursor-pointer group"
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

          {/* Right Controls: [⚪ GLM-5.3-Flash ⌄] [⚙ Max ⌄] [↑ Send] */}
          <div className="flex items-center gap-2 relative">
            
            {/* Model Selector Dropdown: ⚪ GLM-5.3-Flash ⌄ */}
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setIsModelOpen(prev => !prev);
                  setIsContextOpen(false);
                  setIsModeOpen(false);
                  setIsReasoningOpen(false);
                }}
                className="flex items-center gap-1.5 px-2 py-1 rounded-full text-xs text-[#cccccc] hover:text-white hover:bg-[#262629] transition-colors cursor-pointer group"
              >
                <span className="w-2 h-2 rounded-full bg-[#10b981] ring-2 ring-[#10b981]/20" />
                <span className="font-medium text-[#dddddd] truncate max-w-[120px]">{currentModel || 'GLM-5.3-Flash'}</span>
                <ChevronDown className="w-3 h-3 text-[#777777] group-hover:text-[#aaaaaa]" />
              </button>

              {/* Model Dropdown */}
              {isModelOpen && (
                <div className="absolute right-0 bottom-full mb-2 w-64 rounded-xl bg-white dark:bg-[#202022] shadow-2xl border border-[#e5e7eb] dark:border-[#333336] py-1.5 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs max-h-72 overflow-y-auto">
                  <div className="px-3 py-1 text-[10px] font-semibold text-[#6b7280] dark:text-[#777777] uppercase tracking-wider">
                    Select Model
                  </div>
                  {groupedModels.map(group => (
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
                              <span className="truncate">{mod}</span>
                            </div>
                            {isSelected && <Check className="w-3.5 h-3.5 text-[#3b82f6]" />}
                          </button>
                        );
                      })}
                    </div>
                  ))}
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
                disabled={!prompt.trim() && attachedFiles.length === 0}
                className={`w-7 h-7 rounded-full flex items-center justify-center transition-all cursor-pointer shadow-md ${
                  !prompt.trim() && attachedFiles.length === 0
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
