import React, { useState } from 'react';
import { Copy, Check, FileCode, ExternalLink, Terminal } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

interface MarkdownRendererProps {
  content: string;
}

export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content }) => {
  const { openFileInEditor } = useWorkspace();
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
        const isFilePath = codeText.includes('/') || codeText.endsWith('.ts') || codeText.endsWith('.tsx') || codeText.endsWith('.js') || codeText.endsWith('.json') || codeText.endsWith('.zon') || codeText.endsWith('.zig') || codeText.endsWith('.html') || codeText.endsWith('.css') || codeText.endsWith('.md');
        
        parts.push(
          <span 
            key={key++}
            onClick={() => isFilePath ? openFileInEditor(codeText) : undefined}
            className={`px-1.5 py-0.5 rounded font-mono text-[11.5px] ${
              isFilePath 
                ? 'bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#60a5fa] cursor-pointer hover:underline border border-[#bfdbfe] dark:border-[#1e3a8a]' 
                : 'bg-[#f1f5f9] dark:bg-[#28282a] text-[#0f172a] dark:text-[#e2e8f0]'
            }`}
          >
            {codeText}
          </span>
        );
        current = current.slice(codeMatch[0].length);
        continue;
      }

      // 2. Bold: **text** or __text__
      const boldMatch = current.match(/^(\*\*|__)(.*?)\1/);
      if (boldMatch) {
        parts.push(
          <strong key={key++} className="font-semibold text-[#0f172a] dark:text-white">
            {boldMatch[2]}
          </strong>
        );
        current = current.slice(boldMatch[0].length);
        continue;
      }

      // 3. Italic: *text* or _text_
      const italicMatch = current.match(/^(\*|_)(.*?)\1/);
      if (italicMatch) {
        parts.push(
          <em key={key++} className="italic text-[#334155] dark:text-[#cbd5e1]">
            {italicMatch[2]}
          </em>
        );
        current = current.slice(italicMatch[0].length);
        continue;
      }

      // 4. Regular characters
      const nextSpecial = current.search(/[`*_]/);
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
            <div key={`code-block-${i}`} className="my-3 rounded-xl border border-[#e2e8f0] dark:border-[#2f2f31] bg-[#0f172a] text-[#f8fafc] overflow-hidden shadow-xs">
              <div className="flex items-center justify-between px-3.5 py-1.5 bg-[#1e293b] border-b border-[#334155] text-[11px] text-[#94a3b8] font-mono">
                <div className="flex items-center gap-1.5">
                  <FileCode className="w-3.5 h-3.5 text-[#38bdf8]" />
                  <span>{codeLang || 'text'}</span>
                </div>
                <button
                  type="button"
                  onClick={() => handleCopyCode(fullCode, currentIdx)}
                  className="flex items-center gap-1 px-2 py-0.5 rounded hover:bg-[#334155] text-[#94a3b8] hover:text-white transition-colors cursor-pointer"
                >
                  {copiedBlock === currentIdx ? (
                    <>
                      <Check className="w-3 h-3 text-[#10b981]" />
                      <span className="text-[#10b981]">Copied</span>
                    </>
                  ) : (
                    <>
                      <Copy className="w-3 h-3" />
                      <span>Copy</span>
                    </>
                  )}
                </button>
              </div>
              <pre className="p-3.5 font-mono text-[12px] leading-relaxed overflow-x-auto selection:bg-[#38bdf8]/30">
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

      // Headings
      if (line.startsWith('# ')) {
        nodes.push(
          <h1 key={`h1-${i}`} className="text-lg font-bold text-[#0f172a] dark:text-[#f8fafc] pt-3 pb-1 border-b border-[#e2e8f0] dark:border-[#333336]">
            {renderInline(line.slice(2))}
          </h1>
        );
      } else if (line.startsWith('## ')) {
        nodes.push(
          <h2 key={`h2-${i}`} className="text-base font-bold text-[#0f172a] dark:text-[#f8fafc] pt-2.5 pb-0.5">
            {renderInline(line.slice(3))}
          </h2>
        );
      } else if (line.startsWith('### ')) {
        nodes.push(
          <h3 key={`h3-${i}`} className="text-sm font-semibold text-[#0f172a] dark:text-[#f1f5f9] pt-2">
            {renderInline(line.slice(4))}
          </h3>
        );
      } else if (line.startsWith('#### ')) {
        nodes.push(
          <h4 key={`h4-${i}`} className="text-xs font-semibold text-[#0f172a] dark:text-[#f1f5f9] pt-1.5">
            {renderInline(line.slice(5))}
          </h4>
        );
      } else if (line.startsWith('- ') || line.startsWith('* ') || line.startsWith('• ')) {
        nodes.push(
          <div key={`li-${i}`} className="flex items-start gap-2 text-[13px] text-[#334155] dark:text-[#e2e8f0] pl-2 py-0.5">
            <span className="text-[#3b82f6] select-none font-bold">•</span>
            <span className="flex-1 leading-relaxed">{renderInline(line.replace(/^[-*•]\s+/, ''))}</span>
          </div>
        );
      } else if (line.match(/^\d+\.\s/)) {
        const num = line.match(/^(\d+)\.\s/)![1];
        nodes.push(
          <div key={`oli-${i}`} className="flex items-start gap-2 text-[13px] text-[#334155] dark:text-[#e2e8f0] pl-2 py-0.5">
            <span className="text-[#2563eb] dark:text-[#60a5fa] font-mono text-[11px] font-semibold select-none">{num}.</span>
            <span className="flex-1 leading-relaxed">{renderInline(line.replace(/^\d+\.\s+/, ''))}</span>
          </div>
        );
      } else if (line.startsWith('> ')) {
        nodes.push(
          <blockquote key={`quote-${i}`} className="p-2.5 my-2 border-l-4 border-[#2563eb] bg-[#eff6ff] dark:bg-[#1e293b]/70 text-xs text-[#1e40af] dark:text-[#bfdbfe] rounded-r-xl">
            {renderInline(line.slice(2))}
          </blockquote>
        );
      } else if (line.trim() === '') {
        nodes.push(<div key={`empty-${i}`} className="h-1.5" />);
      } else {
        nodes.push(
          <p key={`p-${i}`} className="text-[13px] text-[#334155] dark:text-[#e2e8f0] leading-relaxed">
            {renderInline(line)}
          </p>
        );
      }
    }

    // Flush any open code block
    if (inCode && codeLines.length > 0) {
      nodes.push(
        <div key="unclosed-code" className="my-2 rounded-xl border border-[#e2e8f0] dark:border-[#333336] bg-[#090d16] text-[#f8fafc] p-3 font-mono text-xs overflow-x-auto">
          <code>{codeLines.join('\n')}</code>
        </div>
      );
    }

    return nodes;
  };

  return (
    <div className="space-y-1 font-sans text-left selection:bg-[#bfdbfe]/60 dark:selection:bg-[#264f78]">
      {renderBlocks()}
    </div>
  );
};
