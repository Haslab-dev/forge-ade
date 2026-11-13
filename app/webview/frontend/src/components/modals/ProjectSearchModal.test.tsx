import React from 'react';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { ProjectSearchModal } from './ProjectSearchModal';
import { WorkspaceProvider, useWorkspace } from '../../stores/workspaceStore';
import { useLayoutStore } from '../../stores/layoutStore';
import { isEditableElement } from '../../lib/utils';
import { ApiBridge } from '../../services/apiBridge';
import { AgentApi } from '../../features/agent/agent.api';

describe('ProjectSearchModal', () => {
  beforeEach(() => {
    useLayoutStore.setState({
      isProjectSearchOpen: false,
      projectSearchInitialQuery: '',
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders nothing when isProjectSearchOpen is false', () => {
    render(
      <WorkspaceProvider>
        <ProjectSearchModal />
      </WorkspaceProvider>
    );

    expect(screen.queryByRole('dialog', { name: /Project Search/i })).not.toBeInTheDocument();
  });

  it('renders modal with search input when isProjectSearchOpen is true', () => {
    useLayoutStore.setState({ isProjectSearchOpen: true, projectSearchInitialQuery: 'hello-world' });

    render(
      <WorkspaceProvider>
        <ProjectSearchModal />
      </WorkspaceProvider>
    );

    expect(screen.getByRole('dialog', { name: /Project Search/i })).toBeInTheDocument();
    const input = screen.getByPlaceholderText(/Search across project/i) as HTMLInputElement;
    expect(input).toBeInTheDocument();
    expect(input.value).toBe('hello-world');
  });

  it('toggles replace controls and filter controls', async () => {
    useLayoutStore.setState({ isProjectSearchOpen: true });

    render(
      <WorkspaceProvider>
        <ProjectSearchModal />
      </WorkspaceProvider>
    );

    const replaceBtn = screen.getByTitle(/Toggle Replace/i);
    expect(screen.queryByPlaceholderText(/Replace with\.\.\./i)).not.toBeInTheDocument();

    fireEvent.click(replaceBtn);
    expect(screen.getByPlaceholderText(/Replace with\.\.\./i)).toBeInTheDocument();

    const filterBtn = screen.getByTitle(/Toggle File Filter Globs/i);
    expect(screen.queryByPlaceholderText(/e\.g\. \*\.ts/i)).not.toBeInTheDocument();

    fireEvent.click(filterBtn);
    expect(screen.getByPlaceholderText(/e\.g\. \*\.ts/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/node_modules/i)).toBeInTheDocument();
  });

  it('closes on Escape key press', () => {
    useLayoutStore.setState({ isProjectSearchOpen: true });

    render(
      <WorkspaceProvider>
        <ProjectSearchModal />
      </WorkspaceProvider>
    );

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(useLayoutStore.getState().isProjectSearchOpen).toBe(false);
  });

  it('executes search via ApiBridge and displays results', async () => {
    const mockContentHits = [
      { path: '/project/src/index.ts', line: 12, text: 'console.log("hello world");' }
    ];
    vi.spyOn(ApiBridge, 'searchContent').mockResolvedValue(mockContentHits);
    vi.spyOn(AgentApi, 'searchContent').mockResolvedValue(mockContentHits);
    vi.spyOn(ApiBridge, 'searchFilename').mockResolvedValue([]);
    vi.spyOn(AgentApi, 'searchFilename').mockResolvedValue([]);

    useLayoutStore.setState({ isProjectSearchOpen: true, projectSearchInitialQuery: 'hello' });

    render(
      <WorkspaceProvider>
        <ProjectSearchModal />
      </WorkspaceProvider>
    );

    const input = screen.getByPlaceholderText(/Search across project/i);
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true });

    await waitFor(() => {
      expect(screen.getByText('index.ts')).toBeInTheDocument();
      expect(screen.getByText(/console\.log/i)).toBeInTheDocument();
    }, { timeout: 2000 });
  });

  it('in agent mode, opening a result opens in sidebar and stays in agent mode', async () => {
    const mockHits = [{ path: '/project/src/index.ts', line: 42, text: 'const foo = "bar";' }];
    vi.spyOn(ApiBridge, 'searchContent').mockResolvedValue(mockHits);
    vi.spyOn(AgentApi, 'searchContent').mockResolvedValue(mockHits);
    vi.spyOn(ApiBridge, 'searchFilename').mockResolvedValue([]);
    vi.spyOn(AgentApi, 'searchFilename').mockResolvedValue([]);
    vi.spyOn(ApiBridge, 'fileSize').mockResolvedValue(100);
    vi.spyOn(ApiBridge, 'readFile').mockResolvedValue('const foo = "bar";');

    useLayoutStore.setState({ isProjectSearchOpen: true, projectSearchInitialQuery: 'foo' });
    useWorkspace.setState({ mode: 'agent' });

    render(
      <WorkspaceProvider>
        <ProjectSearchModal />
      </WorkspaceProvider>
    );

    const input = screen.getByPlaceholderText(/Search across project/i);
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true });

    await waitFor(() => {
      expect(screen.getByText('index.ts')).toBeInTheDocument();
    });

    const resultRow = screen.getByText('foo');
    fireEvent.click(resultRow);

    await waitFor(() => {
      expect(useLayoutStore.getState().isProjectSearchOpen).toBe(false);
      expect(useWorkspace.getState().mode).toBe('agent');
      expect(useWorkspace.getState().isRightActionDrawerOpen).toBe(true);
      expect(useWorkspace.getState().activeSideFile).toBe('/project/src/index.ts');
      expect(useWorkspace.getState().activeSideFileLine).toBe(42);
    });
  });

  it('in editor mode, opening a result opens in editor pane tab', async () => {
    const mockHits = [{ path: '/project/src/index.ts', line: 15, text: 'function run() {}' }];
    vi.spyOn(ApiBridge, 'searchContent').mockResolvedValue(mockHits);
    vi.spyOn(AgentApi, 'searchContent').mockResolvedValue(mockHits);
    vi.spyOn(ApiBridge, 'searchFilename').mockResolvedValue([]);
    vi.spyOn(AgentApi, 'searchFilename').mockResolvedValue([]);
    vi.spyOn(ApiBridge, 'fileSize').mockResolvedValue(100);
    vi.spyOn(ApiBridge, 'readFile').mockResolvedValue('function run() {}');

    useLayoutStore.setState({ isProjectSearchOpen: true, projectSearchInitialQuery: 'run' });
    useWorkspace.setState({ mode: 'editor' });

    render(
      <WorkspaceProvider>
        <ProjectSearchModal />
      </WorkspaceProvider>
    );

    const input = screen.getByPlaceholderText(/Search across project/i);
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true });

    await waitFor(() => {
      expect(screen.getByText('index.ts')).toBeInTheDocument();
    });

    const resultRow = screen.getByText('run');
    fireEvent.click(resultRow);

    await waitFor(() => {
      expect(useLayoutStore.getState().isProjectSearchOpen).toBe(false);
      expect(useWorkspace.getState().mode).toBe('editor');
      const openTab = useWorkspace.getState().openTabs.find(t => t.filePath === '/project/src/index.ts');
      expect(openTab).toBeDefined();
      expect(openTab?.line).toBe(15);
    });
  });
});

describe('isEditableElement utility', () => {
  it('identifies inputs, textareas, and select elements as editable', () => {
    const input = document.createElement('input');
    const textarea = document.createElement('textarea');
    const select = document.createElement('select');
    const div = document.createElement('div');
    const button = document.createElement('button');

    expect(isEditableElement(input)).toBe(true);
    expect(isEditableElement(textarea)).toBe(true);
    expect(isEditableElement(select)).toBe(true);
    expect(isEditableElement(div)).toBe(false);
    expect(isEditableElement(button)).toBe(false);
  });

  it('identifies elements inside CodeMirror or contentEditable as editable', () => {
    const cmParent = document.createElement('div');
    cmParent.className = 'cm-editor';
    const cmChild = document.createElement('span');
    cmParent.appendChild(cmChild);
    document.body.appendChild(cmParent);

    expect(isEditableElement(cmChild)).toBe(true);

    const editableDiv = document.createElement('div');
    editableDiv.contentEditable = 'true';
    const child = document.createElement('span');
    editableDiv.appendChild(child);
    document.body.appendChild(editableDiv);

    expect(isEditableElement(child)).toBe(true);

    document.body.removeChild(cmParent);
    document.body.removeChild(editableDiv);
  });
});
