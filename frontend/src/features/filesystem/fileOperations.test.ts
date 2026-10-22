import { describe, it, expect } from 'vitest';
import {
  findFileInTree,
  updateFolderChildrenInTree,
  updateFileContentInTree,
  resolveWorkspacePath,
} from './fileOperations';
import type { FileItem } from '../../types';

describe('fileOperations', () => {
  const tree: FileItem[] = [
    {
      id: 'f1',
      name: 'root.txt',
      path: '/workspace/root.txt',
      type: 'file',
      content: 'hello',
    },
    {
      id: 'd1',
      name: 'src',
      path: '/workspace/src',
      type: 'folder',
      children: [
        {
          id: 'f2',
          name: 'index.ts',
          path: '/workspace/src/index.ts',
          type: 'file',
          content: 'export default 1;',
        },
      ],
    },
  ];

  describe('findFileInTree', () => {
    it('finds top-level and nested files by path or name', () => {
      expect(findFileInTree(tree, '/workspace/root.txt')?.id).toBe('f1');
      expect(findFileInTree(tree, 'index.ts')?.id).toBe('f2');
      expect(findFileInTree(tree, 'nonexistent')).toBeNull();
    });
  });

  describe('updateFolderChildrenInTree', () => {
    it('updates children of target folder recursively', () => {
      const newChild: FileItem = {
        id: 'f3',
        name: 'app.ts',
        path: '/workspace/src/app.ts',
        type: 'file',
      };
      const updated = updateFolderChildrenInTree(tree, '/workspace/src', [newChild]);
      const folder = updated.find(i => i.id === 'd1');
      expect(folder?.children?.length).toBe(1);
      expect(folder?.children?.[0].id).toBe('f3');
    });
  });

  describe('updateFileContentInTree', () => {
    it('updates file content and sets isModified true', () => {
      const updated = updateFileContentInTree(tree, '/workspace/src/index.ts', 'new content');
      const file = findFileInTree(updated, '/workspace/src/index.ts');
      expect(file?.content).toBe('new content');
      expect(file?.isModified).toBe(true);
    });
  });

  describe('resolveWorkspacePath', () => {
    it('keeps absolute paths untouched', () => {
      expect(resolveWorkspacePath('/ws', '/abs/path.ts')).toBe('/abs/path.ts');
      expect(resolveWorkspacePath('C:\\ws', 'C:\\foo\\bar')).toBe('C:\\foo\\bar');
    });

    it('resolves relative paths against workspace root', () => {
      expect(resolveWorkspacePath('/ws', 'src/app.ts')).toBe('/ws/src/app.ts');
      expect(resolveWorkspacePath('/ws/', 'components/btn.tsx')).toBe('/ws/components/btn.tsx');
    });
  });
});
