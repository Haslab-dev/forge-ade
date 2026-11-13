import React from 'react';
import { Split, Check, X, FileCode } from 'lucide-react';
import { cn } from '../../lib/utils';

export interface ModifiedFileItem {
  path: string;
  additions?: number;
  deletions?: number;
  status?: string;
}

interface MultiFileDiffReviewBarProps {
  files: ModifiedFileItem[];
  selectedPath?: string | null;
  onSelectFile: (path: string) => void;
  onAcceptAll?: () => void;
  onRejectAll?: () => void;
  className?: string;
}

export const MultiFileDiffReviewBar: React.FC<MultiFileDiffReviewBarProps> = ({
  files,
  selectedPath,
  onSelectFile,
  onAcceptAll,
  onRejectAll,
  className
}) => {
  if (!files || files.length === 0) return null;

  const totalAdditions = files.reduce((acc, f) => acc + (f.additions || 0), 0);
  const totalDeletions = files.reduce((acc, f) => acc + (f.deletions || 0), 0);

  return (
    <div
      role="region"
      aria-label="Multi-file diff review"
      className={cn(
        "flex flex-wrap items-center justify-between gap-2 border-t border-border bg-panel px-3 py-2 text-ui-xs shadow-xs select-none backdrop-blur-md transition-all",
        className
      )}
    >
      {/* Left: Summary and scrollable file list */}
      <div className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto scrollbar-none">
        <div className="flex shrink-0 items-center gap-1.5 font-medium text-foreground">
          <Split className="size-3.5 text-accent-primary" />
          <span>{files.length} {files.length === 1 ? 'file' : 'files'} changed</span>
          {(totalAdditions > 0 || totalDeletions > 0) && (
            <span className="text-ui-xs font-mono">
              <span className="text-success font-medium">+{totalAdditions}</span>{' '}
              <span className="text-destructive font-medium">-{totalDeletions}</span>
            </span>
          )}
        </div>

        <div className="h-3.5 w-px bg-border shrink-0" />

        {/* File Chips */}
        <div className="flex items-center gap-1.5 min-w-0">
          {files.map((file) => {
            const isSelected = selectedPath === file.path;
            const fileName = file.path.split('/').pop() || file.path;
            return (
              <button
                key={file.path}
                type="button"
                onClick={() => onSelectFile(file.path)}
                title={file.path}
                className={cn(
                  "flex items-center gap-1.5 rounded-md px-2 py-1 text-ui-xs transition-colors cursor-pointer shrink-0 border",
                  isSelected
                    ? "bg-surface-hover text-foreground border-border font-medium shadow-xs"
                    : "bg-surface text-foreground-subtle border-transparent hover:bg-surface-hover hover:text-foreground"
                )}
              >
                <FileCode className="size-3 text-foreground-subtle" />
                <span className="truncate max-w-[140px]">{fileName}</span>
                {typeof file.additions === 'number' && file.additions > 0 && (
                  <span className="text-success font-mono font-medium">+{file.additions}</span>
                )}
                {typeof file.deletions === 'number' && file.deletions > 0 && (
                  <span className="text-destructive font-mono font-medium">-{file.deletions}</span>
                )}
              </button>
            );
          })}
        </div>
      </div>

      {/* Right: Actions */}
      <div className="flex shrink-0 items-center gap-1.5">
        {onRejectAll && (
          <button
            type="button"
            onClick={onRejectAll}
            title="Reject all changes (Restore pre-turn checkpoint)"
            className="flex items-center gap-1 rounded-md px-2.5 py-1 text-destructive font-medium hover:bg-destructive/10 transition-colors cursor-pointer"
          >
            <X className="size-3" />
            <span>Reject All</span>
          </button>
        )}
        {onAcceptAll && (
          <button
            type="button"
            onClick={onAcceptAll}
            title="Accept all changes (Stage / Keep modifications)"
            className="flex items-center gap-1 rounded-md bg-accent-primary px-2.5 py-1 text-accent-primary-foreground font-medium hover:opacity-90 transition-opacity cursor-pointer shadow-xs"
          >
            <Check className="size-3" />
            <span>Accept All</span>
          </button>
        )}
      </div>
    </div>
  );
};

export default MultiFileDiffReviewBar;
