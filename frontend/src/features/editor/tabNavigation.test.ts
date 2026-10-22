import { describe, it, expect } from 'vitest';
import {
  pushNavHistory,
  getNavBack,
  getNavForward,
  cycleActiveTab,
  calculateActiveTabOnClose,
  filterTabsOnClose,
  updateTabBuffer,
  updateTabsOnRename,
  updateTabsOnDelete,
  NavHistoryState,
} from './tabNavigation';
import type { EditorTab } from '../../types';

describe('tabNavigation', () => {
  const tab1: EditorTab = { id: 't1', fileId: 'f1', fileName: 'a.ts', filePath: '/a.ts', type: 'code', content: 'code1' };
  const tab2: EditorTab = { id: 't2', fileId: 'f2', fileName: 'b.ts', filePath: '/b.ts', type: 'code', content: 'code2' };
  const tab3: EditorTab = { id: 't3', fileId: 'f3', fileName: 'c.ts', filePath: '/c.ts', type: 'code', content: 'code3' };

  describe('nav history', () => {
    it('pushes tab and strips content to save memory', () => {
      let nav: NavHistoryState = { stack: [], index: -1 };
      nav = pushNavHistory(nav, tab1);
      expect(nav.stack.length).toBe(1);
      expect(nav.stack[0].content).toBe('');
      expect(nav.index).toBe(0);

      nav = pushNavHistory(nav, tab2);
      expect(nav.stack.length).toBe(2);
      expect(nav.index).toBe(1);

      // Back navigation
      const back = getNavBack(nav);
      expect(back).not.toBeNull();
      expect(back?.targetTab.id).toBe('t1');
      expect(back?.nextNav.index).toBe(0);

      // Forward navigation from back
      const fwd = getNavForward(back!.nextNav);
      expect(fwd).not.toBeNull();
      expect(fwd?.targetTab.id).toBe('t2');
      expect(fwd?.nextNav.index).toBe(1);
    });

    it('ignores diff tabs in nav history', () => {
      const diffTab: EditorTab = { id: 'd1', fileId: 'fd1', fileName: 'd', filePath: '/d', type: 'diff', content: '' };
      const nav = { stack: [tab1], index: 0 };
      const res = pushNavHistory(nav, diffTab);
      expect(res).toBe(nav);
    });
  });

  describe('cycleActiveTab', () => {
    it('cycles forward and backward circularly', () => {
      const tabs = [tab1, tab2, tab3];
      expect(cycleActiveTab(tabs, 't1', 1)?.id).toBe('t2');
      expect(cycleActiveTab(tabs, 't3', 1)?.id).toBe('t1');
      expect(cycleActiveTab(tabs, 't1', -1)?.id).toBe('t3');
    });

    it('returns null if fewer than 2 tabs', () => {
      expect(cycleActiveTab([tab1], 't1', 1)).toBeNull();
    });
  });

  describe('calculateActiveTabOnClose', () => {
    it('activates neighbour when active tab is closed', () => {
      const tabs = [tab1, tab2, tab3];
      // Close t2 (middle) -> t3 (neighbor right)
      expect(calculateActiveTabOnClose(tabs, 't2', 't2')).toBe('t3');
      // Close t3 (end) -> t2 (neighbor left)
      expect(calculateActiveTabOnClose(tabs, 't3', 't3')).toBe('t2');
      // Close tab that is not active -> active tab remains unchanged
      expect(calculateActiveTabOnClose(tabs, 't2', 't1')).toBe('t1');
      // Close last remaining tab -> null
      expect(calculateActiveTabOnClose([tab1], 't1', 't1')).toBeNull();
    });
  });

  describe('updateTabBuffer', () => {
    it('updates modified buffer for target tab', () => {
      const tabs = [tab1, tab2];
      const { updatedTabs, changed } = updateTabBuffer(tabs, 'f1', 'new content');
      expect(changed).toBe(true);
      expect(updatedTabs[0].content).toBe('new content');
      expect(updatedTabs[0].isModified).toBe(true);
    });
  });

  describe('updateTabsOnRename & updateTabsOnDelete', () => {
    it('renames file and prefix directory paths', () => {
      const subTab: EditorTab = { id: 'tsub', fileId: 'fsub', fileName: 'sub.ts', filePath: '/dir/sub.ts', type: 'code', content: '' };
      const tabs = [subTab];
      const renamed = updateTabsOnRename(tabs, '/dir', '/newdir');
      expect(renamed[0].filePath).toBe('/newdir/sub.ts');
    });

    it('deletes file and prefix directory tabs', () => {
      const subTab: EditorTab = { id: 'tsub', fileId: 'fsub', fileName: 'sub.ts', filePath: '/dir/sub.ts', type: 'code', content: '' };
      const tabs = [tab1, subTab];
      const remaining = updateTabsOnDelete(tabs, '/dir');
      expect(remaining.length).toBe(1);
      expect(remaining[0].id).toBe('t1');
    });
  });
});
