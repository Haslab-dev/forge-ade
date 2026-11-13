import { describe, it, expect, vi } from 'vitest';
import { Effect, Exit, Cause } from 'effect';
import {
  WailsBridgeError,
  FileSystemError,
  GitError,
  AgentSessionError,
  DatabaseError,
  AutomationError,
  SSHRemoteError,
} from './errors';
import { callEffect, fromPromiseCall, wailsRetryPolicy } from './wailsBridge';
import { runPromise, runPromiseExit, runSync } from './runtime';
import { FileSystemService, GitService } from './services';
import { ApiBridgeEffect } from '../services/apiBridge';

describe('Effect-TS Core Architecture', () => {
  describe('Domain Tagged Errors', () => {
    it('creates typed FileSystemError with structured fields', () => {
      const err = new FileSystemError({
        operation: 'readFile',
        path: '/tmp/test.txt',
        message: 'File not found',
      });
      expect(err._tag).toBe('FileSystemError');
      expect(err.operation).toBe('readFile');
      expect(err.path).toBe('/tmp/test.txt');
      expect(err.message).toBe('File not found');
    });

    it('creates typed GitError and WailsBridgeError', () => {
      const gitErr = new GitError({
        operation: 'commit',
        repoPath: '/workspace',
        message: 'nothing to commit',
      });
      expect(gitErr._tag).toBe('GitError');

      const bridgeErr = new WailsBridgeError({
        method: 'GetFileTree',
        message: 'RPC timeout',
      });
      expect(bridgeErr._tag).toBe('WailsBridgeError');
      expect(bridgeErr.method).toBe('GetFileTree');
    });
  });

  describe('wailsBridge helpers', () => {
    it('wraps a successful async call in Effect', async () => {
      const effect = fromPromiseCall('testMethod', async () => 'hello effect');
      const result = await runPromise(effect);
      expect(result).toBe('hello effect');
    });

    it('maps promise rejections to typed WailsBridgeError', async () => {
      const effect = fromPromiseCall('failingMethod', async () => {
        throw new Error('Connection refused');
      });
      const exit = await runPromiseExit(effect);
      expect(Exit.isFailure(exit)).toBe(true);
      if (Exit.isFailure(exit)) {
        const found = Cause.findError(exit.cause);
        expect(found._tag).toBe('Success');
        if (found._tag === 'Success') {
          const err = ((found as any).success ?? (found as any).value) as WailsBridgeError;
          expect(err._tag).toBe('WailsBridgeError');
          expect(err.method).toBe('failingMethod');
          expect(err.message).toBe('Connection refused');
        }
      }
    });

    it('callEffect returns fallback when function is undefined', async () => {
      const effect = callEffect('missingFunc', undefined, 'fallbackValue');
      const result = await runPromise(effect);
      expect(result).toBe('fallbackValue');
    });
  });

  describe('Runtime Execution Primitives', () => {
    it('runSync executes synchronous effects', () => {
      const effect = Effect.succeed(42);
      expect(runSync(effect)).toBe(42);
    });

    it('runPromise resolves successful effects', async () => {
      const effect = Effect.gen(function* () {
        const a = yield* Effect.succeed(10);
        const b = yield* Effect.succeed(20);
        return a + b;
      });
      const result = await runPromise(effect);
      expect(result).toBe(30);
    });

    it('runPromiseExit returns Exit.Success without throwing', async () => {
      const effect = Effect.succeed('ok');
      const exit = await runPromiseExit(effect);
      expect(Exit.isSuccess(exit)).toBe(true);
      if (Exit.isSuccess(exit)) {
        expect(exit.value).toBe('ok');
      }
    });
  });

  describe('Context & Service Layer Overrides', () => {
    it('allows overriding FileSystemService with custom mock implementations', async () => {
      const mockFs = {
        openFolder: vi.fn(() => Effect.succeed({ name: 'mockFolder' })),
        openWorkspace: vi.fn(() => Effect.succeed({ name: 'mockWorkspace' })),
        readFile: vi.fn((path: string) => Effect.succeed(`content of ${path}`)),
        readFileBase64: vi.fn(() => Effect.succeed('base64')),
        writeFile: vi.fn(() => Effect.void),
        writeFileBase64: vi.fn(() => Effect.void),
        createFile: vi.fn(() => Effect.void),
        createFolder: vi.fn(() => Effect.void),
        deleteFile: vi.fn(() => Effect.void),
        renameFile: vi.fn(() => Effect.void),
        moveFile: vi.fn(() => Effect.void),
        getFileTree: vi.fn(() => Effect.succeed([])),
        listDirectory: vi.fn(() => Effect.succeed([])),
        expandPath: vi.fn((path: string) => Effect.succeed(path)),
        isDir: vi.fn(() => Effect.succeed(false)),
        copyPath: vi.fn(() => Effect.void),
        openInFinder: vi.fn(() => Effect.void),
        getFileSize: vi.fn(() => Effect.succeed(1024)),
      };

      const program = Effect.gen(function* () {
        const fs = yield* FileSystemService;
        const content = yield* fs.readFile('/custom/path.ts');
        return content;
      });

      const overridden = Effect.provideService(program, FileSystemService, mockFs);
      const res = await runPromise(overridden);
      expect(res).toBe('content of /custom/path.ts');
      expect(mockFs.readFile).toHaveBeenCalledWith('/custom/path.ts');
    });
  });

  describe('ApiBridgeEffect Pipelines', () => {
    it('composes multiple ApiBridgeEffect calls in a single Effect.gen pipeline', async () => {
      const mockFs = {
        openFolder: vi.fn(() => Effect.succeed(null)),
        openWorkspace: vi.fn(() => Effect.succeed(null)),
        readFile: vi.fn((path: string) => Effect.succeed(`// content of ${path}`)),
        readFileBase64: vi.fn(() => Effect.succeed('')),
        writeFile: vi.fn(() => Effect.void),
        writeFileBase64: vi.fn(() => Effect.void),
        createFile: vi.fn(() => Effect.void),
        createFolder: vi.fn(() => Effect.void),
        deleteFile: vi.fn(() => Effect.void),
        renameFile: vi.fn(() => Effect.void),
        moveFile: vi.fn(() => Effect.void),
        getFileTree: vi.fn(() => Effect.succeed([])),
        listDirectory: vi.fn(() => Effect.succeed([])),
        expandPath: vi.fn((path: string) => Effect.succeed(path)),
        isDir: vi.fn(() => Effect.succeed(true)),
        copyPath: vi.fn(() => Effect.void),
        openInFinder: vi.fn(() => Effect.void),
        getFileSize: vi.fn(() => Effect.succeed(512)),
      };

      const pipeline = Effect.gen(function* () {
        const isDirectory = yield* ApiBridgeEffect.isDir('/src');
        const size = yield* ApiBridgeEffect.fileSize('/src/index.ts');
        const content = yield* ApiBridgeEffect.readFile('/src/index.ts');
        return { isDirectory, size, content };
      });

      const result = await runPromise(Effect.provideService(pipeline, FileSystemService, mockFs));
      expect(result).toEqual({
        isDirectory: true,
        size: 512,
        content: '// content of /src/index.ts',
      });
    });
  });

  describe('React 19 useRunEffect Hook', () => {
    it('initializes with null data and loading false when autoRun is false', async () => {
      const { renderHook } = await import('@testing-library/react');
      const { useRunEffect } = await import('./hooks');

      const { result } = renderHook(() =>
        useRunEffect(() => Effect.succeed('executed data'), [], false)
      );

      expect(result.current.data).toBeNull();
      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it('executes and updates state with data on demand', async () => {
      const { renderHook, act } = await import('@testing-library/react');
      const { useRunEffect } = await import('./hooks');

      const { result } = renderHook(() =>
        useRunEffect(() => Effect.succeed('resolved payload'), [], false)
      );

      await act(async () => {
        await result.current.execute();
      });

      expect(result.current.data).toBe('resolved payload');
      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it('captures typed errors into error state when effect fails', async () => {
      const { renderHook, act } = await import('@testing-library/react');
      const { useRunEffect } = await import('./hooks');

      const { result } = renderHook(() =>
        useRunEffect(
          () =>
            Effect.fail(
              new FileSystemError({
                operation: 'test',
                message: 'permission denied',
              })
            ),
          [],
          false
        )
      );

      await act(async () => {
        await result.current.execute();
      });

      expect(result.current.data).toBeNull();
      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBeDefined();
      expect((result.current.error as any)?._tag).toBe('FileSystemError');
    });
  });
});
