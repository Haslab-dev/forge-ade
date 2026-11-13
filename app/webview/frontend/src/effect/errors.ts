import { Data } from 'effect';

export class WailsBridgeError extends Data.TaggedError('WailsBridgeError')<{
  readonly method: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class FileSystemError extends Data.TaggedError('FileSystemError')<{
  readonly operation: string;
  readonly path?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class GitError extends Data.TaggedError('GitError')<{
  readonly operation: string;
  readonly repoPath?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class AgentSessionError extends Data.TaggedError('AgentSessionError')<{
  readonly operation: string;
  readonly sessionId?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class DatabaseError extends Data.TaggedError('DatabaseError')<{
  readonly operation: string;
  readonly dbPath?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class SSHRemoteError extends Data.TaggedError('SSHRemoteError')<{
  readonly operation: string;
  readonly connectionId?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class AutomationError extends Data.TaggedError('AutomationError')<{
  readonly operation: string;
  readonly automationId?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class TerminalError extends Data.TaggedError('TerminalError')<{
  readonly operation: string;
  readonly sessionId?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class IndexingError extends Data.TaggedError('IndexingError')<{
  readonly operation: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export class PluginSkillError extends Data.TaggedError('PluginSkillError')<{
  readonly operation: string;
  readonly id?: string;
  readonly message: string;
  readonly cause?: unknown;
}> {}

export type ForgeAppError =
  | WailsBridgeError
  | FileSystemError
  | GitError
  | AgentSessionError
  | DatabaseError
  | SSHRemoteError
  | AutomationError
  | TerminalError
  | IndexingError
  | PluginSkillError;
