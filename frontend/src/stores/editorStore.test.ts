import { describe, it, expect, beforeEach } from 'vitest';
import { useEditorStore } from './editorStore';
import type { EditorTab } from '../types';

describe('useEditorStore', () => {
  beforeEach(() => {
    useEditorStore.setState({
      openTabs: [],
      activeTabId: null,
      isSplitEditor: false,
    });
  });

  it('opens a new tab and activates it', () => {
    const tab: EditorTab = {
      id: 'tab-1',
      fileId: 'file-1',
      fileName: 'main.go',
      filePath: '/workspace/main.go',
      type: 'code',
    };

    useEditorStore.getState().openTab(tab);

    const state = useEditorStore.getState();
    expect(state.openTabs.length).toBe(1);
    expect(state.activeTabId).toBe('tab-1');
  });

  it('closes a tab and activates adjacent tab', () => {
    const tab1: EditorTab = { id: 'tab-1', fileId: 'f1', fileName: '1.go', filePath: '/1.go', type: 'code' };
    const tab2: EditorTab = { id: 'tab-2', fileId: 'f2', fileName: '2.go', filePath: '/2.go', type: 'code' };

    useEditorStore.getState().openTab(tab1);
    useEditorStore.getState().openTab(tab2);
    expect(useEditorStore.getState().activeTabId).toBe('tab-2');

    useEditorStore.getState().closeTab('tab-2');
    expect(useEditorStore.getState().openTabs.length).toBe(1);
    expect(useEditorStore.getState().activeTabId).toBe('tab-1');
  });

  it('preserves pinned tabs when closing others', () => {
    const tab1: EditorTab = { id: 'tab-1', fileId: 'f1', fileName: '1.go', filePath: '/1.go', type: 'code', isPinned: true };
    const tab2: EditorTab = { id: 'tab-2', fileId: 'f2', fileName: '2.go', filePath: '/2.go', type: 'code' };
    const tab3: EditorTab = { id: 'tab-3', fileId: 'f3', fileName: '3.go', filePath: '/3.go', type: 'code' };

    useEditorStore.getState().openTab(tab1);
    useEditorStore.getState().openTab(tab2);
    useEditorStore.getState().openTab(tab3);

    useEditorStore.getState().closeOtherTabs('tab-2');
    const state = useEditorStore.getState();
    expect(state.openTabs.map((t) => t.id)).toEqual(['tab-1', 'tab-2']);
    expect(state.activeTabId).toBe('tab-2');
  });
});
