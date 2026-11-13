import React, { useState } from 'react';
import { Renderer } from '@openuidev/react-lang';
import type { OpenUIError } from '@openuidev/react-lang';
import { AlertTriangle } from 'lucide-react';
import { opuiLibrary, handleOpenUIAction } from './opuiLibrary';
import { useWorkspace } from '../../stores/workspaceStore';

interface OpenUIBlockProps {
  code: string;
  streaming?: boolean;
}

/**
 * Renders an ```opui or ```openui generative UI block using OpenUI Lang.
 * Renders only the main component content directly without unnecessary outer
 * container wrappers or redundant extra backgrounds.
 */
export const OpenUIBlock: React.FC<OpenUIBlockProps> = ({ code, streaming = false }) => {
  const [failed, setFailed] = useState<OpenUIError[] | null>(null);

  if (failed && failed.length > 0 && !streaming) {
    return (
      <div className="my-3 rounded-xl border border-amber-300 bg-amber-50 p-4 text-neutral-900 [color-scheme:light]">
        <div className="mb-1.5 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-amber-800">
          <AlertTriangle className="size-3.5 text-amber-600" />
          <span>OpenUI block could not be rendered</span>
        </div>
        <pre className="overflow-x-auto font-mono text-xs text-neutral-700 leading-relaxed">{code}</pre>
      </div>
    );
  }

  const sendPrompt = (text: string) => {
    const store = useWorkspace.getState();
    if (store.activeSessionId) {
      void store.sendAgentPrompt(text);
    }
  };

  return (
    <div className="my-2 select-text">
      <Renderer
        response={code}
        library={opuiLibrary}
        isStreaming={streaming}
        onAction={(event) => handleOpenUIAction(event, sendPrompt)}
        onError={(errors) => {
          if (errors && errors.length > 0) {
            console.warn('[OpenUI] Render issues:', errors);
          }
          setFailed(errors);
        }}
      />
    </div>
  );
};

export default OpenUIBlock;
