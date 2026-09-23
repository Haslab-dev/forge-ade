import React, { useState } from 'react';
import { Copy, Check } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

interface MarkdownRendererProps {
  content: string;
}

/**
 * Chat markdown rendering with the reference streamdown look: full-contrast
 * 14px body, subtle neutral inline-code chips (no borders, no link blues),
 * modest semibold headings, neutral code blocks.
 */
export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content }) => {
  const { openFileInEditor } = useWorkspace();
  const [copiedBlock, setCopiedBlock] = useState<number | null>(null);

  const handleCopyCode = (code: string, blockIdx: number) => {
    navigator.clipboard.writeText(code);
    setCopiedBlock(blockIdx);
    setTimeout(() => setCopiedBlock(null), 1500);
  };

  // Parse inline markdown tokens: `code`, **bold**, *italic*, [link](url)
  const renderInline = (text: string) => {
    const parts: React.ReactNode[] = [];
    let current = text;
    let key = 0;

    while (current.length > 0) {
      // 1. Inline code: `something`
      const codeMatch = current.match(/^`([^`]+)`/);
      if (codeMatch) {
        const codeText = codeMatch[1];
        const isFilePath = codeText.includes('/') || /\.(ts|tsx|js|json|zon|zig|html|css|md|go|py|rs)$/.test(codeText);

        parts.push(
          <code
            key={key++}
            onClick={() => (isFilePath ? openFileInEditor(codeText) : undefined)}
            className={`rounded-md bg-surface px-1 py-0.5 font-mono text-[0.9em] text-foreground break-all [overflow-wrap:anywhere] ${
              isFilePath ? 'cursor-pointer hover:bg-surface-hover' : ''
            }`}
          >
            {codeText}
          </code>
        );
        current = current.slice(codeMatch[0].length);
        continue;
      }

      // 2. Links: [text](url)
      const linkMatch = current.match(/^\[([^\]]+)\]\(([^)\s]+)\)/);
      if (linkMatch) {
        parts.push(
          <a
            key={key++}
            href={linkMatch[2]}
            target="_blank"
            rel="noreferrer"
            className="text-info underline decoration-info/40 underline-offset-2 hover:decoration-info break-words [overflow-wrap:anywhere]"
          >
            {renderInline(linkMatch[1])}
          </a>
        );
        current = current.slice(linkMatch[0].length);
        continue;
      }

      // 3. Bold: **text** or __text__
      const boldMatch = current.match(/^(\*\*|__)(.*?)\1/);
      if (boldMatch) {
        parts.push(
          <strong key={key++} className="font-semibold text-foreground break-words [overflow-wrap:anywhere]">
            {boldMatch[2]}
          </strong>
        );
        current = current.slice(boldMatch[0].length);
        continue;
      }

      // 4. Italic: *text* or _text_
      const italicMatch = current.match(/^(\*|_)(.*?)\1/);
      if (italicMatch) {
        parts.push(
          <em key={key++} className="italic text-foreground break-words [overflow-wrap:anywhere]">
            {italicMatch[2]}
          </em>
        );
        current = current.slice(italicMatch[0].length);
        continue;
      }

      // 5. Regular characters
      const nextSpecial = current.search(/[`*_\[]/);
      if (nextSpecial === -1) {
        parts.push(current);
        break;
      } else if (nextSpecial === 0) {
        parts.push(current[0]);
        current = current.slice(1);
      } else {
        parts.push(current.slice(0, nextSpecial));
        current = current.slice(nextSpecial);
      }
    }

    return parts;
  };

  const renderBlocks = () => {
    const lines = content.split('\n');
    const nodes: React.ReactNode[] = [];
    let inCode = false;
    let codeLines: string[] = [];
    let codeLang = '';
    let blockIndex = 0;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];

      // Code block start / end
      if (line.startsWith('```')) {
        if (inCode) {
          const fullCode = codeLines.join('\n');
          const currentIdx = blockIndex++;
          nodes.push(
            <div key={`code-block-${i}`} className="my-3 w-full max-w-full min-w-0 overflow-hidden rounded-xl border border-border bg-card">
              <div className="flex items-center justify-between border-b border-border px-3.5 py-1.5 font-mono text-ui-xs text-foreground-subtlest">
                <span>{codeLang || 'text'}</span>
                <button
                  type="button"
                  onClick={() => handleCopyCode(fullCode, currentIdx)}
                  className="flex cursor-pointer items-center gap-1 rounded px-2 py-0.5 text-foreground-subtlest transition-colors hover:bg-surface-hover hover:text-foreground"
                >
                  {copiedBlock === currentIdx ? (
                    <>
                      <Check className="size-3 text-success" />
                      <span className="text-success">Copied</span>
                    </>
                  ) : (
                    <>
                      <Copy className="size-3" />
                      <span>Copy</span>
                    </>
                  )}
                </button>
              </div>
              <pre className="max-w-full min-w-0 overflow-x-auto p-3.5 font-mono text-ui-sm/[1.6] text-foreground">
                <code>{fullCode}</code>
              </pre>
            </div>
          );
          inCode = false;
          codeLines = [];
          codeLang = '';
        } else {
          inCode = true;
          codeLang = line.slice(3).trim();
        }
        continue;
      }

      if (inCode) {
        codeLines.push(line);
        continue;
      }

      // Markdown table
      if (line.trim().startsWith('|') && line.trim().endsWith('|')) {
        const tableLines: string[] = [];
        while (i < lines.length && lines[i].trim().startsWith('|') && lines[i].trim().endsWith('|')) {
          tableLines.push(lines[i]);
          i++;
        }
        i--;

        if (tableLines.length >= 2) {
          const parseRow = (rowStr: string) => rowStr.trim().slice(1, -1).split('|').map(c => c.trim());
          const headerCells = parseRow(tableLines[0]);
          const isSep = tableLines[1].replace(/[-|:\s]/g, '').length === 0;
          const bodyRows = (isSep ? tableLines.slice(2) : tableLines.slice(1)).map(parseRow);

          nodes.push(
            <div key={`table-${i}`} className="my-3 w-full max-w-full overflow-x-auto rounded-lg border border-border">
              <table className="w-full min-w-full divide-y divide-border text-left text-ui-sm">
                <thead className="bg-surface-hover font-medium text-foreground">
                  <tr>
                    {headerCells.map((h, cIdx) => (
                      <th key={cIdx} className="whitespace-nowrap px-3 py-2">
                        {renderInline(h)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-border text-foreground">
                  {bodyRows.map((row, rIdx) => (
                    <tr key={rIdx}>
                      {row.map((cell, cIdx) => (
                        <td key={cIdx} className="break-words px-3 py-2 [overflow-wrap:anywhere]">
                          {renderInline(cell)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          );
          continue;
        }
      }

      // Headings — modest semibold, like streamdown in messages
      if (line.startsWith('# ')) {
        nodes.push(
          <h1 key={`h1-${i}`} className="break-words pt-4 pb-1 text-ui-lg font-semibold text-foreground [overflow-wrap:anywhere]">
            {renderInline(line.slice(2))}
          </h1>
        );
      } else if (line.startsWith('## ')) {
        nodes.push(
          <h2 key={`h2-${i}`} className="break-words pt-3 pb-0.5 text-ui-base font-semibold text-foreground [overflow-wrap:anywhere]">
            {renderInline(line.slice(3))}
          </h2>
        );
      } else if (line.startsWith('### ')) {
        nodes.push(
          <h3 key={`h3-${i}`} className="break-words pt-2.5 text-ui-base font-semibold text-foreground [overflow-wrap:anywhere]">
            {renderInline(line.slice(4))}
          </h3>
        );
      } else if (line.startsWith('#### ')) {
        nodes.push(
          <h4 key={`h4-${i}`} className="break-words pt-2 text-ui-sm font-semibold text-foreground [overflow-wrap:anywhere]">
            {renderInline(line.slice(5))}
          </h4>
        );
      } else if (line.startsWith('- ') || line.startsWith('* ') || line.startsWith('• ')) {
        nodes.push(
          <div key={`li-${i}`} className="flex max-w-full min-w-0 items-start gap-2 py-0.5 text-ui-base text-foreground [overflow-wrap:anywhere]">
            <span className="select-none shrink-0 text-foreground-subtlest">•</span>
            <span className="min-w-0 flex-1 break-words leading-[1.6] [overflow-wrap:anywhere]">{renderInline(line.replace(/^[-*•]\s+/, ''))}</span>
          </div>
        );
      } else if (line.match(/^\d+\.\s/)) {
        const num = line.match(/^(\d+)\.\s/)![1];
        nodes.push(
          <div key={`oli-${i}`} className="flex max-w-full min-w-0 items-start gap-2 py-0.5 text-ui-base text-foreground [overflow-wrap:anywhere]">
            <span className="select-none shrink-0 font-mono text-ui-sm text-foreground-subtlest">{num}.</span>
            <span className="min-w-0 flex-1 break-words leading-[1.6] [overflow-wrap:anywhere]">{renderInline(line.replace(/^\d+\.\s+/, ''))}</span>
          </div>
        );
      } else if (line.startsWith('> ')) {
        nodes.push(
          <blockquote key={`quote-${i}`} className="my-2 max-w-full break-words border-l-2 border-border pl-3 text-foreground-subtle [overflow-wrap:anywhere]">
            {renderInline(line.slice(2))}
          </blockquote>
        );
      } else if (line.trim() === '') {
        nodes.push(<div key={`empty-${i}`} className="h-2" />);
      } else {
        nodes.push(
          <p key={`p-${i}`} className="max-w-full min-w-0 break-words text-ui-base/[1.6] text-foreground [overflow-wrap:anywhere]">
            {renderInline(line)}
          </p>
        );
      }
    }

    // Flush any open code block
    if (inCode && codeLines.length > 0) {
      nodes.push(
        <div key="unclosed-code" className="my-3 max-w-full min-w-0 overflow-x-auto rounded-xl border border-border bg-card p-3.5 font-mono text-ui-sm/[1.6] text-foreground">
          <code>{codeLines.join('\n')}</code>
        </div>
      );
    }

    return nodes;
  };

  return (
    <div className="w-full max-w-full min-w-0 break-words space-y-1 text-left [overflow-wrap:anywhere]">
      {renderBlocks()}
    </div>
  );
};
