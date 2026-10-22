import React, { useState, useEffect } from 'react';
import { Copy, Check, FileCode, Image as ImageIcon } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { parseFilePath } from '../../lib/utils';
import { openInBrowser, isLocalhostUrl } from '../../lib/browser';
import { ApiBridge } from '../../services/apiBridge';

interface MarkdownRendererProps {
  content: string;
  className?: string;
  muted?: boolean;
}

/**
 * Chat code block: themed code surface, language label, copy
 * and wrap toggles in the header, and the runtime code font size.
 */
const CodeBlockView: React.FC<{
  code: string;
  lang: string;
  blockIdx: number;
  copied: boolean;
  onCopy: (code: string, blockIdx: number) => void;
}> = ({ code, lang, blockIdx, copied, onCopy }) => {
  const [wrap, setWrap] = useState(false);
  const isDiff = lang.toLowerCase() === 'diff';

  return (
    <div
      className={`chat-markdown-codeblock my-3 w-full max-w-full min-w-0 overflow-hidden rounded-lg border border-border-default bg-code text-code-foreground`}
      data-wrap={wrap ? 'true' : 'false'}
    >
      <div className="chat-markdown-codeblock-header flex items-center justify-between border-b border-border-subtle px-3.5 py-1.5 font-mono text-2xs uppercase tracking-wide">
        <span className="truncate">{lang || 'text'}</span>
        <div className="flex shrink-0 items-center gap-1">
          <button
            type="button"
            onClick={() => setWrap((w) => !w)}
            className="cursor-pointer rounded px-2 py-0.5 transition-colors hover:bg-surface-hover"
            title={wrap ? 'Disable soft wrap' : 'Enable soft wrap'}
          >
            {wrap ? 'No wrap' : 'Wrap'}
          </button>
          <button
            type="button"
            onClick={() => onCopy(code, blockIdx)}
            className="flex cursor-pointer items-center gap-1 rounded px-2 py-0.5 transition-colors hover:bg-surface-hover"
          >
            {copied ? (
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
      </div>
      <pre
        className={`max-w-full min-w-0 max-h-[380px] overflow-y-auto p-3.5 font-mono leading-[1.6] ${wrap ? 'whitespace-pre-wrap overflow-x-hidden [overflow-wrap:anywhere]' : 'overflow-x-auto'}`}
        style={{ fontSize: 'var(--font-size-code, 0.8125rem)' }}
      >
        {isDiff ? (
          <code>
            {code.split('\n').map((cl, cli) => {
              let lineClass = 'text-foreground-secondary';
              if (cl.startsWith('+') && !cl.startsWith('+++')) {
                lineClass = 'text-diff-addition-foreground bg-diff-addition/10 font-medium';
              } else if (cl.startsWith('-') && !cl.startsWith('---')) {
                lineClass = 'text-diff-deletion-foreground bg-diff-deletion/10 font-medium';
              } else if (cl.startsWith('@@')) {
                lineClass = 'text-info-foreground bg-info/10 font-semibold';
              } else if (cl.startsWith('---') || cl.startsWith('+++') || cl.startsWith('diff ')) {
                lineClass = 'text-foreground-subtle font-semibold';
              }
              return (
                <span key={cli} className={`block px-2 -mx-2 rounded-[2px] ${lineClass}`}>
                  {cl || ' '}
                </span>
              );
            })}
          </code>
        ) : (
          <code>{code}</code>
        )}
      </pre>
    </div>
  );
};

const MarkdownImageThumbnail: React.FC<{
  src: string;
  alt: string;
  onClick: () => void;
}> = ({ src, alt, onClick }) => {
  const [dataUrl, setDataUrl] = useState<string | null>(
    src.startsWith('data:') || src.startsWith('http://') || src.startsWith('https://') ? src : null
  );

  useEffect(() => {
    if (src.startsWith('data:') || src.startsWith('http://') || src.startsWith('https://')) {
      setDataUrl(src);
      return;
    }
    const cleanPath = src.replace(/^file:\/\//, '');
    let active = true;
    ApiBridge.readFileBase64(cleanPath)
      .then(b64 => {
        if (!active || !b64) return;
        const ext = cleanPath.split('.').pop()?.toLowerCase() || 'png';
        const mime = ext === 'svg' ? 'image/svg+xml' : ext === 'jpg' || ext === 'jpeg' ? 'image/jpeg' : `image/${ext}`;
        setDataUrl(`data:${mime};base64,${b64}`);
      })
      .catch(() => {});
    return () => { active = false; };
  }, [src]);

  return (
    <span
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onClick();
      }}
      className="my-1.5 inline-flex flex-col gap-1 rounded-lg border border-border/60 bg-surface/80 p-1.5 hover:border-info/70 transition-all cursor-pointer group max-w-sm select-none"
      title={`Click to view ${alt} in right sidebar`}
    >
      <span className="relative overflow-hidden rounded bg-black/10 dark:bg-black/30 flex items-center justify-center max-h-48 min-h-[48px] min-w-[80px]">
        {dataUrl ? (
          <img
            src={dataUrl}
            alt={alt}
            className="max-h-48 max-w-full object-contain rounded transition-transform duration-200 group-hover:scale-[1.02]"
          />
        ) : (
          <div className="flex items-center gap-1.5 p-3 text-muted-foreground text-ui-xs font-mono">
            <ImageIcon className="size-4 text-info" />
            <span>{alt || 'Image'}</span>
          </div>
        )}
      </span>
      <span className="flex items-center justify-between gap-1 px-1 text-ui-xs font-mono text-muted-foreground group-hover:text-info">
        <span className="truncate">{alt || 'Image'}</span>
        <span className="text-[10px] shrink-0 font-sans">View in sidebar ↗</span>
      </span>
    </span>
  );
};

/**
 * Chat markdown rendering with the reference streamdown look: full-contrast
 * 14px body, subtle neutral inline-code chips (no borders, no link blues),
 * modest semibold headings, neutral code blocks.
 * Supports `muted` mode for thinking/scratchpad blocks with soft, subdued grey tones.
 */
const MarkdownRendererImpl: React.FC<MarkdownRendererProps> = ({ content, className, muted = false }) => {
  const { openSideFile, activeWorkspacePath } = useWorkspace();
  const [copiedBlock, setCopiedBlock] = useState<number | null>(null);

  const handleCopyCode = (code: string, blockIdx: number) => {
    navigator.clipboard.writeText(code);
    setCopiedBlock(blockIdx);
    setTimeout(() => setCopiedBlock(null), 1500);
  };

  // Parse inline markdown tokens: `code`, **bold**, *italic*, [link](url), file paths
  const renderInline = (text: string) => {
    const parts: React.ReactNode[] = [];
    let current = text;
    let key = 0;

    while (current.length > 0) {
      // 1. Inline code: `something`
      const codeMatch = current.match(/^`([^`]+)`/);
      if (codeMatch) {
        const codeText = codeMatch[1];
        const parsed = parseFilePath(codeText, activeWorkspacePath);

        if (parsed.isLikelyFilePath) {
          parts.push(
            <code
              key={key++}
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                openSideFile(codeText);
              }}
              title={`Open ${parsed.displayName} in right sidebar`}
              className="inline-flex items-center gap-1 rounded-md bg-surface px-1.5 py-0.5 font-mono text-[0.9em] text-foreground hover:bg-surface-hover hover:text-info cursor-pointer border border-border/50 transition-colors break-all [overflow-wrap:anywhere]"
            >
              <FileCode className="w-3 h-3 text-foreground-subtle shrink-0" />
              <span>{codeText}</span>
            </code>
          );
        } else {
          parts.push(
            <code
              key={key++}
              className="rounded-md bg-surface px-1 py-0.5 font-mono text-[0.9em] text-foreground break-all [overflow-wrap:anywhere]"
            >
              {codeText}
            </code>
          );
        }
        current = current.slice(codeMatch[0].length);
        continue;
      }

      // 1.5 Markdown image: ![alt](url)
      const imageMatch = current.match(/^!\[([^\]]*)\]\(([^)\s]+)\)/);
      if (imageMatch) {
        const altText = imageMatch[1] || 'Image';
        const rawUrl = imageMatch[2];
        const parsed = parseFilePath(rawUrl, activeWorkspacePath);
        const resolvedPath = parsed.cleanPath || rawUrl.replace(/^file:\/\//, '');

        parts.push(
          <MarkdownImageThumbnail
            key={key++}
            src={rawUrl}
            alt={altText}
            onClick={() => void openSideFile(resolvedPath)}
          />
        );
        current = current.slice(imageMatch[0].length);
        continue;
      }

      // 2. Links: [text](url)
      const linkMatch = current.match(/^\[([^\]]+)\]\(([^)\s]+)\)/);
      if (linkMatch) {
        const linkText = linkMatch[1];
        const url = linkMatch[2];
        const parsed = parseFilePath(url, activeWorkspacePath);

        if (url.startsWith('file://') || parsed.isLikelyFilePath) {
          parts.push(
            <button
              key={key++}
              type="button"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                openSideFile(url);
              }}
              title={`Open ${parsed.displayName} in right sidebar`}
              className="inline-flex items-center gap-1 text-info underline decoration-info/40 underline-offset-2 hover:decoration-info cursor-pointer font-medium break-words [overflow-wrap:anywhere]"
            >
              <FileCode className="w-3.5 h-3.5 shrink-0 text-info/70" />
              <span>{renderInline(linkText)}</span>
            </button>
          );
        } else {
          parts.push(
            <a
              key={key++}
              href={url}
              onClick={(e) => {
                if (isLocalhostUrl(url)) {
                  e.preventDefault();
                  openInBrowser(url);
                }
              }}
              target="_blank"
              rel="noreferrer"
              className="text-info underline decoration-info/40 underline-offset-2 hover:decoration-info break-words [overflow-wrap:anywhere]"
            >
              {renderInline(linkText)}
            </a>
          );
        }
        current = current.slice(linkMatch[0].length);
        continue;
      }

      // 3. Raw file:// URI in plain text
      const fileUriMatch = current.match(/^file:\/\/[^\s<>"')\]]+/);
      if (fileUriMatch) {
        const fileUri = fileUriMatch[0];
        const parsed = parseFilePath(fileUri, activeWorkspacePath);
        parts.push(
          <button
            key={key++}
            type="button"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              openSideFile(fileUri);
            }}
            title={`Open ${parsed.displayName} in right sidebar`}
            className="inline-flex items-center gap-1 text-info underline decoration-info/40 underline-offset-2 hover:decoration-info cursor-pointer font-mono text-[0.9em] break-all [overflow-wrap:anywhere]"
          >
            <FileCode className="w-3.5 h-3.5 shrink-0 text-info/70" />
            <span>{parsed.displayName}</span>
          </button>
        );
        current = current.slice(fileUri.length);
        continue;
      }

      // 4. Raw file paths in plain text (e.g. frontend/src/lib/utils.ts:45)
      const rawFileMatch = current.match(/^(?:(?:\.\/|\.\.\/|[a-zA-Z0-9_.-]+\/)[a-zA-Z0-9_./-]+\.(?:ts|tsx|js|jsx|json|go|py|rs|css|scss|html|md|toml|yaml|yml|sh|c|h|cpp|hpp|sql|txt|zon|zig|swift|kt|java))(?::\d+)?(?:#L\d+)?/);
      if (rawFileMatch) {
        const rawPath = rawFileMatch[0];
        const parsed = parseFilePath(rawPath, activeWorkspacePath);
        parts.push(
          <button
            key={key++}
            type="button"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              openSideFile(rawPath);
            }}
            title={`Open ${parsed.displayName} in right sidebar`}
            className="inline-flex items-center gap-1 text-info underline decoration-info/40 underline-offset-2 hover:decoration-info cursor-pointer font-mono text-[0.9em] break-all [overflow-wrap:anywhere]"
          >
            <FileCode className="w-3.5 h-3.5 shrink-0 text-info/70" />
            <span>{rawPath}</span>
          </button>
        );
        current = current.slice(rawPath.length);
        continue;
      }

      // 5. Bold: **text** or __text__
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

      // 6. Italic: *text* or _text_
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

      // 7. Regular characters: find the next delimiter
      const nextSpecial = current.search(/[`*_\[!]|file:\/\/|(?:\.\/|\.\.\/|[a-zA-Z0-9_.-]+\/)[a-zA-Z0-9_./-]+\.(?:ts|tsx|js|jsx|json|go|py|rs|css|scss|html|md|toml|yaml|yml|sh|c|h|cpp|hpp|sql|txt|zon|zig|swift|kt|java)/);
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
            <CodeBlockView
              key={`code-block-${i}`}
              code={fullCode}
              lang={codeLang}
              blockIdx={currentIdx}
              copied={copiedBlock === currentIdx}
              onCopy={handleCopyCode}
            />
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

      // Headings — clear hierarchy matching ChatGPT web standards
      if (line.startsWith('# ')) {
        nodes.push(
          <h1 key={`h1-${i}`} className={`break-words pt-3.5 pb-1 font-semibold leading-tight [overflow-wrap:anywhere] ${muted ? 'text-ui-sm text-foreground-subtle' : 'text-[22px] leading-[1.3] text-foreground'}`}>
            {renderInline(line.slice(2))}
          </h1>
        );
      } else if (line.startsWith('## ')) {
        nodes.push(
          <h2 key={`h2-${i}`} className={`break-words pt-3 pb-0.5 font-semibold leading-snug [overflow-wrap:anywhere] ${muted ? 'text-ui-sm text-foreground-subtle' : 'text-[18px] leading-[1.35] text-foreground'}`}>
            {renderInline(line.slice(3))}
          </h2>
        );
      } else if (line.startsWith('### ')) {
        nodes.push(
          <h3 key={`h3-${i}`} className={`break-words pt-2 pb-0.5 font-semibold leading-snug [overflow-wrap:anywhere] ${muted ? 'text-ui-xs text-foreground-subtle' : 'text-[16px] leading-[1.4] text-foreground'}`}>
            {renderInline(line.slice(4))}
          </h3>
        );
      } else if (line.startsWith('#### ')) {
        nodes.push(
          <h4 key={`h4-${i}`} className={`break-words pt-1.5 font-semibold leading-normal [overflow-wrap:anywhere] ${muted ? 'text-ui-xs text-foreground-subtle' : 'text-[14.5px] text-foreground'}`}>
            {renderInline(line.slice(5))}
          </h4>
        );
      } else if (line.startsWith('- ') || line.startsWith('* ') || line.startsWith('• ')) {
        nodes.push(
          <div key={`li-${i}`} className={`flex max-w-full min-w-0 items-start gap-2 py-0.5 [overflow-wrap:anywhere] ${muted ? 'text-ui-sm text-foreground-subtle' : 'text-[15px] text-foreground'}`}>
            <span className="select-none shrink-0 text-foreground-subtlest">•</span>
            <span className="min-w-0 flex-1 break-words leading-[1.65] [overflow-wrap:anywhere]">{renderInline(line.replace(/^[-*•]\s+/, ''))}</span>
          </div>
        );
      } else if (line.match(/^\d+\.\s/)) {
        const num = line.match(/^(\d+)\.\s/)![1];
        nodes.push(
          <div key={`oli-${i}`} className={`flex max-w-full min-w-0 items-start gap-2 py-0.5 [overflow-wrap:anywhere] ${muted ? 'text-ui-sm text-foreground-subtle' : 'text-[15px] text-foreground'}`}>
            <span className="select-none shrink-0 font-mono text-[13px] text-foreground-subtlest">{num}.</span>
            <span className="min-w-0 flex-1 break-words leading-[1.65] [overflow-wrap:anywhere]">{renderInline(line.replace(/^\d+\.\s+/, ''))}</span>
          </div>
        );
      } else if (line.startsWith('> ')) {
        nodes.push(
          <blockquote key={`quote-${i}`} className={`my-2 max-w-full break-words border-l-2 pl-3 [overflow-wrap:anywhere] ${muted ? 'border-border/50 text-foreground-subtlest text-ui-sm' : 'border-border text-foreground-subtle'}`}>
            {renderInline(line.slice(2))}
          </blockquote>
        );
      } else if (line.trim() === '') {
        nodes.push(<div key={`empty-${i}`} className="h-2" />);
      } else {
        nodes.push(
          <p key={`p-${i}`} className={`max-w-full min-w-0 break-words [overflow-wrap:anywhere] ${muted ? 'text-ui-sm/[1.6] text-foreground-subtle' : 'text-[15px]/[1.65] text-foreground'}`}>
            {renderInline(line)}
          </p>
        );
      }
    }

    // Flush any open code block
    if (inCode && codeLines.length > 0) {
      nodes.push(
        <div key="unclosed-code" className={`my-3 max-w-full min-w-0 overflow-x-auto rounded-lg border border-border-default bg-code p-3.5 font-mono leading-[1.6] text-code-foreground ${muted ? 'opacity-80' : ''}`} style={{ fontSize: 'var(--font-size-code, 0.8125rem)' }}>
          <code>{codeLines.join('\n')}</code>
        </div>
      );
    }

    return nodes;
  };

  return (
    <div className={`w-full max-w-full min-w-0 break-words space-y-1 text-left [overflow-wrap:anywhere] ${className || ''} ${muted ? 'text-foreground-subtle' : ''}`}>
      {renderBlocks()}
    </div>
  );
};

/**
 * Memoized by content: streaming updates re-render the whole conversation on
 * every text delta, and each finished message's markdown must not re-parse.
 */
export const MarkdownRenderer = React.memo(MarkdownRendererImpl);
