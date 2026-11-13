import { describe, it, expect, beforeEach } from 'vitest';
import { useWorkspace } from './workspaceStore';

describe('useWorkspace (Full Zustand Store)', () => {
  beforeEach(() => {
    useWorkspace.setState({
      mode: 'agent',
      agentWorkspacePath: '/test/agent/path',
      editorWorkspacePath: '/test/editor/path',
      activeTabId: null,
      openTabs: []
    });
  });

  it('provides synchronous access via getState() outside React', () => {
    const state = useWorkspace.getState();
    expect(state.mode).toBe('agent');
    expect(state.activeWorkspacePath).toBe('/test/agent/path');
  });

  it('dynamically computes activeWorkspacePath when switching mode', () => {
    expect(useWorkspace.getState().activeWorkspacePath).toBe('/test/agent/path');
    useWorkspace.getState().setMode('editor');
    expect(useWorkspace.getState().mode).toBe('editor');
    expect(useWorkspace.getState().activeWorkspacePath).toBe('/test/editor/path');
  });

  it('allows fine-grained Zustand subscriptions without React Context', () => {
    let observedMode = '';
    const unsub = useWorkspace.subscribe((state) => {
      observedMode = state.mode;
    });

    useWorkspace.getState().setMode('settings');
    expect(observedMode).toBe('settings');
    unsub();
  });

  it('manages tabs and activeTabId directly in store', () => {
    const tab = {
      id: 'tab-1',
      fileId: 'file-1',
      fileName: 'main.go',
      filePath: '/test/main.go',
      content: 'package main',
      type: 'code' as const
    };

    useWorkspace.getState().openTab(tab);
    expect(useWorkspace.getState().activeTabId).toBe('tab-1');

    useWorkspace.getState().setTabBuffer('file-1', 'package main // updated');
    const updated = useWorkspace.getState().openTabs.find(t => t.id === 'tab-1');
    expect(updated?.content).toBe('package main // updated');
    expect(updated?.isModified).toBe(true);

    useWorkspace.getState().closeTab('tab-1');
    expect(useWorkspace.getState().openTabs.length).toBe(0);
  });
});
