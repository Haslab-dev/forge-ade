import React from 'react';
import { 
  X, 
  FileText,
  Copy,
  Check,
  Code2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { OpenUIBlock } from '../agent/OpenUIBlock';

interface MarkdownPreviewProps {
  onBackToEditor?: () => void;
}

export const MarkdownPreview: React.FC<MarkdownPreviewProps> = ({ onBackToEditor }) => {
  const { openTabs, activeTabId, closeTab, selectedFile, activeWorkspacePath } = useWorkspace();
  const [copied, setCopied] = React.useState(false);

  const activeCodeTab = openTabs.find(t => t.id === activeTabId);
  const previewTab = openTabs.find(t => t.type === 'preview') || activeCodeTab;

  const content = previewTab?.content ?? selectedFile?.content ?? '# Welcome to Workspace\n\nOpen or pick a folder to begin editing files.';
  const fileName = previewTab?.fileName ?? selectedFile?.name ?? 'Preview';

  const handleCopy = () => {
    navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  // Simple clean markdown parser for headings, lists, codeblocks, bold, tables
  const renderMarkdown = (text: string) => {
    const lines = text.split('\n');
    const elements: React.ReactNode[] = [];
    let inCodeBlock = false;
    let codeBlockContent: string[] = [];
    let codeBlockLang = '';

    lines.forEach((line, index) => {
      if (line.startsWith('```')) {
        if (inCodeBlock) {
          const lower = codeBlockLang.toLowerCase();
          if (lower === 'opui' || lower === 'openui') {
            elements.push(
              <OpenUIBlock key={`opui-${index}`} code={codeBlockContent.join('\n')} streaming={false} />
            );
          } else {
            elements.push(
              <pre key={`code-${index}`} className="p-3.5 my-2.5 rounded-xl bg-surface text-foreground font-mono text-[13.5px]/[1.6] overflow-x-auto border border-border">
                <code>{codeBlockContent.join('\n')}</code>
              </pre>
            );
          }
          codeBlockContent = [];
          inCodeBlock = false;
        } else {
          inCodeBlock = true;
          codeBlockLang = line.replace('```', '').trim();
        }
        return;
      }

      if (inCodeBlock) {
        codeBlockContent.push(line);
        return;
      }

      if (line.startsWith('# ')) {
        elements.push(
          <h1 key={index} className="text-[22px] font-bold text-foreground pt-4 pb-1.5 border-b border-border tracking-tight leading-tight">
            {line.replace('# ', '')}
          </h1>
        );
      } else if (line.startsWith('## ')) {
        elements.push(
          <h2 key={index} className="text-[18px] font-bold text-foreground pt-3 pb-1 tracking-tight leading-snug">
            {line.replace('## ', '')}
          </h2>
        );
      } else if (line.startsWith('### ')) {
        elements.push(
          <h3 key={index} className="text-[16px] font-semibold text-foreground pt-2 pb-0.5 leading-snug">
            {line.replace('### ', '')}
          </h3>
        );
      } else if (line.startsWith('- ') || line.startsWith('* ')) {
        elements.push(
          <li key={index} className="text-[15px] text-foreground ml-4 list-disc py-0.5 leading-[1.65]">
            {line.replace(/^[-*]\s+/, '')}
          </li>
        );
      } else if (line.startsWith('> ')) {
        elements.push(
          <blockquote key={index} className="p-3 my-2 border-l-4 border-info/50 bg-surface/50 text-[14.5px] text-foreground-subtle rounded-r-lg leading-normal">
            {line.replace('> ', '')}
          </blockquote>
        );
      } else if (line.trim() === '') {
        elements.push(<div key={index} className="h-2" />);
      } else {
        elements.push(
          <p key={index} className="text-[15px] text-foreground leading-[1.65]">
            {line}
          </p>
        );
      }
    });

    return elements;
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-card text-fg-primary dark:text-foreground-secondary overflow-hidden transition-colors duration-150">
      {/* Preview Tab Bar */}
      <div className="h-[35px] min-h-[35px] bg-background dark:bg-[#252526] flex items-center justify-between border-b border-border px-2 select-none">
        <div className="flex items-center h-full">
          <div className="h-full flex items-center gap-2 px-3 text-xs bg-card text-foreground font-medium border-t-2 border-t-[#2563eb] dark:border-t-[#007acc] border-r border-border">
            <FileText className="w-3.5 h-3.5 text-primary" />
            <span className="truncate">{fileName} (Preview)</span>
            {previewTab && (
              <button
                type="button"
                onClick={() => closeTab(previewTab.id)}
                className="p-0.5 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-foreground-subtlest dark:text-foreground-subtle hover:text-foreground ml-1"
              >
                <X className="w-3 h-3" />
              </button>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2 text-foreground-subtlest dark:text-foreground-subtle">
          {onBackToEditor && (
            <button
              type="button"
              onClick={onBackToEditor}
              className="p-1 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-foreground-subtlest dark:text-foreground-subtle hover:text-foreground transition-colors flex items-center gap-1.5 text-ui-xs font-medium"
              title="Back to Editor"
            >
              <Code2 className="w-3.5 h-3.5 text-primary" />
              <span>Editor</span>
            </button>
          )}
          <button
            type="button"
            onClick={handleCopy}
            className="p-1 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-foreground-subtlest dark:text-foreground-subtle hover:text-foreground transition-colors flex items-center gap-1 text-ui-xs"
            title="Copy Markdown"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
            <span>Copy</span>
          </button>
          <span className="text-ui-xs px-1.5 py-0.5 rounded bg-surface-hover dark:bg-surface-hover text-foreground-subtle dark:text-foreground-subtle font-mono">
            Markdown Preview
          </span>
        </div>
      </div>

      {/* Rendered Markdown Body */}
      <div className="flex-1 overflow-y-auto p-6 md:p-8 bg-card font-sans selection:bg-[#bfdbfe]/60 dark:selection:bg-[#264f78] space-y-2">
        {renderMarkdown(content)}
      </div>
    </div>
  );
};
