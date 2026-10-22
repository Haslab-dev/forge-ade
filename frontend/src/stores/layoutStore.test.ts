import { describe, it, expect, beforeEach } from 'vitest';
import { useLayoutStore, clampLeftSidebarWidth, clampRightPaneWidth } from './layoutStore';

describe('useLayoutStore (Zustand)', () => {
  beforeEach(() => {
    useLayoutStore.setState({
      activeActivity: 'explorer',
      agentLeftSidebarOpen: true,
      editorLeftSidebarOpen: true,
      rightActionDrawerOpen: false,
      leftSidebarWidth: 360,
      rightPaneWidth: 540,
      isFolderModalOpen: false,
      isCommandPaletteOpen: false,
      commandPaletteMode: 'files',
      settingsActiveSection: 'general',
    });
  });

  it('clamps sidebar widths within constraints', () => {
    expect(clampLeftSidebarWidth(100)).toBe(240); // min is 240
    expect(clampRightPaneWidth(150)).toBe(360); // min is 360
  });

  it('toggles left and right sidebar drawer states', () => {
    useLayoutStore.getState().setAgentLeftSidebarOpen(false);
    expect(useLayoutStore.getState().agentLeftSidebarOpen).toBe(false);

    useLayoutStore.getState().setRightActionDrawerOpen(true);
    expect(useLayoutStore.getState().rightActionDrawerOpen).toBe(true);
  });

  it('opens command palette with specified mode', () => {
    useLayoutStore.getState().openCommandPalette('commands');
    expect(useLayoutStore.getState().isCommandPaletteOpen).toBe(true);
    expect(useLayoutStore.getState().commandPaletteMode).toBe('commands');
  });
});
