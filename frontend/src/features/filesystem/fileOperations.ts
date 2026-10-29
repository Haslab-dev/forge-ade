/**
 * File System Tree Operations & Utilities
 * Pure recursive tree manipulators and path resolvers decoupled from store/UI.
 */

import type { FileItem } from '../../types';

/**
 * Searches a file tree recursively for a file by path or name.
 */
export function findFileInTree(items: FileItem[], path: string): FileItem | null {
  const clean = path.replace(/^\/+/, '');
  for (const item of items) {
    const itemClean = item.path.replace(/^\/+/, '');
    if (itemClean === clean || item.path === path || item.name === path || item.path.endsWith(clean)) {
      return item;
    }
    if (item.children) {
      const found = findFileInTree(item.children, path);
      if (found) return found;
    }
  }
  return null;
}

/**
 * Recursively updates children of a matching folder node in a tree.
 */
export function updateFolderChildrenInTree(
  items: FileItem[],
  folderPath: string,
  children: FileItem[]
): FileItem[] {
  const norm = (p?: string) => (p ? p.replace(/\\/g, '/').replace(/\/+$/, '') : '');
  const target = norm(folderPath);

  return items.map(item => {
    const itemPath = norm(item.path);
    if (itemPath === target || item.id === folderPath || item.name === folderPath) {
      return { ...item, children };
    }
    if (item.children) {
      return { ...item, children: updateFolderChildrenInTree(item.children, folderPath, children) };
    }
    return item;
  });
}

/**
 * Recursively updates content and dirty flag for a matching file in a tree.
 */
export function updateFileContentInTree(
  items: FileItem[],
  targetPathOrId: string,
  newContent: string
): FileItem[] {
  return items.map(item => {
    if (item.id === targetPathOrId || item.path === targetPathOrId) {
      return { ...item, content: newContent, isModified: true };
    }
    if (item.children) {
      return { ...item, children: updateFileContentInTree(item.children, targetPathOrId, newContent) };
    }
    return item;
  });
}

/**
 * Resolves a full absolute path from a potentially relative path and active workspace root.
 */
export function resolveWorkspacePath(activeWorkspacePath: string, targetPath: string): string {
  if (targetPath.startsWith('/') || targetPath.includes(':')) {
    return targetPath;
  }
  const cleanWorkspace = activeWorkspacePath.replace(/\/+$/, '');
  const cleanTarget = targetPath.replace(/^\/+/, '');
  return `${cleanWorkspace}/${cleanTarget}`;
}
