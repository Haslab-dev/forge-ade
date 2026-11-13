import { describe, it, expect } from 'vitest';
import { detectSymbolsInDoc } from './symbolDetector';

describe('detectSymbolsInDoc', () => {
  it('detects Go functions, methods, and structs', () => {
    const code = `
package main

type ServerConfig struct {
    Port int
}

func NewServer() *Server {
    return &Server{}
}

func (s *Server) Start() error {
    return nil
}
`;
    const symbols = detectSymbolsInDoc(code);
    expect(symbols.map(s => ({ name: s.name, kind: s.kind }))).toEqual([
      { name: 'ServerConfig', kind: 'struct' },
      { name: 'NewServer', kind: 'function' },
      { name: 'Start', kind: 'method' }
    ]);
  });

  it('detects TypeScript / JavaScript functions and classes', () => {
    const code = `
export class AgentSessionManager {
}

export async function processTurn(prompt: string) {
    return prompt;
}

const formatTitle = (raw: string) => {
    return raw.trim();
};
`;
    const symbols = detectSymbolsInDoc(code);
    expect(symbols.map(s => ({ name: s.name, kind: s.kind }))).toEqual([
      { name: 'AgentSessionManager', kind: 'class' },
      { name: 'processTurn', kind: 'function' },
      { name: 'formatTitle', kind: 'function' }
    ]);
  });

  it('skips control keywords like if, for, while', () => {
    const code = `
if x > 10 {
    for i := 0; i < 10; i++ {
    }
}
`;
    const symbols = detectSymbolsInDoc(code);
    expect(symbols).toHaveLength(0);
  });
});
