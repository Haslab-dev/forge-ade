import React, { useMemo, useState, useEffect } from 'react';
import { 
  Pencil, 
  SquareTerminal, 
  Terminal,
  Search, 
  FileText, 
  ChevronDown, 
  Loader2, 
  AlertTriangle, 
  Wrench,
  Code2,
  Sparkles,
  Copy,
  Check,
  ExternalLink
} from 'lucide-react';
import { ToolExecution, FileDiff } from '../../types';
import { parseTwoFilesDiff } from '../diff/DiffViewer';
import { cn } from '../../lib/utils';
import { highlightCodeLine, highlightTerminalLine, highlightTerminalCommand } from '../../lib/diff-highlight';
import { WorkLogRow, WorkLogDetails, WorkLogStatusGlyph, type WorkLogStatus } from './WorkLog';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

export function cleanArgPath(val: string): string {
  if (!val) return '';
  let s = val.trim();
  // Strip surrounding quotes, braces, brackets, escapes
  s = s.replace(/^[\s{"'`\\]+/, '').replace(/[\s}"'`\\]+$/, '').trim();

  // Strip JSON property wrappers like Path":" or path: or "Path":
  s = s.replace(/^(?:["']?(?:path|filepath|file|filename|target|targetfile|target_file|command|commandline|content)["']?\s*[:=]\s*["']?)+/i, '').trim();

  // Strip surrounding quotes/braces again after removing property key prefix
  s = s.replace(/^[\s{"'`\\]+/, '').replace(/[\s}"'`\\]+$/, '').trim();

  // If there's an embedded absolute path inside like '... /Users/...', extract the clean path
  const absMatch = s.match(/((?:\/|[a-zA-Z]:[/\\])[\w.\- /\\@]+)/);
  if (absMatch && (s.startsWith('{') || s.includes('":') || s.includes('Path":'))) {
    s = absMatch[1].trim();
  }

  return s;
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
      if (typeof p === 'object' && p !== null) {
        path = p.path || p.Path || p.filepath || p.filePath || p.FilePath || p.file_path || 
               p.file || p.File || p.filename || p.fileName || p.FileName || 
               p.target_file || p.targetFile || p.TargetFile || p.target_path || p.targetPath || p.TargetPath || '';
        
        cmd = p.command || p.Command || p.cmd || p.Cmd || p.script || p.Script || 
              p.CommandLine || p.commandLine || p.command_line || p.cmdline || '';
        
        content = p.content || p.Content || p.text || p.Text || p.code || p.Code || 
                  p.body || p.Body || p.file_content || p.CodeContent || p.codeContent || '';
        
        oldString = p.old_string || p.oldString || p.OldString || p.old || p.target || p.Target || p.target_string || p.TargetString || '';
        newString = p.new_string || p.newString || p.NewString || p.new || p.replacement || p.Replacement || p.replacement_string || p.ReplacementString || '';

        // Case-insensitive key loop fallback if any fields are still empty
        for (const [key, val] of Object.entries(p)) {
          if (typeof val !== 'string') continue;
          const k = key.toLowerCase();
          if (!path && (k === 'path' || k === 'filepath' || k === 'file_path' || k === 'file' || k === 'filename' || k === 'targetfile' || k === 'target_file' || k === 'targetpath')) {
            path = val;
          }
          if (!cmd && (k === 'command' || k === 'cmd' || k === 'script' || k === 'commandline' || k === 'command_line')) {
            cmd = val;
          }
          if (!content && (k === 'content' || k === 'text' || k === 'code' || k === 'body' || k === 'file_content' || k === 'codecontent')) {
            content = val;
          }
          if (!oldString && (k === 'old_string' || k === 'oldstring' || k === 'old' || k === 'target' || k === 'target_string')) {
            oldString = val;
          }
          if (!newString && (k === 'new_string' || k === 'newstring' || k === 'new' || k === 'replacement' || k === 'replacement_string')) {
            newString = val;
          }
        }
      }
    } catch {
      const pm = str.match(/"(?:path|filepath|filePath|file_path|file|filename|target_file|targetFile|TargetFile|targetPath|target_path)":\s*"([^"]+)"/i);
      if (pm) path = pm[1];
      const cm = str.match(/"(?:command|cmd|script|CommandLine|commandLine)":\s*"([^"]+)"/i);
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
    if (!candidate.startsWith('{') && !candidate.includes('":') && !candidate.includes('\n') && candidate.length < 250 && candidate !== '{}' && candidate !== '{ }') {
      path = cleanArgPath(candidate);
    }
  }

  if (!cmd && !path) {
    if (str !== '{}' && str !== '{ }' && !str.startsWith('{"')) {
      cmd = str;
    }
  }

  if (nameLower.includes('python') && !content) {
    content = cmd || (str !== '{}' && str !== '{ }' ? str : '');
  }

  return { path, cmd, content, oldString, newString };
}

export function extractShellCommand(tool: ToolExecution, parsedArgs?: ParsedToolArgs): string {
  const args = parsedArgs || parseToolArgs(tool.command || '', tool.toolName || '');
  let raw = args.cmd || '';
  if (!raw && tool.command && tool.command !== '{}' && tool.command !== '{ }') {
    if (tool.command.trim().startsWith('{')) {
      const p = parseToolArgs(tool.command, tool.toolName || '');
      raw = p.cmd || '';
    } else {
      raw = tool.command;
    }
  }
  const toolAny = tool as any;
  if (!raw && toolAny.input) {
    if (typeof toolAny.input === 'string' && !toolAny.input.trim().startsWith('{')) {
      raw = toolAny.input;
    } else if (typeof toolAny.input === 'object' && toolAny.input !== null) {
      raw = toolAny.input.command || toolAny.input.cmd || toolAny.input.script || toolAny.input.CommandLine || '';
    }
  }
  if (!raw && toolAny.args && typeof toolAny.args === 'object' && toolAny.args !== null) {
    raw = toolAny.args.command || toolAny.args.cmd || toolAny.args.script || toolAny.args.CommandLine || '';
  }
  if (!raw) return '';

  const trimmed = raw.trim();
  const wrapped = /^(?:\S*\/)?(?:ba|z)?sh\s+-l?c\s+(['"])([\s\S]*)\1$/.exec(trimmed);
  const clean = (wrapped ? wrapped[2] : trimmed).trim();
  const firstLine = clean.split('\n')[0].trim();
  return firstLine || clean;
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
    <div className="mt-1 w-full max-w-full rounded-xl border border-destructive/30 bg-destructive/10 p-3 font-mono text-ui-xs text-destructive leading-relaxed max-h-60 overflow-y-auto overflow-x-auto whitespace-pre-wrap break-words select-text scrollbar-thin">
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
    if (rawDiffText || oldStr || newStr) {
      const fallbackText = rawDiffText || (newStr ? `+ ${newStr}` : oldStr ? `- ${oldStr}` : '');
      return (
        <div className="mt-1 w-full rounded-xl border border-border/50 bg-card p-3 font-mono text-ui-xs text-foreground-secondary whitespace-pre-wrap break-words select-text">
          {fallbackText}
        </div>
      );
    }
    return (
      <div className="mt-1 w-full rounded-xl border border-border/50 bg-card p-3 font-mono text-ui-xs text-foreground-subtlest">
        No visual line changes to display
      </div>
    );
  }

  return (
    <div className="mt-1 w-full max-w-full rounded-xl border border-border/50 bg-card/90 overflow-hidden text-ui-xs shadow-xs font-mono select-text">
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
    <div className="mt-1 w-full max-w-full rounded-xl border border-success/30 bg-success/5 border-l-[3px] border-l-success overflow-hidden text-ui-xs shadow-xs font-mono select-text">
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
export function ToolReadBlock({
  content,
  ext,
  filePath
}: {
  content: string;
  ext?: string;
  filePath?: string;
}) {
  const [showAll, setShowAll] = useState(false);
  const [copied, setCopied] = useState(false);
  const [fetchedContent, setFetchedContent] = useState<string | null>(null);
  const [fetchLoading, setFetchLoading] = useState(false);

  useEffect(() => {
    if (!content && filePath) {
      let active = true;
      setFetchLoading(true);
      ApiBridge.readFile(filePath)
        .then(data => {
          if (active && data) setFetchedContent(data);
        })
        .catch(() => {})
        .finally(() => {
          if (active) setFetchLoading(false);
        });
      return () => { active = false; };
    }
  }, [content, filePath]);

  const effectiveContent = content || fetchedContent || '';
  const lines = useMemo(() => effectiveContent.split('\n'), [effectiveContent]);
  const isLong = lines.length > 40;
  const visibleLines = isLong && !showAll ? lines.slice(0, 30) : lines;

  const fileName = filePath ? (filePath.split('/').filter(Boolean).pop() || filePath) : '';

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation();
    void navigator.clipboard.writeText(effectiveContent);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const handleOpenSideFile = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (filePath) {
      void useWorkspace.getState().openSideFile(filePath);
    }
  };

  return (
    <div className="mt-1 w-full max-w-full rounded-xl border border-border/50 bg-card/80 overflow-hidden text-ui-xs shadow-xs font-mono select-text">
      {filePath && (
        <div className="flex items-center justify-between border-b border-border/40 bg-surface/60 px-3 py-1.5 select-none font-sans">
          <div className="flex min-w-0 flex-1 items-center gap-2 truncate">
            <FileText className="size-3.5 text-primary shrink-0" />
            <span className="truncate text-foreground font-semibold font-mono text-ui-xs" title={filePath}>
              {fileName}
            </span>
            <span className="text-[11px] text-foreground-subtlest font-mono shrink-0 hidden sm:inline truncate">
              {filePath}
            </span>
          </div>
          <div className="flex items-center gap-1.5 shrink-0 ml-2">
            <button
              type="button"
              onClick={handleOpenSideFile}
              className="inline-flex items-center gap-1 text-[11px] font-sans text-foreground-subtle hover:text-foreground px-2 py-0.5 rounded hover:bg-surface-hover transition-colors cursor-pointer"
              title="Open in Side Viewer"
            >
              <ExternalLink className="size-3" />
              <span>Open in Side Viewer</span>
            </button>
            {effectiveContent && (
              <button
                type="button"
                onClick={handleCopy}
                className="flex items-center gap-1 text-[11px] font-sans text-foreground-subtle hover:text-foreground px-1.5 py-0.5 rounded hover:bg-surface-hover transition-colors cursor-pointer"
                title="Copy file content"
              >
                {copied ? (
                  <>
                    <Check className="size-3 text-success" />
                    <span className="text-success text-3xs">Copied</span>
                  </>
                ) : (
                  <>
                    <Copy className="size-3" />
                    <span className="text-3xs">Copy</span>
                  </>
                )}
              </button>
            )}
          </div>
        </div>
      )}

      <div className="max-h-96 overflow-y-auto overflow-x-auto p-2 scrollbar-thin">
        {fetchLoading ? (
          <div className="p-4 flex items-center justify-center gap-2 text-foreground-subtle text-ui-xs">
            <Loader2 className="size-3.5 animate-spin text-primary" />
            <span>Loading {fileName}...</span>
          </div>
        ) : !effectiveContent && filePath ? (
          <div className="p-4 text-center text-foreground-subtlest font-sans text-ui-xs space-y-2">
            <p>File content not returned in stream output.</p>
            <button
              type="button"
              onClick={handleOpenSideFile}
              className="inline-flex items-center gap-1.5 px-3 py-1 rounded-md border border-border bg-surface text-ui-xs text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
            >
              <ExternalLink className="size-3.5" />
              <span>Open {fileName} in Side Viewer</span>
            </button>
          </div>
        ) : (
          visibleLines.map((line, idx) => (
            <div key={idx} className="flex items-start px-2 py-0.5">
              <span className="w-8 text-right pr-3 text-foreground-subtlest select-none shrink-0 font-mono text-ui-xs">
                {idx + 1}
              </span>
              <span className="flex-1 whitespace-pre overflow-x-auto text-foreground-secondary">
                {highlightCodeLine(line, ext) || ' '}
              </span>
            </div>
          ))
        )}
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
  const [copied, setCopied] = useState(false);
  const lines = useMemo(() => (output || '').split('\n'), [output]);
  const isLong = lines.length > 35;
  const visibleLines = isLong && !showAll ? lines.slice(0, 25) : lines;

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation();
    const textToCopy = output ? (command && command !== '{}' ? `$ ${command}\n${output}` : output) : command;
    void navigator.clipboard.writeText(textToCopy);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className="mt-1 w-full max-w-full rounded-xl border border-border/50 bg-card/90 overflow-hidden font-mono text-ui-xs leading-relaxed shadow-xs select-text">
      {/* Terminal Card Header */}
      <div className="flex items-center justify-between border-b border-border/40 bg-surface/60 px-3 py-1.5 select-none">
        <div className="flex min-w-0 flex-1 items-center gap-2 truncate">
          <Terminal className="size-3.5 text-primary shrink-0" />
          <span className="truncate text-foreground font-semibold">
            {command && command !== '{}' && command !== '{ }' ? highlightTerminalCommand(command) : 'Terminal Output'}
          </span>
        </div>
        <button
          type="button"
          onClick={handleCopy}
          className="flex items-center gap-1 text-[11px] font-sans text-foreground-subtle hover:text-foreground px-1.5 py-0.5 rounded hover:bg-surface-hover transition-colors cursor-pointer shrink-0 ml-2"
          title="Copy command and output"
        >
          {copied ? (
            <>
              <Check className="size-3 text-success" />
              <span className="text-success text-3xs">Copied</span>
            </>
          ) : (
            <>
              <Copy className="size-3" />
              <span className="text-3xs">Copy</span>
            </>
          )}
        </button>
      </div>

      {/* Terminal Output */}
      <div className="p-3 bg-card/60 font-mono text-ui-xs leading-relaxed max-h-96 overflow-y-auto overflow-x-auto scrollbar-thin select-text space-y-0.5">
        {visibleLines.length === 0 || (visibleLines.length === 1 && !visibleLines[0]) ? (
          <span className="text-foreground-subtlest italic">(no output)</span>
        ) : (
          visibleLines.map((line, idx) => (
            <div key={idx} className="whitespace-pre-wrap break-words min-h-[1.125rem] text-foreground-secondary">
              {highlightTerminalLine(line) || ' '}
            </div>
          ))
        )}
      </div>

      {isLong && (
        <div className="p-2 border-t border-border/30 bg-surface/30">
          <button
            type="button"
            onClick={() => setShowAll(v => !v)}
            className="flex w-full items-center justify-center gap-1.5 rounded-md border border-border/60 bg-surface/80 py-1 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface hover:text-foreground cursor-pointer select-none"
          >
            <span>
              {showAll
                ? 'Collapse output'
                : `Output truncated (+${lines.length - 25} lines · click to expand)`}
            </span>
            <ChevronDown className={cn("w-3 h-3 transition-transform", showAll && "rotate-180")} />
          </button>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 5C. TOOL PYTHON BLOCK (Interactive Python REPL / multi-line code + result)
// ---------------------------------------------------------------------------
export function ToolPythonBlock({
  code,
  output
}: {
  code: string;
  output: string;
}) {
  const [showAllOutput, setShowAllOutput] = useState(false);
  const codeLines = useMemo(() => (code || '').split('\n'), [code]);
  const outputLines = useMemo(() => (output || '').split('\n'), [output]);
  const isOutputLong = outputLines.length > 25;
  const visibleOutputLines = isOutputLong && !showAllOutput ? outputLines.slice(0, 20) : outputLines;

  return (
    <div className="mt-1 w-full max-w-full rounded-xl border border-border/50 bg-card/90 overflow-hidden text-ui-xs shadow-xs font-mono select-text">
      {/* Code Header */}
      <div className="bg-surface/60 border-b border-border/40 px-3 py-1.5 flex items-center justify-between text-ui-xs text-foreground-subtle">
        <span className="font-semibold text-foreground-secondary flex items-center gap-1.5">
          <Code2 className="w-3.5 h-3.5 text-primary" />
          <span>Python</span>
        </span>
        <span className="text-2xs text-foreground-subtlest font-mono">
          {codeLines.length} {codeLines.length === 1 ? 'line' : 'lines'}
        </span>
      </div>

      {/* Code lines */}
      <div className="max-h-72 overflow-y-auto overflow-x-auto p-2 scrollbar-thin bg-card/60">
        {codeLines.map((line, idx) => (
          <div key={idx} className="flex items-start px-2 py-0.5">
            <span className="w-8 text-right pr-3 text-foreground-subtlest select-none shrink-0 font-mono text-ui-xs">
              {idx + 1}
            </span>
            <span className="flex-1 whitespace-pre overflow-x-auto text-foreground-secondary">
              {highlightCodeLine(line, 'py') || ' '}
            </span>
          </div>
        ))}
      </div>

      {/* Output / Result Header */}
      <div className="bg-surface/40 border-t border-border/40 px-3 py-1.5 flex items-center justify-between text-ui-xs text-foreground-subtle">
        <span className="font-semibold text-foreground-secondary">Output / Result</span>
      </div>

      {/* Output lines */}
      <div className="p-3 bg-surface/20 max-h-72 overflow-y-auto overflow-x-auto scrollbar-thin">
        {outputLines.length === 0 || (outputLines.length === 1 && !outputLines[0]) ? (
          <span className="text-foreground-subtlest italic">(no output)</span>
        ) : (
          visibleOutputLines.map((line, idx) => (
            <div key={idx} className="whitespace-pre-wrap break-words min-h-[1.125rem] text-foreground-secondary">
              {highlightTerminalLine(line) || ' '}
            </div>
          ))
        )}
        {isOutputLong && (
          <button
            type="button"
            onClick={() => setShowAllOutput(v => !v)}
            className="mt-2 flex w-full items-center justify-center gap-1.5 rounded-md border border-border/60 bg-surface/80 py-1 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface hover:text-foreground cursor-pointer select-none"
          >
            <span>
              {showAllOutput
                ? 'Collapse output'
                : `Output truncated (+${outputLines.length - 20} lines · click to expand)`}
            </span>
            <ChevronDown className={cn("w-3 h-3 transition-transform", showAllOutput && "rotate-180")} />
          </button>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// 5B. SKILL LOAD BLOCK (Scrollable SKILL.md snippet block)
// ---------------------------------------------------------------------------
export function parseSkillToolData(tool: ToolExecution): {
  name: string;
  description: string;
  path: string;
  body: string;
} {
  let name = '';
  let description = '';
  let path = '';
  let body = '';

  // 1. Try display payload if present
  const toolAny = tool as any;
  if (toolAny.display && typeof toolAny.display === 'object') {
    name = toolAny.display.skill_name || toolAny.display.name || '';
    description = toolAny.display.description || '';
    path = toolAny.display.path || toolAny.display.base_dir || '';
    body = toolAny.display.body || '';
  }

  // 2. Parse command / args
  if (!name && tool.command) {
    const raw = tool.command.trim();
    if (raw.startsWith('{')) {
      try {
        const parsed = JSON.parse(raw);
        name = parsed.skill || parsed.name || '';
      } catch {}
    } else {
      name = raw.replace(/^(?:load_skill|skill)\s+/i, '').trim();
    }
  }

  // 3. Parse output
  const output = tool.output || '';
  if (output.trim().startsWith('{')) {
    try {
      const parsed = JSON.parse(output.trim());
      if (!name) name = parsed.name || parsed.skill || '';
      if (!description) description = parsed.description || '';
      if (!path) path = parsed.path || parsed.base_dir || '';
      if (!body) body = parsed.body || '';
    } catch {}
  }

  if (!body && output) {
    // Check format: "Skill "foo" loaded successfully.\nDescription: bar\nDirectory: dir\n\nInstructions (SKILL.md):\n<content>"
    const instrMatch = output.match(/Instructions \(SKILL\.md\):\s*\n([\s\S]*)/i);
    if (instrMatch) {
      body = instrMatch[1].trim();
    }
    const nameMatch = output.match(/Skill\s+["']([^"']+)["']\s+loaded/i) || output.match(/Skill\s+([^\s]+)\s+loaded/i);
    if (nameMatch && !name) {
      name = nameMatch[1];
    }
    const descMatch = output.match(/Description:\s*([^\n]+)/i);
    if (descMatch && !description) {
      description = descMatch[1].trim();
    }
    const dirMatch = output.match(/Directory:\s*([^\n]+)/i);
    if (dirMatch && !path) {
      path = dirMatch[1].trim();
    }
    if (!body && !instrMatch) {
      body = output;
    }
  }

  return { name: name || 'skill', description, path, body };
}

export function ToolSkillBlock({
  tool,
  isOpen = true,
  onToggle,
  workspacePath
}: {
  tool: ToolExecution;
  isOpen?: boolean;
  onToggle?: () => void;
  workspacePath?: string;
}) {
  const { name, description, path, body } = useMemo(() => parseSkillToolData(tool), [tool]);
  const [copied, setCopied] = useState(false);

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(body);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const lines = useMemo(() => (body || '').split('\n'), [body]);
  const displayPath = path ? (path.split('/').filter(Boolean).slice(-2).join('/') || 'SKILL.md') : 'SKILL.md';

  return (
    <div className="w-full max-w-full rounded-lg bg-secondary dark:bg-input/20 overflow-hidden select-text text-left">
      {/* Header snippet info bar */}
      <div
        onClick={onToggle}
        className={cn(
          "flex items-center justify-between px-3 py-2 select-none transition-colors",
          onToggle && "cursor-pointer hover:bg-accent/50",
          isOpen ? "border-b border-border-subtle" : "rounded-lg"
        )}
      >
        <div className="flex items-center gap-2 min-w-0">
          <Sparkles className="size-4 shrink-0 text-icon-muted" />
          <span className="text-ui-sm font-medium text-foreground truncate">
            {name}
          </span>
          <span className="truncate font-mono text-2xs text-muted-foreground/90 shrink-0">
            {displayPath}
          </span>
          <span className="text-2xs text-fg-tertiary font-mono shrink-0 hidden sm:inline">
            ({lines.length} lines)
          </span>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <button
            type="button"
            onClick={handleCopy}
            className="flex items-center gap-1 text-ui-xs text-foreground-subtle hover:text-foreground transition-colors px-2 py-0.5 rounded hover:bg-surface-hover cursor-pointer"
            title="Copy SKILL.md content"
          >
            {copied ? (
              <>
                <Check className="w-3.5 h-3.5 text-success" />
                <span className="text-success font-medium">Copied</span>
              </>
            ) : (
              <>
                <Copy className="w-3.5 h-3.5" />
                <span>Copy</span>
              </>
            )}
          </button>
          {onToggle && (
            <ChevronDown
              className={cn(
                "w-3.5 h-3.5 text-foreground-subtlest transition-transform",
                !isOpen && "-rotate-90"
              )}
            />
          )}
        </div>
      </div>

      {isOpen && (
        <>
          {description && (
            <div className="px-3.5 pt-2 text-ui-xs text-foreground-subtle leading-normal">
              {description}
            </div>
          )}

          {/* Scrollable snippet block */}
          <div className="p-2.5">
            <pre className="m-0 max-h-72 overflow-y-auto overflow-x-auto rounded-md bg-code p-3 font-mono text-[length:var(--font-size-code,0.8125rem)]/[1.6] text-code-foreground select-text">
              <code>{body || '(No instructions body in SKILL.md)'}</code>
            </pre>
          </div>
        </>
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

  const isBash = nameLower.includes('bash') || nameLower.includes('shell') || nameLower.includes('exec') || nameLower.includes('command') || nameLower.includes('terminal');
  const isEdit = nameLower.includes('edit') || nameLower.includes('patch');
  const isWrite = nameLower.includes('write') || nameLower.includes('create');
  const isRead = nameLower.includes('read') || nameLower.includes('view') || nameLower.includes('cat');
  const isSearch = nameLower.includes('search') || nameLower.includes('grep') || nameLower.includes('glob') || nameLower.includes('find');
  const isPython = nameLower.includes('python');
  const isSkill = nameLower.includes('skill');

  const parsedArgs = useMemo(() => parseToolArgs(tool.command || '', tool.toolName || ''), [tool.command, tool.toolName]);
  const shellCmd = useMemo(() => isBash ? extractShellCommand(tool, parsedArgs) : '', [isBash, tool, parsedArgs]);

  // Clean target file
  let fullPath = parsedArgs.path || tool.diff?.filePath || '';
  if (workspacePath && fullPath.startsWith(workspacePath)) {
    fullPath = fullPath.slice(workspacePath.length).replace(/^\/+/, '');
  }
  fullPath = cleanArgPath(fullPath);
  const pathParts = fullPath ? fullPath.split('/').filter(Boolean) : [];
  const targetFile = pathParts.length > 0 ? pathParts[pathParts.length - 1] : fullPath;
  const targetDir = pathParts.length > 1 ? pathParts.slice(0, -1).join('/') : '';

  // Icon & Kind Label
  let kind = 'Ran';
  let Icon = SquareTerminal;

  if (isSkill) {
    kind = 'Skill';
    Icon = Sparkles;
  } else if (isEdit) {
    kind = 'Edit';
    Icon = Pencil;
  } else if (isWrite) {
    kind = 'Write';
    Icon = Pencil;
  } else if (isRead) {
    kind = 'Read';
    Icon = FileText;
  } else if (isSearch) {
    kind = 'Search';
    Icon = Search;
  } else if (isPython) {
    kind = 'Python';
    Icon = Code2;
  } else if (isBash) {
    kind = 'Ran';
    Icon = Terminal;
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

  const hasOutput = Boolean(tool.output || tool.diff || parsedArgs.content || parsedArgs.cmd || parsedArgs.path);

  // Skill cards are self-contained with their own header + snippet block.
  // Rendering ToolSkillBlock directly avoids the redundant double-header!
  if (isSkill) {
    return (
      <div className="w-full max-w-full min-w-0">
        <ToolSkillBlock
          tool={tool}
          isOpen={isOpen}
          onToggle={hasOutput ? onToggle : undefined}
          workspacePath={workspacePath}
        />
      </div>
    );
  }

  const statusMapped: WorkLogStatus =
    tool.status === 'failed' || isFailed
      ? 'failed'
      : isRunning
        ? 'running'
        : 'completed';

  const labelNode = isBash ? (
    <div className={cn('flex min-w-0 flex-1 items-center gap-1.5 truncate font-mono text-[length:var(--font-size-code,0.75rem)] select-none cursor-pointer', isRunning && 'live-tool-shine')}>
      <span className="shrink-0 rounded bg-surface border border-border/60 px-1 py-0.2 text-3xs font-semibold text-primary/85 select-none">
        $
      </span>
      <span className="truncate text-foreground/90 font-medium select-none cursor-pointer" title={shellCmd || tool.command}>
        {shellCmd || 'terminal command'}
      </span>
    </div>
  ) : (
    <div className={cn('flex min-w-0 flex-1 items-center gap-1.5 truncate select-none cursor-pointer', isRunning && 'live-tool-shine')}>
      <span className="shrink-0 rounded bg-surface border border-border/60 px-1.5 py-0.2 font-mono text-3xs font-medium text-foreground-subtle select-none cursor-pointer">
        {kind}
      </span>
      {targetFile ? (
        <span
          onClick={(e) => {
            e.stopPropagation();
            if (fullPath) void useWorkspace.getState().openSideFile(fullPath);
          }}
          className="inline-flex min-w-0 max-w-[380px] items-center gap-1.5 truncate hover:text-primary transition-colors cursor-pointer text-left select-none"
          title={`Open ${fullPath} in side preview`}
        >
          <span className="truncate font-mono text-[length:var(--font-size-code,0.6875rem)] font-medium text-foreground/90 hover:text-primary select-none">
            {targetFile}
          </span>
          {targetDir && (
            <span className="hidden sm:inline truncate font-mono text-2xs text-foreground-subtlest select-none">
              {targetDir}
            </span>
          )}
        </span>
      ) : isPython ? (
        <span
          className="min-w-0 flex-1 truncate font-mono text-[length:var(--font-size-code,0.6875rem)] select-none cursor-pointer"
          title={parsedArgs.content || parsedArgs.cmd || tool.command}
        >
          {parsedArgs.content ? parsedArgs.content.trim().split('\n')[0].slice(0, 90) : (parsedArgs.cmd || (tool.command !== '{}' && tool.command !== '{ }' ? tool.command : ''))}
        </span>
      ) : (
        <span className="min-w-0 flex-1 truncate font-mono text-[length:var(--font-size-code,0.6875rem)] select-none cursor-pointer" title={tool.command}>
          {parsedArgs.cmd || (tool.command !== '{}' && tool.command !== '{ }' ? tool.command : '')}
        </span>
      )}
    </div>
  );

  const trailingNode = (
    <div className="flex items-center gap-2 shrink-0">
      {readLinesCount !== null && (
        <span className="shrink-0 font-mono text-3xs tabular-nums text-fg-muted">
          {readLinesCount} lines
        </span>
      )}

      {diffStats && (
        <span className="shrink-0 flex items-center gap-1 font-mono text-3xs tabular-nums leading-none">
          <span className="text-diff-addition-foreground">+{diffStats.additions}</span>
          <span className="text-diff-deletion-foreground">-{diffStats.deletions}</span>
        </span>
      )}

      <WorkLogStatusGlyph status={statusMapped} />
    </div>
  );

  return (
    <div className="w-full max-w-full min-w-0 my-0.5">
      <WorkLogRow
        icon={<Icon className="size-4 stroke-2" />}
        label={labelNode}
        trailing={trailingNode}
        expanded={isOpen}
        onToggle={hasOutput ? onToggle : undefined}
      >
        {hasOutput && (
          <WorkLogDetails kind="panel">
            <ToolExecutionContent tool={tool} />
          </WorkLogDetails>
        )}
      </WorkLogRow>
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
  const isSkill = nameLower.includes('skill');

  // 1. Error
  if (isFailed) {
    return <ToolErrorBlock errorText={tool.output || 'Tool execution failed'} />;
  }

  // 1B. Skill load (Scrollable SKILL.md block)
  if (isSkill) {
    return <ToolSkillBlock tool={tool} />;
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

  // 4. Read (Clean file code with line numbers or Side Viewer link)
  if (isRead) {
    return <ToolReadBlock content={readInfo ? readInfo.content : (tool.output || '')} ext={ext} filePath={fullPath} />;
  }

  // 5. Python (Interactive REPL or multi-line script + evaluated result)
  if (nameLower.includes('python')) {
    const code = parsedArgs.content || parsedArgs.cmd || tool.command || '';
    return <ToolPythonBlock code={code} output={tool.output || ''} />;
  }

  // 6. Terminal / Bash / Default ($ command followed by output)
  const cmd = parsedArgs.cmd || (tool.command !== '{}' && tool.command !== '{ }' ? tool.command : '') || '';
  return <ToolTerminalBlock command={cmd} output={tool.output || ''} />;
}
