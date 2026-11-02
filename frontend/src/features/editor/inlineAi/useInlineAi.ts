import { useState, useRef, useCallback } from 'react';
import { EditorView } from '@codemirror/view';
import { InlineAiSessionState, CodeLensAction, SymbolLensInfo } from './inlineAiTypes';
import { setActiveDiffEffect } from './inlineDiffExtension';
import { computeLineDiff } from './inlineDiffUtils';
import { AgentEngine } from '../../../services/agentEngine';

interface UseInlineAiOptions {
  getView: () => EditorView | null;
  filePath: string;
  activeModel: string;
  onDocChange: (newContent: string) => void;
}

export function useInlineAi({
  getView,
  filePath,
  activeModel,
  onDocChange
}: UseInlineAiOptions) {
  const [session, setSession] = useState<InlineAiSessionState>({
    isOpen: false,
    status: 'idle',
    prompt: '',
    from: 0,
    to: 0,
    startLine: 1,
    endLine: 1,
    originalText: '',
    generatedText: ''
  });

  const abortControllerRef = useRef<AbortController | null>(null);
  const engineRef = useRef(new AgentEngine());
  const backupDocRef = useRef<string>('');

  const openInlineAi = useCallback((symbol?: SymbolLensInfo, initialAction?: CodeLensAction) => {
    const view = getView();
    if (!view) return;

    let from = view.state.selection.main.from;
    let to = view.state.selection.main.to;
    let originalText = '';
    let startLine = 1;
    let endLine = 1;

    if (symbol) {
      from = symbol.from;
      to = symbol.to;
      const doc = view.state.doc;
      const startLineObj = doc.lineAt(from);
      const endLineObj = doc.lineAt(Math.min(to, doc.length));
      startLine = startLineObj.number;
      endLine = endLineObj.number;
      originalText = doc.sliceString(startLineObj.from, endLineObj.to);
      from = startLineObj.from;
      to = endLineObj.to;
    } else if (from !== to) {
      const doc = view.state.doc;
      const startLineObj = doc.lineAt(from);
      const endLineObj = doc.lineAt(to);
      startLine = startLineObj.number;
      endLine = endLineObj.number;
      from = startLineObj.from;
      to = endLineObj.to;
      originalText = doc.sliceString(from, to);
    } else {
      const doc = view.state.doc;
      const lineObj = doc.lineAt(from);
      startLine = lineObj.number;
      endLine = lineObj.number;
      from = lineObj.from;
      to = lineObj.to;
      originalText = lineObj.text;
    }

    let defaultPrompt = '';
    if (initialAction === 'refactor') defaultPrompt = 'Refactor this code to improve clarity and idiomatic style';
    else if (initialAction === 'test') defaultPrompt = 'Write comprehensive unit tests with edge cases';
    else if (initialAction === 'explain') defaultPrompt = 'Add clean docstrings explaining logic and parameters';
    else if (initialAction === 'optimize') defaultPrompt = 'Optimize performance and minimize allocations';

    setSession({
      isOpen: true,
      status: 'idle',
      prompt: defaultPrompt,
      from,
      to,
      startLine,
      endLine,
      originalText,
      generatedText: '',
      actionKind: initialAction || 'custom',
      targetSymbol: symbol?.name
    });
  }, [getView]);

  const closeInlineAi = useCallback(() => {
    const view = getView();
    if (view) {
      view.dispatch({ effects: setActiveDiffEffect.of(null) });
    }
    abortControllerRef.current?.abort();
    setSession(prev => ({ ...prev, isOpen: false, status: 'idle' }));
  }, [getView]);

  const submitInlineAi = useCallback(async (promptText: string) => {
    const view = getView();
    if (!view || !promptText.trim()) return;

    abortControllerRef.current?.abort();
    abortControllerRef.current = new AbortController();

    setSession(prev => ({ ...prev, status: 'generating', error: undefined }));
    backupDocRef.current = view.state.doc.toString();

    const docText = view.state.doc.toString();
    const selection = session.originalText;
    const isInsertion = !selection.trim();

    const systemInstruction = `You are an expert pair programmer and inline code transformation assistant.
You are assisting in file: "${filePath}".
${isInsertion
  ? 'Generate net-new code to insert at cursor based on the user instruction.'
  : 'Transform the user-provided code block based on the user instruction.'}

RULES:
1. Return ONLY the code replacement.
2. Do NOT wrap the entire answer in markdown code fences (\`\`\`go, \`\`\`ts, etc.) unless specifically creating markdown text.
3. Preserve the indentation level of the original block.
4. Output no conversational remarks or pleasantries. Output clean, ready-to-run code only.`;

    const userPrompt = `FILE: ${filePath}
INSTRUCTION: ${promptText}

ORIGINAL CODE BLOCK (Lines ${session.startLine}-${session.endLine}):
\`\`\`
${selection}
\`\`\`

SURROUNDING FILE CONTEXT:
\`\`\`
${docText.slice(Math.max(0, session.from - 800), session.from)}
<--- EDIT TARGET --->
${docText.slice(session.to, Math.min(docText.length, session.to + 800))}
\`\`\`

Provide the replacement code for <--- EDIT TARGET ---> now.`;

    try {
      let accumulated = '';
      await engineRef.current.streamSideChat(
        activeModel,
        [
          { role: 'system', content: systemInstruction },
          { role: 'user', content: userPrompt }
        ],
        { workspacePath: '' },
        (token: string) => {
          accumulated += token;
        }
      );

      // Clean any accidental markdown wrappers from output
      let cleaned = accumulated.trim();
      if (cleaned.startsWith('```')) {
        const firstLineEnd = cleaned.indexOf('\n');
        if (firstLineEnd !== -1) {
          cleaned = cleaned.slice(firstLineEnd + 1);
        }
        if (cleaned.endsWith('```')) {
          cleaned = cleaned.slice(0, cleaned.length - 3).trimEnd();
        }
      }

      // Compute visual diff
      const diffLines = computeLineDiff(session.originalText, cleaned);
      const addedSet = new Set<number>();
      const deletedSet = new Set<number>();

      let currentRelativeLine = session.startLine;
      diffLines.forEach(d => {
        if (d.type === 'added') {
          addedSet.add(currentRelativeLine);
          currentRelativeLine++;
        } else if (d.type === 'deleted') {
          deletedSet.add(currentRelativeLine);
          currentRelativeLine++;
        } else {
          currentRelativeLine++;
        }
      });

      // Update document temporarily with the generated code to show inline diff
      view.dispatch({
        changes: { from: session.from, to: session.to, insert: cleaned },
        effects: setActiveDiffEffect.of({
          fromLine: session.startLine,
          toLine: session.startLine + cleaned.split('\n').length - 1,
          addedLines: addedSet,
          deletedLines: deletedSet
        })
      });

      setSession(prev => ({
        ...prev,
        status: 'diff',
        generatedText: cleaned
      }));
    } catch (err: any) {
      setSession(prev => ({
        ...prev,
        status: 'error',
        error: err?.message || 'Failed to generate inline code'
      }));
    }
  }, [getView, filePath, activeModel, session]);

  const acceptDiff = useCallback(() => {
    const view = getView();
    if (!view) return;

    // Clear visual diff decorations
    view.dispatch({ effects: setActiveDiffEffect.of(null) });
    onDocChange(view.state.doc.toString());
    closeInlineAi();
  }, [getView, onDocChange, closeInlineAi]);

  const rejectDiff = useCallback(() => {
    const view = getView();
    if (!view) return;

    // Revert back to backup document before generation
    if (backupDocRef.current) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: backupDocRef.current },
        effects: setActiveDiffEffect.of(null)
      });
      onDocChange(backupDocRef.current);
    }
    closeInlineAi();
  }, [getView, onDocChange, closeInlineAi]);

  const stopGeneration = useCallback(() => {
    engineRef.current.abort();
    abortControllerRef.current?.abort();
    setSession(prev => ({ ...prev, status: 'idle' }));
  }, []);

  return {
    session,
    openInlineAi,
    closeInlineAi,
    submitInlineAi,
    acceptDiff,
    rejectDiff,
    stopGeneration,
    setPrompt: (prompt: string) => setSession(prev => ({ ...prev, prompt }))
  };
}
