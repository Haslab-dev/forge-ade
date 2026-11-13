import { Context, Effect } from 'effect';
import {
  FileSystemError,
  GitError,
  AgentSessionError,
  DatabaseError,
  AutomationError,
  SSHRemoteError,
} from './errors';
import * as Wails from '../lib/wails';
import type { FileItem } from '../types';
import type { DatabaseInfo, QueryResult } from '../../bindings/github.com/hasdev/forge-ade/internal/dbviewer/models';

// ============================================================================
// FileSystemService
// ============================================================================
export interface FileSystemServiceShape {
  readonly openFolder: (path: string) => Effect.Effect<any, FileSystemError>;
  readonly openWorkspace: (path: string) => Effect.Effect<any, FileSystemError>;
  readonly readFile: (path: string) => Effect.Effect<string, FileSystemError>;
  readonly readFileBase64: (path: string) => Effect.Effect<string, FileSystemError>;
  readonly writeFile: (path: string, content: string) => Effect.Effect<void, FileSystemError>;
  readonly writeFileBase64: (path: string, base64Data: string) => Effect.Effect<void, FileSystemError>;
  readonly createFile: (path: string) => Effect.Effect<void, FileSystemError>;
  readonly createFolder: (path: string) => Effect.Effect<void, FileSystemError>;
  readonly deleteFile: (path: string) => Effect.Effect<void, FileSystemError>;
  readonly renameFile: (oldPath: string, newPath: string) => Effect.Effect<void, FileSystemError>;
  readonly moveFile: (sourcePath: string, targetPath: string) => Effect.Effect<void, FileSystemError>;
  readonly getFileTree: (depth?: number) => Effect.Effect<FileItem[], FileSystemError>;
  readonly listDirectory: (path: string) => Effect.Effect<FileItem[], FileSystemError>;
  readonly expandPath: (path: string) => Effect.Effect<string, FileSystemError>;
  readonly isDir: (path: string) => Effect.Effect<boolean, FileSystemError>;
  readonly copyPath: (src: string, dst: string) => Effect.Effect<void, FileSystemError>;
  readonly openInFinder: (path: string) => Effect.Effect<void, FileSystemError>;
  readonly getFileSize: (path: string) => Effect.Effect<number, FileSystemError>;
}

export const FileSystemService = Context.Reference<FileSystemServiceShape>('@forge/FileSystemService', {
  defaultValue: () => ({
    openFolder: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.OpenFolder(path),
        catch: (err) => new FileSystemError({ operation: 'openFolder', path, message: String(err), cause: err }),
      }),
    openWorkspace: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.OpenWorkspace(path),
        catch: (err) => new FileSystemError({ operation: 'openWorkspace', path, message: String(err), cause: err }),
      }),
    readFile: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.ReadFile(path),
        catch: (err) => new FileSystemError({ operation: 'readFile', path, message: String(err), cause: err }),
      }),
    readFileBase64: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.ReadFileBase64(path),
        catch: (err) => new FileSystemError({ operation: 'readFileBase64', path, message: String(err), cause: err }),
      }),
    writeFile: (path: string, content: string) =>
      Effect.tryPromise({
        try: () => Wails.WriteFile(path, content),
        catch: (err) => new FileSystemError({ operation: 'writeFile', path, message: String(err), cause: err }),
      }),
    writeFileBase64: (path: string, base64Data: string) =>
      Effect.tryPromise({
        try: () => Wails.WriteFileBase64(path, base64Data),
        catch: (err) => new FileSystemError({ operation: 'writeFileBase64', path, message: String(err), cause: err }),
      }),
    createFile: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.CreateFile(path),
        catch: (err) => new FileSystemError({ operation: 'createFile', path, message: String(err), cause: err }),
      }),
    createFolder: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.CreateFolder(path),
        catch: (err) => new FileSystemError({ operation: 'createFolder', path, message: String(err), cause: err }),
      }),
    deleteFile: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.DeleteFile(path),
        catch: (err) => new FileSystemError({ operation: 'deleteFile', path, message: String(err), cause: err }),
      }),
    renameFile: (oldPath: string, newPath: string) =>
      Effect.tryPromise({
        try: () => Wails.RenameFile(oldPath, newPath),
        catch: (err) => new FileSystemError({ operation: 'renameFile', path: oldPath, message: String(err), cause: err }),
      }),
    moveFile: (sourcePath: string, targetPath: string) =>
      Effect.tryPromise({
        try: () => Wails.MoveFile(sourcePath, targetPath),
        catch: (err) => new FileSystemError({ operation: 'moveFile', path: sourcePath, message: String(err), cause: err }),
      }),
    getFileTree: (depth = -1) =>
      Effect.tryPromise({
        try: () => Wails.GetFileTree(depth) as Promise<FileItem[]>,
        catch: (err) => new FileSystemError({ operation: 'getFileTree', message: String(err), cause: err }),
      }),
    listDirectory: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.ListDirectory(path) as Promise<FileItem[]>,
        catch: (err) => new FileSystemError({ operation: 'listDirectory', path, message: String(err), cause: err }),
      }),
    expandPath: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.ExpandPath(path),
        catch: (err) => new FileSystemError({ operation: 'expandPath', path, message: String(err), cause: err }),
      }),
    isDir: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.IsDir(path),
        catch: (err) => new FileSystemError({ operation: 'isDir', path, message: String(err), cause: err }),
      }),
    copyPath: (src: string, dst: string) =>
      Effect.tryPromise({
        try: () => Wails.CopyPath(src, dst),
        catch: (err) => new FileSystemError({ operation: 'copyPath', path: src, message: String(err), cause: err }),
      }),
    openInFinder: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.OpenInFinder(path),
        catch: (err) => new FileSystemError({ operation: 'openInFinder', path, message: String(err), cause: err }),
      }),
    getFileSize: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.GetFileSize(path),
        catch: (err) => new FileSystemError({ operation: 'getFileSize', path, message: String(err), cause: err }),
      }),
  }),
});

// ============================================================================
// GitService
// ============================================================================
export interface GitServiceShape {
  readonly getStatus: (path: string) => Effect.Effect<any, GitError>;
  readonly getBranches: (path: string) => Effect.Effect<string[], GitError>;
  readonly checkout: (path: string, branch: string) => Effect.Effect<void, GitError>;
  readonly checkoutNewBranch: (path: string, branch: string) => Effect.Effect<void, GitError>;
  readonly stage: (path: string, files: string[]) => Effect.Effect<void, GitError>;
  readonly unstage: (path: string, files: string[]) => Effect.Effect<void, GitError>;
  readonly discard: (path: string, files: string[]) => Effect.Effect<void, GitError>;
  readonly commit: (path: string, message: string) => Effect.Effect<void, GitError>;
  readonly push: (path: string) => Effect.Effect<void, GitError>;
  readonly pull: (path: string) => Effect.Effect<void, GitError>;
  readonly fetch: (path: string) => Effect.Effect<void, GitError>;
  readonly getFileDiff: (repoPath: string, filePath: string) => Effect.Effect<string, GitError>;
  readonly getCommitGraph: (repoPath: string, offset?: number, limit?: number, branch?: string) => Effect.Effect<any[], GitError>;
  readonly generateAICommitMessage: (repoPath: string, providerId: string, model: string, instruction?: string) => Effect.Effect<string, GitError>;
}

export const GitService = Context.Reference<GitServiceShape>('@forge/GitService', {
  defaultValue: () => ({
    getStatus: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.GetGitStatus(path),
        catch: (err) => new GitError({ operation: 'getStatus', repoPath: path, message: String(err), cause: err }),
      }),
    getBranches: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.GetGitBranches(path),
        catch: (err) => new GitError({ operation: 'getBranches', repoPath: path, message: String(err), cause: err }),
      }),
    checkout: (path: string, branch: string) =>
      Effect.tryPromise({
        try: () => Wails.GitCheckout(path, branch),
        catch: (err) => new GitError({ operation: 'checkout', repoPath: path, message: String(err), cause: err }),
      }),
    checkoutNewBranch: (path: string, branch: string) =>
      Effect.tryPromise({
        try: () => Wails.GitCheckoutNewBranch(path, branch),
        catch: (err) => new GitError({ operation: 'checkoutNewBranch', repoPath: path, message: String(err), cause: err }),
      }),
    stage: (path: string, files: string[]) =>
      Effect.tryPromise({
        try: () => Wails.GitStage(path, files),
        catch: (err) => new GitError({ operation: 'stage', repoPath: path, message: String(err), cause: err }),
      }),
    unstage: (path: string, files: string[]) =>
      Effect.tryPromise({
        try: () => Wails.GitUnstage(path, files),
        catch: (err) => new GitError({ operation: 'unstage', repoPath: path, message: String(err), cause: err }),
      }),
    discard: (path: string, files: string[]) =>
      Effect.tryPromise({
        try: () => Wails.GitDiscard(path, files),
        catch: (err) => new GitError({ operation: 'discard', repoPath: path, message: String(err), cause: err }),
      }),
    commit: (path: string, message: string) =>
      Effect.tryPromise({
        try: () => Wails.GitCommit(path, message),
        catch: (err) => new GitError({ operation: 'commit', repoPath: path, message: String(err), cause: err }),
      }),
    push: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.GitPush(path),
        catch: (err) => new GitError({ operation: 'push', repoPath: path, message: String(err), cause: err }),
      }),
    pull: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.GitPull(path),
        catch: (err) => new GitError({ operation: 'pull', repoPath: path, message: String(err), cause: err }),
      }),
    fetch: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.GitFetch(path),
        catch: (err) => new GitError({ operation: 'fetch', repoPath: path, message: String(err), cause: err }),
      }),
    getFileDiff: (repoPath: string, filePath: string) =>
      Effect.tryPromise({
        try: () => Wails.GetGitFileDiff(repoPath, filePath),
        catch: (err) => new GitError({ operation: 'getFileDiff', repoPath, message: String(err), cause: err }),
      }),
    getCommitGraph: (repoPath: string, offset = 0, limit = 50, branch = 'HEAD') =>
      Effect.tryPromise({
        try: () => Wails.GetGitCommitGraph(repoPath, offset, limit, branch),
        catch: (err) => new GitError({ operation: 'getCommitGraph', repoPath, message: String(err), cause: err }),
      }),
    generateAICommitMessage: (repoPath: string, providerId: string, model: string, instruction?: string) =>
      Effect.tryPromise({
        try: () => Wails.GenerateAICommitMessage(repoPath, providerId, model, instruction),
        catch: (err) => new GitError({ operation: 'generateAICommitMessage', repoPath, message: String(err), cause: err }),
      }),
  }),
});

// ============================================================================
// AgentSessionService
// ============================================================================
export interface AgentSessionServiceShape {
  readonly saveSessionsDisk: (sessionJson: string, workspacePath: string) => Effect.Effect<void, AgentSessionError>;
  readonly loadSessionsDisk: (workspacePath: string) => Effect.Effect<string[], AgentSessionError>;
  readonly deleteSessionDisk: (sessionId: string, workspacePath: string) => Effect.Effect<void, AgentSessionError>;
  readonly fetchProviderModels: (apiKey: string, baseURL: string) => Effect.Effect<string[], AgentSessionError>;
}

export const AgentSessionService = Context.Reference<AgentSessionServiceShape>('@forge/AgentSessionService', {
  defaultValue: () => ({
    saveSessionsDisk: (sessionJson: string, workspacePath: string) =>
      Effect.tryPromise({
        try: () => Wails.SaveAgentSessionDisk(sessionJson, workspacePath),
        catch: (err) => new AgentSessionError({ operation: 'saveSessionsDisk', message: String(err), cause: err }),
      }),
    loadSessionsDisk: (workspacePath: string) =>
      Effect.tryPromise({
        try: () => Wails.LoadAgentSessionsDisk(workspacePath),
        catch: (err) => new AgentSessionError({ operation: 'loadSessionsDisk', message: String(err), cause: err }),
      }),
    deleteSessionDisk: (sessionId: string, workspacePath: string) =>
      Effect.tryPromise({
        try: () => Wails.DeleteAgentSessionDisk(sessionId, workspacePath),
        catch: (err) => new AgentSessionError({ operation: 'deleteSessionDisk', sessionId, message: String(err), cause: err }),
      }),
    fetchProviderModels: (apiKey: string, baseURL: string) =>
      Effect.tryPromise({
        try: () => Wails.FetchProviderModels(apiKey, baseURL),
        catch: (err) => new AgentSessionError({ operation: 'fetchProviderModels', message: String(err), cause: err }),
      }),
  }),
});

// ============================================================================
// DatabaseService
// ============================================================================
export interface DatabaseServiceShape {
  readonly inspectDatabase: (path: string) => Effect.Effect<DatabaseInfo, DatabaseError>;
  readonly getTableRows: (
    path: string,
    tableName: string,
    limit: number,
    offset: number,
    sortCol?: string,
    sortAsc?: boolean,
    filterCol?: string,
    filterVal?: string
  ) => Effect.Effect<QueryResult, DatabaseError>;
  readonly executeQuery: (path: string, query: string, limit?: number) => Effect.Effect<QueryResult, DatabaseError>;
}

export const DatabaseService = Context.Reference<DatabaseServiceShape>('@forge/DatabaseService', {
  defaultValue: () => ({
    inspectDatabase: (path: string) =>
      Effect.tryPromise({
        try: () => Wails.InspectDatabase(path) as Promise<DatabaseInfo>,
        catch: (err) => new DatabaseError({ operation: 'inspectDatabase', dbPath: path, message: String(err), cause: err }),
      }),
    getTableRows: (
      path: string,
      tableName: string,
      limit: number,
      offset: number,
      sortCol = '',
      sortAsc = true,
      filterCol = '',
      filterVal = ''
    ) =>
      Effect.tryPromise({
        try: () =>
          Wails.GetDatabaseTableRows(path, tableName, limit, offset, sortCol, sortAsc, filterCol, filterVal) as Promise<QueryResult>,
        catch: (err) => new DatabaseError({ operation: 'getTableRows', dbPath: path, message: String(err), cause: err }),
      }),
    executeQuery: (path: string, query: string, limit = 500) =>
      Effect.tryPromise({
        try: () => Wails.ExecuteDatabaseQuery(path, query, limit) as Promise<QueryResult>,
        catch: (err) => new DatabaseError({ operation: 'executeQuery', dbPath: path, message: String(err), cause: err }),
      }),
  }),
});

// ============================================================================
// AutomationService
// ============================================================================
export interface AutomationServiceShape {
  readonly listAutomations: () => Effect.Effect<Wails.Automation[], AutomationError>;
  readonly saveAutomation: (input: Wails.AutomationSaveInput) => Effect.Effect<Wails.Automation, AutomationError>;
  readonly deleteAutomation: (id: string) => Effect.Effect<void, AutomationError>;
  readonly recordRun: (id: string, sessionId: string, workspace: string) => Effect.Effect<Wails.Automation, AutomationError>;
  readonly setEnabled: (id: string, enabled: boolean) => Effect.Effect<void, AutomationError>;
}

export const AutomationService = Context.Reference<AutomationServiceShape>('@forge/AutomationService', {
  defaultValue: () => ({
    listAutomations: () =>
      Effect.tryPromise({
        try: () => Wails.ListAutomations(),
        catch: (err) => new AutomationError({ operation: 'listAutomations', message: String(err), cause: err }),
      }),
    saveAutomation: (input: Wails.AutomationSaveInput) =>
      Effect.tryPromise({
        try: () => Wails.SaveAutomation(input),
        catch: (err) => new AutomationError({ operation: 'saveAutomation', message: String(err), cause: err }),
      }),
    deleteAutomation: (id: string) =>
      Effect.tryPromise({
        try: () => Wails.DeleteAutomation(id),
        catch: (err) => new AutomationError({ operation: 'deleteAutomation', automationId: id, message: String(err), cause: err }),
      }),
    recordRun: (id: string, sessionId: string, workspace: string) =>
      Effect.tryPromise({
        try: () => Wails.RecordAutomationRun(id, sessionId, workspace),
        catch: (err) => new AutomationError({ operation: 'recordRun', automationId: id, message: String(err), cause: err }),
      }),
    setEnabled: (id: string, enabled: boolean) =>
      Effect.tryPromise({
        try: () => Wails.SetAutomationEnabled(id, enabled),
        catch: (err) => new AutomationError({ operation: 'setEnabled', automationId: id, message: String(err), cause: err }),
      }),
  }),
});

// ============================================================================
// SSHService
// ============================================================================
export interface SSHServiceShape {
  readonly listConnections: () => Effect.Effect<Wails.SSHConnectionStatus[], SSHRemoteError>;
  readonly connect: (config: Wails.SSHConfig) => Effect.Effect<Wails.SSHConnectionStatus | null, SSHRemoteError>;
  readonly disconnect: (id: string) => Effect.Effect<void, SSHRemoteError>;
  readonly uploadFile: (localPath: string, remoteURI: string) => Effect.Effect<void, SSHRemoteError>;
}

export const SSHService = Context.Reference<SSHServiceShape>('@forge/SSHService', {
  defaultValue: () => ({
    listConnections: () =>
      Effect.tryPromise({
        try: () => Wails.ListSSHConnections(),
        catch: (err) => new SSHRemoteError({ operation: 'listConnections', message: String(err), cause: err }),
      }),
    connect: (config: Wails.SSHConfig) =>
      Effect.tryPromise({
        try: () => Wails.ConnectSSH(config),
        catch: (err) => new SSHRemoteError({ operation: 'connect', message: String(err), cause: err }),
      }),
    disconnect: (id: string) =>
      Effect.tryPromise({
        try: () => Wails.DisconnectSSH(id),
        catch: (err) => new SSHRemoteError({ operation: 'disconnect', connectionId: id, message: String(err), cause: err }),
      }),
    uploadFile: (localPath: string, remoteURI: string) =>
      Effect.tryPromise({
        try: () => Wails.UploadFileToRemote(localPath, remoteURI),
        catch: (err) => new SSHRemoteError({ operation: 'uploadFile', message: String(err), cause: err }),
      }),
  }),
});
