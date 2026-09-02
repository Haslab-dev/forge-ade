import React, { useState } from 'react';
import { 
  Check, 
  X, 
  Columns, 
  FileText, 
  Sparkles, 
  RotateCcw, 
  GitCompare,
  ArrowRight
} from 'lucide-react';
import { FileDiff } from '../../types';
import { useWorkspace } from '../../stores/workspaceStore';

interface DiffViewerProps {
  diff: FileDiff;
  onClose?: () => void;
  isInline?: boolean;
}

export const DiffViewer: React.FC<DiffViewerProps> = ({ diff, onClose, isInline = false }) => {
  const { acceptDiff, rejectDiff, createNewSession, openFileInEditor } = useWorkspace();
  const [viewMode, setViewMode] = useState<'split' | 'unified'>('split');

  const origLines = diff.originalContent ? diff.originalContent.split('\n') : [];
  const modLines = diff.modifiedContent ? diff.modifiedContent.split('\n') : [];

  const maxLines = Math.max(origLines.length, modLines.length);

  const handleAccept = () => {
    acceptDiff(diff.id);
  };

  const handleReject = () => {
    rejectDiff(diff.id);
  };

  const handleAskRefine = () => {
    createNewSession(`Please refine the changes in ${diff.filePath}`);
  };

  return (
    <div className={`flex flex-col bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] rounded-2xl overflow-hidden shadow-xs font-sans ${
      isInline ? 'w-full' : 'h-full'
    }`}>
      {/* Header */}
      <div className="px-4 py-2.5 bg-[#f8fafc] dark:bg-[#252526] border-b border-[#e2e8f0] dark:border-[#2b2b2b] flex items-center justify-between flex-wrap gap-2 text-xs select-none">
        <div className="flex items-center gap-2">
          <GitCompare className="w-4 h-4 text-[#2563eb] dark:text-[#38bdf8]" />
          <span 
            onClick={() => openFileInEditor(diff.filePath)}
            className="font-bold text-[#0f172a] dark:text-white hover:underline cursor-pointer font-mono"
          >
            {diff.filePath}
          </span>
          <span className="flex items-center gap-1 font-mono text-[11px]">
            <span className="text-[#16a34a] dark:text-[#4ade80] font-semibold">+{diff.additions}</span>
            <span className="text-[#dc2626] dark:text-[#f87171] font-semibold">-{diff.deletions}</span>
          </span>
          <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold font-mono ${
            diff.status === 'accepted' 
              ? 'bg-[#ecfdf5] text-[#059669] dark:bg-[#064e3b] dark:text-[#a7f3d0]' 
              : diff.status === 'rejected'
              ? 'bg-[#fee2e2] text-[#dc2626] dark:bg-[#450a0a] dark:text-[#fca5a5]'
              : 'bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#93c5fd]'
          }`}>
            {diff.status.toUpperCase()}
          </span>
        </div>

        {/* View Controls & Action Buttons */}
        <div className="flex items-center gap-2">
          <div className="bg-[#e2e8f0] dark:bg-[#1e1e1e] p-0.5 rounded-lg flex items-center text-[11px]">
            <button
              type="button"
              onClick={() => setViewMode('split')}
              className={`px-2 py-0.5 rounded transition-colors ${
                viewMode === 'split' ? 'bg-white dark:bg-[#333333] text-[#0f172a] dark:text-white font-semibold shadow-2xs' : 'text-[#64748b] dark:text-[#a1a1aa]'
              }`}
            >
              Split
            </button>
            <button
              type="button"
              onClick={() => setViewMode('unified')}
              className={`px-2 py-0.5 rounded transition-colors ${
                viewMode === 'unified' ? 'bg-white dark:bg-[#333333] text-[#0f172a] dark:text-white font-semibold shadow-2xs' : 'text-[#64748b] dark:text-[#a1a1aa]'
              }`}
            >
              Unified
            </button>
          </div>

          <button
            type="button"
            onClick={handleAskRefine}
            className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-[#eff6ff] hover:bg-[#dbeafe] text-[#2563eb] dark:bg-[#1e293b] dark:hover:bg-[#28394f] dark:text-[#93c5fd] font-semibold text-xs transition-colors"
            title="Ask My-ADE to adjust this diff"
          >
            <Sparkles className="w-3.5 h-3.5" />
            <span>Refine</span>
          </button>

          {diff.status === 'pending' && (
            <>
              <button
                type="button"
                onClick={handleReject}
                className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-[#fee2e2] hover:bg-[#fecaca] text-[#dc2626] dark:bg-[#450a0a] dark:hover:bg-[#5c0d0d] dark:text-[#fca5a5] font-semibold text-xs transition-colors"
                title="Reject changes"
              >
                <X className="w-3.5 h-3.5" />
                <span>Reject</span>
              </button>

              <button
                type="button"
                onClick={handleAccept}
                className="flex items-center gap-1 px-3 py-1 rounded-lg bg-[#16a34a] hover:bg-[#15803d] text-white font-semibold text-xs transition-colors shadow-2xs"
                title="Accept and write to workspace"
              >
                <Check className="w-3.5 h-3.5" />
                <span>Accept</span>
              </button>
            </>
          )}

          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333333] rounded text-[#64748b] dark:text-[#a1a1aa]"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      {/* Diff Content Viewport */}
      <div className="flex-1 overflow-auto font-mono text-[12px] leading-[20px] select-text">
        {viewMode === 'split' ? (
          <div className="grid grid-cols-2 divide-x divide-[#e2e8f0] dark:divide-[#2b2b2b] min-w-[600px]">
            {/* Left: Original Content */}
            <div className="bg-[#fafafa] dark:bg-[#1a1a1a]">
              <div className="px-3 py-1 bg-[#f1f5f9] dark:bg-[#222222] text-[11px] font-bold text-[#64748b] dark:text-[#888] border-b border-[#e2e8f0] dark:border-[#2b2b2b] select-none">
                Original (Workspace)
              </div>
              <div className="p-2 space-y-0.5">
                {origLines.length === 0 ? (
                  <div className="text-[#9ca3af] italic px-2 py-4 select-none">[New File — No Original Content]</div>
                ) : (
                  origLines.map((line, idx) => {
                    const isRemoved = !modLines.includes(line);
                    return (
                      <div
                        key={idx}
                        className={`flex items-start ${
                          isRemoved ? 'bg-[#fee2e2]/60 dark:bg-[#450a0a]/50 text-[#991b1b] dark:text-[#fca5a5]' : 'text-[#334155] dark:text-[#cccccc]'
                        }`}
                      >
                        <span className="w-8 text-right pr-2 text-[#94a3b8] dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                          {idx + 1}
                        </span>
                        <span className="w-4 text-center select-none font-bold text-[#dc2626]">
                          {isRemoved ? '-' : ' '}
                        </span>
                        <span className="flex-1 whitespace-pre overflow-x-auto">{line || ' '}</span>
                      </div>
                    );
                  })
                )}
              </div>
            </div>

            {/* Right: Modified Content */}
            <div className="bg-white dark:bg-[#181818]">
              <div className="px-3 py-1 bg-[#f1f5f9] dark:bg-[#222222] text-[11px] font-bold text-[#16a34a] dark:text-[#4ade80] border-b border-[#e2e8f0] dark:border-[#2b2b2b] select-none">
                Proposed by Agent
              </div>
              <div className="p-2 space-y-0.5">
                {modLines.map((line, idx) => {
                  const isAdded = !origLines.includes(line);
                  return (
                    <div
                      key={idx}
                      className={`flex items-start ${
                        isAdded ? 'bg-[#dcfce7]/70 dark:bg-[#064e3b]/50 text-[#166534] dark:text-[#86efac]' : 'text-[#334155] dark:text-[#cccccc]'
                      }`}
                    >
                      <span className="w-8 text-right pr-2 text-[#94a3b8] dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                        {idx + 1}
                      </span>
                      <span className="w-4 text-center select-none font-bold text-[#16a34a]">
                        {isAdded ? '+' : ' '}
                      </span>
                      <span className="flex-1 whitespace-pre overflow-x-auto">{line || ' '}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        ) : (
          /* Unified Diff View */
          <div className="p-2 space-y-0.5 bg-white dark:bg-[#181818]">
            {modLines.map((line, idx) => {
              const isAdded = !origLines.includes(line);
              return (
                <div
                  key={idx}
                  className={`flex items-start ${
                    isAdded ? 'bg-[#dcfce7]/70 dark:bg-[#064e3b]/50 text-[#166534] dark:text-[#86efac]' : 'text-[#334155] dark:text-[#cccccc]'
                  }`}
                >
                  <span className="w-8 text-right pr-2 text-[#94a3b8] dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                    {idx + 1}
                  </span>
                  <span className="w-4 text-center select-none font-bold text-[#16a34a]">
                    {isAdded ? '+' : ' '}
                  </span>
                  <span className="flex-1 whitespace-pre overflow-x-auto">{line || ' '}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
