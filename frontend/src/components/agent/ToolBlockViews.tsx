import React, { useMemo, useState } from 'react';
import { 
  Pencil, 
  SquareTerminal, 
  Search, 
  FileText, 
  ChevronDown, 
  Loader2, 
  AlertTriangle, 
  Wrench,
  Code2
} from 'lucide-react';
import { ToolExecution, FileDiff } from '../../types';
import { parseTwoFilesDiff } from '../diff/DiffViewer';
import { cn } from '../../lib/utils';
import { highlightCodeLine, highlightTerminalLine, highlightTerminalCommand } from '../../lib/diff-highlight';

export function cleanArgPath(val: string): string {
  if (!val) return '';
  return val.replace(/^[\s{"'`\\]+/, '').replace(/[\s}"'`\\]+$/, '').trim();
}

export interface ParsedToolArgs {
  path: string;
  cmd: string;
  content: string;
  oldString?: string;
  newString?: string;
}

export function parseToolArgs(raw: string, toolName: string): ParsedToolArgs {
  let path = '';
  let cmd = '';
  let content = '';
  let oldString: string | undefined;
  let newString: string | undefined;

  const str = (raw || '').trim();
  if (str.startsWith('{')) {
    try {
      const p = JSON.parse(str);
      path = p.path || p.filepath || p.filePath || p.file_path || p.file || p.filename || p.target_file || p.targetFile || p.TargetFile || '';
      cmd = p.command || p.cmd || p.script || p.CommandLine || '';
      content = p.content || p.text || p.code || p.body || p.file_content || p.CodeContent || '';
      oldString = p.old_string || p.old || p.target || '';
      newString = p.new_string || p.new || p.replacement || '';
    } catch {
      const pm = str.match(/"(?:path|filepath|filePath|file_path|file|filename|target_file|targetFile|TargetFile)":\s*"([^"]+)"/i);
      if (pm) path = pm[1];
      const cm = str.match(/"(?:command|cmd|script|CommandLine)":\s*"([^"]+)"/i);
      if (cm) cmd = cm[1];
      const ctm = str.match(/"(?:content|text|code|body|file_content|CodeContent)":\s*"([\s\S]+?)"(?:\s*,\s*"|\s*})/i);
      if (ctm) content = ctm[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
      const oldM = str.match(/"(?:old_string|old|target)":\s*"([\s\S]+?)"(?:\s*,\s*"|\s*})/i);
      if (oldM) oldString = oldM[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
      const newM = str.match(/"(?:new_string|new|replacement)":\s*"([\s\S]+?)"(?:\s*,\s*"|\s*})/i);
      if (newM) newString = newM[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
    }
  }

  path = cleanArgPath(path);
  const nameLower = (toolName || '').toLowerCase();
  if (!path && (nameLower.includes('write') || nameLower.includes('create') || nameLower.includes('edit') || nameLower.includes('read') || nameLower.includes('view'))) {
    let candidate = cleanArgPath(str);
    candidate = candidate.replace(/^(?:read_file|read|cat|write|create|edit)\s+/i, '');
    if (!candidate.includes('\n') && candidate.length < 250) {
      path = cleanArgPath(candidate);
    }
  }

  if (!cmd && !path) {
    cmd = str;
  }

  return { path, cmd, content, oldString, newString };
}

export function parseReadOutput(output: string): { filePath?: string; totalLines?: number; content: string } {
  if (!output) return { content: '' };
  const headerMatch = output.match(/^Contents of ([^\n(]+)(?:\s*\((\d+)\s+total lines\))?:\n([\s\S]*)$/);
  if (headerMatch) {
    return {
      filePath: headerMatch[1].trim(),
      totalLines: headerMatch[2] ? parseInt(headerMatch[2], 10) : undefined,
      content: headerMatch[3]
    };
  }
  return { content: output };
}

export interface InlineDiffLine {
  type: 'normal' | 'add' | 'del';
  lineNum: number;
  text: string;
}

export function parseInlineDiffLines(
  diffText?: string,
  oldStr?: string,
  newStr?: string
): InlineDiffLine[] {
  const result: InlineDiffLine[] = [];

  if (diffText && diffText.includes('@@')) {
    const lines = diffText.split('\n');
    let displayLine = 1;
    let inHunk = false;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      if (line.startsWith('@@')) {
        inHunk = true;
        const match = line.match(/@@ -(\d+)/);
        if (match) {
          displayLine = parseInt(match[1], 10);
        }
        continue;
      }
      if (!inHunk) continue;
      if (
        line.startsWith('diff --git') ||
        line.startsWith('---') ||
        line.startsWith('+++') ||
        line.startsWith('index ') ||
        line.startsWith('\\')
      ) {
        continue;
      }

      if (line.startsWith('+')) {
        result.push({
          type: 'add',
          lineNum: displayLine++,
          text: line.slice(1)
        });
      } else if (line.startsWith('-')) {
        result.push({
          type: 'del',
          lineNum: displayLine++,
          text: line.slice(1)
        });
      } else if (line.startsWith(' ')) {
        result.push({
          type: 'normal',
          lineNum: displayLine++,
          text: line.slice(1)
        });
      } else if (line.trim().length > 0) {
        result.push({
          type: 'normal',
          lineNum: displayLine++,
          text: line
        });
      }
    }
  }

  // Fallback to oldStr / newStr if diff text produced no lines
  if (result.length === 0 && (oldStr || newStr)) {
    let lineIdx = 1;
    if (oldStr) {
      for (const l of oldStr.split('\n')) {
        result.push({ type: 'del', lineNum: lineIdx++, text: l });
      }
    }
    if (newStr) {
      for (const l of newStr.split('\n')) {
        result.push({ type: 'add', lineNum: lineIdx++, text: l });
      }
    }
  }

  return result;
}

// ---------------------------------------------------------------------------
// 1. TOOL ERROR BLOCK
// ---------------------------------------------------------------------------
export function ToolErrorBlock({ errorText }: { errorText: string }) {
  let cleanError = errorText || 'Tool execution failed';
  const match = cleanError.match(/<tool_use_error>([\s\S]*?)<\/tool_use_error>/i);
  if (match) {
    cleanError = match[1].trim();
  }

  return (
    <div className="mt-1.5 ml-6 w-[calc(100%-1.5rem)] max-w-full rounded-xl border border-destructive/30 bg-destructive/10 p-3 font-mono text-ui-xs text-destructive leading-relaxed max-h-60 overflow-y-auto overflow-x-auto whitespace-pre-wrap break-words select-text scrollbar-thin">
      {cleanError}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 2. TOOL INLINE DIFF BLOCK (For Edit tool changes: +1 -1)
// ---------------------------------------------------------------------------
export function ToolInlineDiffBlock({
  diff,
  rawDiffText,
  oldStr,
  newStr,
  ext
}: {
  diff?: FileDiff;
  rawDiffText?: string;
  oldStr?: string;
  newStr?: string;
  ext?: string;
}) {
  const lines: InlineDiffLine[] = useMemo(() => {
    // 1. If we have FileDiff with original and modified content
    if (diff?.originalContent !== undefined && diff?.modifiedContent !== undefined) {
      const parsed = parseTwoFilesDiff(diff.originalContent, diff.modifiedContent);
      let lineCounter = 1;
      const res: InlineDiffLine[] = [];
      for (const pl of parsed.lines) {
        if (pl.type === 'file-header' || pl.type === 'header') continue;
        res.push({
          type: pl.type === 'add' ? 'add' : pl.type === 'del' ? 'del' : 'normal',
          lineNum: lineCounter++,
          text: pl.text
        });
      }
      if (res.length > 0) return res;
    }

    // 2. If we have unified diff text or old/new strings
    const text = rawDiffText || diff?.originalContent || '';
    return parseInlineDiffLines(text, oldStr, newStr);
  }, [diff, rawDiffText, oldStr, newStr]);

  if (lines.length === 0) {
    return (
      <div className="mt-1.5 ml-6 w-[calc(100%-1.5rem)] rounded-xl border border-border/50 bg-card p-3 font-mono text-ui-xs text-foreground-subtlest">
        No visual line changes to display
      </div>
    );
  }

  return (
    <div className="mt-1.5 ml-6 w-[calc(100%-1.5rem)] max-w-full rounded-xl border border-border/50 bg-card/90 overflow-hidden text-ui-xs shadow-xs font-mono select-text">
      <div className="max-h-72 overflow-y-auto overflow-x-auto p-1.5 scrollbar-thin">
        {lines.map((l, idx) => {
          const isAdd = l.type === 'add';
          const isDel = l.type === 'del';

          return (
            <div
              key={idx}
              className={cn(
                "flex items-start px-2 py-0.5 leading-relaxed transition-colors",
                isAdd && "bg-success/15 text-success border-l-[3px] border-l-success",
                isDel && "bg-destructive/15 text-destructive border-l-[3px] border-l-destructive",
                !isAdd && !isDel && "text-foreground-secondary border-l-[3px] border-l-transparent"
              )}
            >
              <span
                className={cn(
                  "w-8 text-right pr-3 select-none shrink-0 font-mono text-ui-xs",
                  isAdd && "text-success font-semibold",
                  isDel && "text-destructive font-semibold",
                  !isAdd && !isDel && "text-foreground-subtlest"
                )}
              >
                {l.lineNum}
              </span>
              <span className="flex-1 whitespace-pre overflow-x-auto">
                {highlightCodeLine(l.text, ext) || ' '}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// 3. TOOL WRITE BLOCK (Solid green accent bar on left, green line numbers)
// ---------------------------------------------------------------------------
export function ToolWriteBlock({ content, ext }: { content: string; ext?: string }) {
  const lines = useMemo(() => (content || '').split('\n'), [content]);

  return (
    <div className="mt-1.5 ml-6 w-[calc(100%-1.5rem)] max-w-full rounded-xl border border-success/30 bg-success/5 border-l-[3px] border-l-success overflow-hidden text-ui-xs shadow-xs font-mono select-text">
      <div className="max-h-72 overflow-y-auto overflow-x-auto p-2 scrollbar-thin">
        {lines.map((line, idx) => (
          <div key={idx} className="flex items-start px-2 py-0.5">
            <span className="w-8 text-right pr-3 text-success select-none shrink-0 font-mono text-ui-xs font-semibold">
              {idx + 1}
            </span>
            <span className="flex-1 whitespace-pre overflow-x-auto text-foreground-secondary">
              {highlightCodeLine(line, ext) || ' '}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// 4. TOOL READ BLOCK
// ---------------------------------------------------------------------------
export function ToolReadBlock({ content, ext }: { content: string; ext?: string }) {
  const [showAll, setShowAll] = useState(false);
  const lines = useMemo(() => (content || '').split('\n'), [content]);
  const isLong = lines.length > 40;
  const visibleLines = isLong && !showAll ? lines.slice(0, 30) : lines;

  return (
    <div className="mt-1.5 ml-6 w-[calc(100%-1.5rem)] max-w-full rounded-xl border border-border/50 bg-card/80 overflow-hidden text-ui-xs shadow-xs font-mono select-text">
      <div className="max-h-96 overflow-y-auto overflow-x-auto p-2 scrollbar-thin">
        {visibleLines.map((line, idx) => (
          <div key={idx} className="flex items-start px-2 py-0.5">
            <span className="w-8 text-right pr-3 text-foreground-subtlest select-none shrink-0 font-mono text-ui-xs">
              {idx + 1}
            </span>
            <span className="flex-1 whitespace-pre overflow-x-auto text-foreground-secondary">
              {highlightCodeLine(line, ext) || ' '}
            </span>
          </div>
        ))}
        {isLong && (
          <div className="pt-2 px-2 pb-1">
            <button
              type="button"
              onClick={() => setShowAll(v => !v)}
              className="flex w-full items-center justify-center gap-1.5 rounded-md border border-border/60 bg-surface/80 py-1 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface hover:text-foreground cursor-pointer select-none"
            >
              <span>
                {showAll
                  ? 'Collapse lines'
                  : `Truncated (+${lines.length - 30} lines · click to expand)`}
              </span>
              <ChevronDown className={cn("w-3 h-3 transition-transform", showAll && "rotate-180")} />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// 5. TOOL TERMINAL BLOCK ($ ls -la ... followed by stdout)
// ---------------------------------------------------------------------------
export function ToolTerminalBlock({
  command,
  output
}: {
  command: string;
  output: string;
}) {
  const [showAll, setShowAll] = useState(false);
  const lines = useMemo(() => (output || '').split('\n'), [output]);
  const isLong = lines.length > 35;
  const visibleLines = isLong && !showAll ? lines.slice(0, 25) : lines;

  return (
    <div className="mt-1.5 ml-6 w-[calc(100%-1.5rem)] max-w-full rounded-xl border border-border/50 bg-card/80 p-3 font-mono text-ui-xs leading-relaxed max-h-96 overflow-y-auto overflow-x-auto scrollbar-thin shadow-xs select-text">
      {command && (
        <div className="text-foreground font-semibold pb-1.5 flex items-center gap-1.5">
          {highlightTerminalCommand(command)}
        </div>
      )}
      <div className="font-mono text-ui-xs leading-relaxed m-0 p-0 select-text space-y-0.5">
        {visibleLines.length === 0 || (visibleLines.length === 1 && !visibleLines[0]) ? (
          <span className="text-foreground-subtlest italic">(no output)</span>
        ) : (
          visibleLines.map((line, idx) => (
            <div key={idx} className="whitespace-pre-wrap break-words min-h-[1.125rem]">
              {highlightTerminalLine(line) || ' '}
            </div>
          ))
        )}
      </div>
      {isLong && (
        <button
          type="button"
          onClick={() => setShowAll(v => !v)}
          className="mt-2.5 flex w-full items-center justify-center gap-1.5 rounded-md border border-border/60 bg-surface/80 py-1 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface hover:text-foreground cursor-pointer select-none"
        >
          <span>
            {showAll
              ? 'Collapse output'
              : `Output truncated (+${lines.length - 25} lines · click to expand)`}
          </span>
          <ChevronDown className={cn("w-3 h-3 transition-transform", showAll && "rotate-180")} />
        </button>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 6. TOOL EXECUTION CARD (Header row + Expandable block matching Screenshot 2)
// ---------------------------------------------------------------------------
export function ToolExecutionCard({
  tool,
  isOpen,
  onToggle,
  workspacePath
}: {
  tool: ToolExecution;
  isOpen: boolean;
  onToggle: () => void;
  workspacePath?: string;
}) {
  const nameLower = (tool.toolName || '').toLowerCase();
  const isFailed = tool.status === 'failed' || (tool.output || '').includes('<tool_use_error>');
  const isRunning = tool.status === 'running';

  const isBash = nameLower.includes('bash') || nameLower.includes('shell') || nameLower.includes('exec') || nameLower.includes('command');
  const isEdit = nameLower.includes('edit');
  const isWrite = nameLower.includes('write') || nameLower.includes('create');
  const isRead = nameLower.includes('read') || nameLower.includes('view') || nameLower.includes('cat');
  const isPython = nameLower.includes('python');

  const parsedArgs = useMemo(() => parseToolArgs(tool.command || '', tool.toolName || ''), [tool.command, tool.toolName]);

  // Clean target file
  let fullPath = parsedArgs.path || tool.diff?.filePath || '';
  if (workspacePath && fullPath.startsWith(workspacePath)) {
    fullPath = fullPath.slice(workspacePath.length).replace(/^\/+/, '');
  }
  fullPath = cleanArgPath(fullPath);
  const targetFile = fullPath ? (fullPath.split('/').filter(Boolean).pop() || fullPath) : '';

  // Icon & Kind Label matching Screenshot 2
  let kind = 'Ran';
  let Icon = SquareTerminal;

  if (isEdit) {
    kind = 'Edit';
    Icon = Pencil;
  } else if (isWrite) {
    kind = 'Write';
    Icon = Pencil;
  } else if (isRead) {
    kind = 'Read';
    Icon = Search;
  } else if (isPython) {
    kind = 'Python';
    Icon = Code2;
  } else if (isBash) {
    kind = 'Terminal';
    Icon = SquareTerminal;
  } else {
    kind = (tool.toolName || 'Tool').replace(/_/g, ' ').replace(/^\w/, c => c.toUpperCase());
    Icon = Wrench;
  }

  // Additions and Deletions calculation
  const diffStats = useMemo(() => {
    if (tool.diff && (tool.diff.additions > 0 || tool.diff.deletions > 0)) {
      return { additions: tool.diff.additions, deletions: tool.diff.deletions };
    }
    if (isWrite) {
      const code = parsedArgs.content || tool.diff?.modifiedContent || '';
      if (code) {
        const linesCount = code.split('\n').length;
        return { additions: linesCount, deletions: 0 };
      }
    }
    if (isEdit) {
      const diffText = tool.output || '';
      if (diffText.includes('@@')) {
        let add = 0;
        let del = 0;
        for (const l of diffText.split('\n')) {
          if (l.startsWith('+') && !l.startsWith('+++')) add++;
          else if (l.startsWith('-') && !l.startsWith('---')) del++;
        }
        if (add > 0 || del > 0) return { additions: add, deletions: del };
      }
    }
    return null;
  }, [tool.diff, isWrite, isEdit, parsedArgs.content, tool.output]);

  // Read lines count
  const readLinesCount = useMemo(() => {
    if (!isRead || !tool.output) return null;
    const parsed = parseReadOutput(tool.output);
    if (parsed.totalLines) return parsed.totalLines;
    if (parsed.content) return parsed.content.split('\n').length;
    return null;
  }, [isRead, tool.output]);

  const hasOutput = Boolean(tool.output || tool.diff || parsedArgs.content || parsedArgs.cmd);

  return (
    <div className="w-full max-w-full min-w-0">
      {/* 1. Sleek One-line Header Row */}
      <button
        type="button"
        onClick={() => { if (hasOutput) onToggle(); }}
        className="group/tool-summary inline-flex max-w-full cursor-pointer items-center gap-2 self-start text-left text-ui-base py-0.5 w-full transition-colors select-none"
      >
        <Icon className="w-4 h-4 shrink-0 text-foreground-subtlest" />
        <span className="font-medium text-foreground-secondary shrink-0">
          {kind}
        </span>

        {targetFile ? (
          <span className="inline-flex items-center gap-1.5 min-w-0 max-w-[340px] truncate">
            <FileText className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />
            <span className="font-mono text-ui-sm text-foreground font-medium truncate" title={fullPath}>
              {targetFile}
            </span>
          </span>
        ) : isBash ? null : (
          <span className="truncate text-foreground-subtlest font-mono text-ui-xs min-w-0 flex-1" title={tool.command}>
            {parsedArgs.cmd || tool.command}
          </span>
        )}

        {readLinesCount !== null && (
          <span className="shrink-0 font-mono text-ui-xs text-foreground-subtle font-medium">
            ({readLinesCount} lines)
          </span>
        )}

        {diffStats && (
          <span className="shrink-0 rounded bg-surface border border-border/40 px-1.5 py-0.5 font-mono text-ui-xs font-medium leading-none flex items-center gap-1 shadow-2xs">
            <span className="text-success font-semibold">+{diffStats.additions}</span>
            <span className="text-foreground-subtlest">/</span>
            <span className="text-destructive font-semibold">-{diffStats.deletions}</span>
            <span className="text-foreground-subtlest text-[10px]">lines</span>
          </span>
        )}

        {isRunning && (
          <span className="shrink-0 text-ui-xs text-foreground-subtlest flex items-center gap-1">
            <Loader2 className="w-3 h-3 animate-spin text-warning" />
            <span>Running…</span>
          </span>
        )}

        {isFailed && (
          <span className="shrink-0 text-ui-xs text-destructive font-mono">
            Failed
          </span>
        )}

        {hasOutput && (
          <ChevronDown
            className={cn(
              "w-3.5 h-3.5 shrink-0 text-foreground-subtlest transition-transform",
              !isOpen && "-rotate-90"
            )}
          />
        )}
      </button>

      {/* 2. Expanded Block directly below */}
      {isOpen && (
        <ToolExecutionContent tool={tool} />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 7. TOOL EXECUTION CONTENT (Direct block renderer)
// ---------------------------------------------------------------------------
export function ToolExecutionContent({ tool }: { tool: ToolExecution }) {
  const nameLower = (tool.toolName || '').toLowerCase();
  const isFailed = tool.status === 'failed' || (tool.output || '').includes('<tool_use_error>');
  const parsedArgs = useMemo(() => parseToolArgs(tool.command || '', tool.toolName || ''), [tool.command, tool.toolName]);

  const isRead = nameLower.includes('read') || nameLower.includes('view') || nameLower.includes('cat');
  const readInfo = useMemo(() => isRead && tool.output ? parseReadOutput(tool.output) : null, [isRead, tool.output]);
  const fullPath = parsedArgs.path || tool.diff?.filePath || readInfo?.filePath || '';
  let ext = fullPath ? (fullPath.split('.').pop() || '').toLowerCase() : '';
  if (!ext && nameLower.includes('python')) ext = 'py';
  if (!ext && (nameLower.includes('bash') || nameLower.includes('shell'))) ext = 'sh';

  // 1. Error
  if (isFailed) {
    return <ToolErrorBlock errorText={tool.output || 'Tool execution failed'} />;
  }

  // 2. Edit (Diff view: +1 -1 with highlighted rows)
  const isEdit = nameLower.includes('edit');
  const hasUnifiedDiffText = (tool.output || '').includes('@@ -') && ((tool.output || '').includes('+++') || (tool.output || '').includes('---'));

  if (isEdit || (tool.diff && !nameLower.includes('write')) || hasUnifiedDiffText) {
    return (
      <ToolInlineDiffBlock
        diff={tool.diff}
        rawDiffText={hasUnifiedDiffText ? tool.output : undefined}
        oldStr={parsedArgs.oldString}
        newStr={parsedArgs.newString}
        ext={ext}
      />
    );
  }

  // 3. Write (Full file code with solid green left indicator)
  const isWrite = nameLower.includes('write') || nameLower.includes('create');
  if (isWrite) {
    const codeContent = parsedArgs.content || tool.diff?.modifiedContent || tool.output || '';
    return <ToolWriteBlock content={codeContent} ext={ext} />;
  }

  // 4. Read (Clean file code with line numbers)
  if (isRead && tool.output) {
    return <ToolReadBlock content={readInfo ? readInfo.content : tool.output} ext={ext} />;
  }

  // 5. Terminal / Bash / Python / Default ($ command followed by output)
  const cmd = parsedArgs.cmd || tool.command || '';
  return <ToolTerminalBlock command={cmd} output={tool.output || ''} />;
}
