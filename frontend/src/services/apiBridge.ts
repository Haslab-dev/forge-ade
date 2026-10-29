import { Effect } from 'effect';
import {
  FileSystemService,
  GitService,
  AgentSessionService,
  DatabaseService,
  AutomationService,
  SSHService,
  FileSystemError,
  GitError,
  AgentSessionError,
  DatabaseError,
  AutomationError,
  SSHRemoteError,
} from '../effect';
import {
  FileItem,
  PluginInfo,
  CreatePluginRequest,
  CreateSkillRequest,
  AgentMemoryEntry,
  IndexingStatusInfo,
  AgentCliRuntimeConfig,
  LSPDiagnostic,
  WorkspaceSymbol,
  BoardTask,
  BoardTaskStatus,
} from '../types';
import {
  BrowserOpenURL as WailsBrowserOpenURL,
  GetProcessRSS as WailsGetProcessRSS,
  type Automation as ZAutomation,
  type AutomationSaveInput as ZAutomationSaveInput,
  type SSHConfig,
  type SSHConnectionStatus,
  GetAgentMCPInstallStatuses,
  InstallForgeMCPToAgent,
  UninstallForgeMCPFromAgent,
  InstallForgeMCPToAllAgents,
} from '../lib/wails';
import type { DatabaseInfo, QueryResult } from '../../bindings/github.com/hasdev/forge-ade/internal/dbviewer/models';

import {
  WorkspaceApi,
  type WorkspaceInfo,
  FilesystemApi,
  mapFileInfoToFileItem,
  GitApi,
  TerminalApi,
  type CommandExecutionResult,
  AgentApi,
  normalizeSymbol,
  IntegrationsApi,
  RemoteApi,
  DatabaseApi,
  AutomationsApi,
  BoardApi,
} from '../features';

export { mapFileInfoToFileItem, normalizeSymbol, type WorkspaceInfo, type CommandExecutionResult };

/**
 * Effect-TS first API bridge exposing pure, typed, composable Effect programs.
 * Can be yielded directly inside `Effect.gen` pipelines across the frontend.
 */
export class ApiBridgeEffect {
  public static openFolder(dirPath: string): Effect.Effect<any, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.openFolder(dirPath);
    });
  }

  public static openWorkspace(workspacePath: string): Effect.Effect<any, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.openWorkspace(workspacePath);
    });
  }

  public static readFile(filePath: string): Effect.Effect<string, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.readFile(filePath);
    });
  }

  public static readFileBase64(filePath: string): Effect.Effect<string, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.readFileBase64(filePath);
    });
  }

  public static writeFile(filePath: string, content: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.writeFile(filePath, content);
    });
  }

  public static writeFileBase64(filePath: string, base64Data: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.writeFileBase64(filePath, base64Data);
    });
  }

  public static createFile(filePath: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.createFile(filePath);
    });
  }

  public static createFolder(folderPath: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.createFolder(folderPath);
    });
  }

  public static deleteFile(filePath: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.deleteFile(filePath);
    });
  }

  public static renameFile(oldPath: string, newPath: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.renameFile(oldPath, newPath);
    });
  }

  public static moveFile(sourcePath: string, targetPath: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.moveFile(sourcePath, targetPath);
    });
  }

  public static copyPath(src: string, dst: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.copyPath(src, dst);
    });
  }

  public static openInFinder(path: string): Effect.Effect<void, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.openInFinder(path);
    });
  }

  public static fileSize(path: string): Effect.Effect<number, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.getFileSize(path);
    });
  }

  public static isDir(path: string): Effect.Effect<boolean, FileSystemError> {
    return Effect.gen(function* () {
      const fs = yield* FileSystemService;
      return yield* fs.isDir(path);
    });
  }

  public static getGitStatus(repoPath: string): Effect.Effect<any, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.getStatus(repoPath);
    });
  }

  public static getGitBranches(repoPath: string): Effect.Effect<string[], GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.getBranches(repoPath);
    });
  }

  public static gitCheckout(repoPath: string, branch: string): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.checkout(repoPath, branch);
    });
  }

  public static gitCheckoutNewBranch(repoPath: string, branch: string): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.checkoutNewBranch(repoPath, branch);
    });
  }

  public static gitStage(repoPath: string, files: string[]): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.stage(repoPath, files);
    });
  }

  public static gitUnstage(repoPath: string, files: string[]): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.unstage(repoPath, files);
    });
  }

  public static gitDiscard(repoPath: string, files: string[]): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.discard(repoPath, files);
    });
  }

  public static gitCommit(repoPath: string, message: string): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.commit(repoPath, message);
    });
  }

  public static gitPush(repoPath: string): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.push(repoPath);
    });
  }

  public static gitPull(repoPath: string): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.pull(repoPath);
    });
  }

  public static gitFetch(repoPath: string): Effect.Effect<void, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.fetch(repoPath);
    });
  }

  public static getGitFileDiff(repoPath: string, filePath: string): Effect.Effect<string, GitError> {
    return Effect.gen(function* () {
      const git = yield* GitService;
      return yield* git.getFileDiff(repoPath, filePath);
    });
  }

  public static inspectDatabase(path: string): Effect.Effect<DatabaseInfo, DatabaseError> {
    return Effect.gen(function* () {
      const db = yield* DatabaseService;
      return yield* db.inspectDatabase(path);
    });
  }

  public static getDatabaseTableRows(
    path: string,
    tableName: string,
    limit: number,
    offset: number,
    sortCol?: string,
    sortAsc?: boolean,
    filterCol?: string,
    filterVal?: string
  ): Effect.Effect<QueryResult, DatabaseError> {
    return Effect.gen(function* () {
      const db = yield* DatabaseService;
      return yield* db.getTableRows(path, tableName, limit, offset, sortCol, sortAsc, filterCol, filterVal);
    });
  }

  public static executeDatabaseQuery(path: string, query: string, limit = 500): Effect.Effect<QueryResult, DatabaseError> {
    return Effect.gen(function* () {
      const db = yield* DatabaseService;
      return yield* db.executeQuery(path, query, limit);
    });
  }

  public static listAutomations(): Effect.Effect<ZAutomation[], AutomationError> {
    return Effect.gen(function* () {
      const auto = yield* AutomationService;
      return yield* auto.listAutomations();
    });
  }

  public static saveAutomation(input: ZAutomationSaveInput): Effect.Effect<ZAutomation, AutomationError> {
    return Effect.gen(function* () {
      const auto = yield* AutomationService;
      return yield* auto.saveAutomation(input);
    });
  }

  public static deleteAutomation(id: string): Effect.Effect<void, AutomationError> {
    return Effect.gen(function* () {
      const auto = yield* AutomationService;
      return yield* auto.deleteAutomation(id);
    });
  }

  public static listSSHConnections(): Effect.Effect<SSHConnectionStatus[], SSHRemoteError> {
    return Effect.gen(function* () {
      const ssh = yield* SSHService;
      return yield* ssh.listConnections();
    });
  }

  public static connectSSH(config: SSHConfig): Effect.Effect<SSHConnectionStatus | null, SSHRemoteError> {
    return Effect.gen(function* () {
      const ssh = yield* SSHService;
      return yield* ssh.connect(config);
    });
  }

  public static disconnectSSH(id: string): Effect.Effect<void, SSHRemoteError> {
    return Effect.gen(function* () {
      const ssh = yield* SSHService;
      return yield* ssh.disconnect(id);
    });
  }

  public static uploadFileToRemote(localPath: string, remoteURI: string): Effect.Effect<void, SSHRemoteError> {
    return Effect.gen(function* () {
      const ssh = yield* SSHService;
      return yield* ssh.uploadFile(localPath, remoteURI);
    });
  }
}

/**
 * Unified facade class maintaining full backward compatibility
 * while delegating implementation cleanly to domain feature modules.
 */
export class ApiBridge {
  // ── Terminal & Config ──────────────────────────────────────────────────────
  public static ensureAgentCliPath = TerminalApi.ensureAgentCliPath;
  public static formatConfigContent = TerminalApi.formatConfigContent;
  public static checkSyntax = TerminalApi.checkSyntax;
  public static listAgentCliRuntimeConfigs = TerminalApi.listAgentCliRuntimeConfigs;
  public static openInTerminal = TerminalApi.openInTerminal;
  public static executeCommand = TerminalApi.executeCommand;
  public static createShell = TerminalApi.createShell;
  public static writeSession = TerminalApi.writeSession;
  public static resizeSession = TerminalApi.resizeSession;
  public static stopSession = TerminalApi.stopSession;
  public static listSessions = TerminalApi.listSessions;
  public static getAgentMCPInstallStatuses = GetAgentMCPInstallStatuses;
  public static installForgeMCPToAgent = InstallForgeMCPToAgent;
  public static uninstallForgeMCPFromAgent = UninstallForgeMCPFromAgent;
  public static installForgeMCPToAllAgents = InstallForgeMCPToAllAgents;

  // ── Filesystem ─────────────────────────────────────────────────────────────
  public static isDir = FilesystemApi.isDir;
  public static fileSize = FilesystemApi.fileSize;
  public static readDirectoryTree = FilesystemApi.readDirectoryTree;
  public static listDirectory = FilesystemApi.listDirectory;
  public static readFile = FilesystemApi.readFile;
  public static readFileBase64 = FilesystemApi.readFileBase64;
  public static writeFile = FilesystemApi.writeFile;
  public static writeFileBase64 = FilesystemApi.writeFileBase64;
  public static saveAttachment = FilesystemApi.saveAttachment;
  public static createFile = FilesystemApi.createFile;
  public static createFolder = FilesystemApi.createFolder;
  public static deleteFile = FilesystemApi.deleteFile;
  public static renameFile = FilesystemApi.renameFile;
  public static moveFile = FilesystemApi.moveFile;
  public static copyPath = FilesystemApi.copyPath;
  public static openInFinder = FilesystemApi.openInFinder;
  public static expandPath = FilesystemApi.expandPath;
  public static getSandboxDir = FilesystemApi.getSandboxDir;
  public static clearSandbox = FilesystemApi.clearSandbox;

  // ── Workspace ──────────────────────────────────────────────────────────────
  public static getWorkspaceInfo = WorkspaceApi.getWorkspaceInfo;
  public static openFolder = WorkspaceApi.openFolder;
  public static openWorkspace = WorkspaceApi.openWorkspace;
  public static closeWorkspace = WorkspaceApi.closeWorkspace;
  public static getRecentProjects = WorkspaceApi.getRecentProjects;
  public static removeRecent = WorkspaceApi.removeRecent;
  public static pickNativeDirectory = WorkspaceApi.pickNativeDirectory;
  public static pickNativeFiles = WorkspaceApi.pickNativeFiles;
  public static pickDirectoryInBrowser = WorkspaceApi.pickDirectoryInBrowser;

  // ── Git ────────────────────────────────────────────────────────────────────
  public static gitStatus = GitApi.gitStatus;
  public static getGitBranches = GitApi.getGitBranches;
  public static gitCheckout = GitApi.gitCheckout;
  public static gitCreateBranch = GitApi.gitCreateBranch;
  public static gitDiff = GitApi.gitDiff;
  public static gitCommitFileDiff = GitApi.gitCommitFileDiff;
  public static gitCommitDiff = GitApi.gitCommitDiff;
  public static gitLog = GitApi.gitLog;
  public static gitStage = GitApi.gitStage;
  public static gitUnstage = GitApi.gitUnstage;
  public static gitDiscard = GitApi.gitDiscard;
  public static gitCheckIgnored = GitApi.gitCheckIgnored;
  public static gitCommit = GitApi.gitCommit;
  public static gitAiCommitMessage = GitApi.gitAiCommitMessage;
  public static gitPush = GitApi.gitPush;
  public static gitFetch = GitApi.gitFetch;
  public static gitPull = GitApi.gitPull;
  public static getGitCommitGraph = GitApi.getGitCommitGraph;
  public static getPullRequests = GitApi.getPullRequests;
  public static getPullRequestDetail = GitApi.getPullRequestDetail;
  public static getPullRequestDiff = GitApi.getPullRequestDiff;
  public static createPullRequestComment = GitApi.createPullRequestComment;
  public static reviewPullRequest = GitApi.reviewPullRequest;
  public static mergePullRequest = GitApi.mergePullRequest;
  public static closePullRequest = GitApi.closePullRequest;
  public static reopenPullRequest = GitApi.reopenPullRequest;
  public static checkoutPullRequest = GitApi.checkoutPullRequest;
  public static createPullRequest = GitApi.createPullRequest;

  // ── Agent, Search & Memory ─────────────────────────────────────────────────
  public static saveSessionJsonl = AgentApi.saveSessionJsonl;
  public static loadSessionsJsonl = AgentApi.loadSessionsJsonl;
  public static deleteSessionJsonl = AgentApi.deleteSessionJsonl;
  public static listMemories = AgentApi.listMemories;
  public static saveMemory = AgentApi.saveMemory;
  public static deleteMemory = AgentApi.deleteMemory;
  public static reloadMemories = AgentApi.reloadMemories;
  public static handshakeACP = AgentApi.handshakeACP;
  public static searchContent = AgentApi.searchContent;
  public static searchFilename = AgentApi.searchFilename;
  public static searchReplaceAll = AgentApi.searchReplaceAll;
  public static getIndexStatus = AgentApi.getIndexStatus;
  public static reindexWorkspace = AgentApi.reindexWorkspace;
  public static getFileOutline = AgentApi.getFileOutline;
  public static searchWorkspaceSymbols = AgentApi.searchWorkspaceSymbols;
  public static getComputerUseStatus = AgentApi.getComputerUseStatus;
  public static openComputerUsePermissions = AgentApi.openComputerUsePermissions;
  public static fetchModels = AgentApi.fetchModels;

  // ── Integrations (Plugins & Skills & MCP) ──────────────────────────────────
  public static discoverMcps = IntegrationsApi.discoverMcps;
  public static importDiscoveredMcps = IntegrationsApi.importDiscoveredMcps;
  public static discoverSkills = IntegrationsApi.discoverSkills;
  public static importDiscoveredSkills = IntegrationsApi.importDiscoveredSkills;
  public static listPlugins = IntegrationsApi.listPlugins;
  public static getPlugin = IntegrationsApi.getPlugin;
  public static createPlugin = IntegrationsApi.createPlugin;
  public static togglePlugin = IntegrationsApi.togglePlugin;
  public static deletePlugin = IntegrationsApi.deletePlugin;
  public static reloadPlugins = IntegrationsApi.reloadPlugins;
  public static listSkills = IntegrationsApi.listSkills;
  public static createSkill = IntegrationsApi.createSkill;
  public static deleteSkill = IntegrationsApi.deleteSkill;
  public static reloadSkills = IntegrationsApi.reloadSkills;

  // ── Remote (SSH) ───────────────────────────────────────────────────────────
  public static connectSSH = RemoteApi.connectSSH;
  public static disconnectSSH = RemoteApi.disconnectSSH;
  public static listSSHConnections = RemoteApi.listSSHConnections;
  public static openSSHWorkspace = RemoteApi.openSSHWorkspace;
  public static uploadFileToRemote = RemoteApi.uploadFileToRemote;
  public static uploadFilesToRemote = RemoteApi.uploadFilesToRemote;
  public static openUploadFileDialog = RemoteApi.openUploadFileDialog;
  public static openUploadFolderDialog = RemoteApi.openUploadFolderDialog;

  // ── Database ───────────────────────────────────────────────────────────────
  public static inspectDatabase = DatabaseApi.inspectDatabase;
  public static getDatabaseTableRows = DatabaseApi.getDatabaseTableRows;
  public static executeDatabaseQuery = DatabaseApi.executeDatabaseQuery;

  // ── Automations ────────────────────────────────────────────────────────────
  public static listAutomations = AutomationsApi.listAutomations;
  public static saveAutomation = AutomationsApi.saveAutomation;
  public static deleteAutomation = AutomationsApi.deleteAutomation;
  public static recordAutomationRun = AutomationsApi.recordAutomationRun;
  public static claimAutomationFire = AutomationsApi.claimAutomationFire;
  public static setAutomationEnabled = AutomationsApi.setAutomationEnabled;

  // ── Workflow Board (BoardTask) ─────────────────────────────────────────────
  public static listBoardTasks = BoardApi.listBoardTasks;
  public static getBoardTask = BoardApi.getBoardTask;
  public static saveBoardTask = BoardApi.saveBoardTask;
  public static deleteBoardTask = BoardApi.deleteBoardTask;
  public static moveBoardTaskStatus = BoardApi.moveBoardTaskStatus;

  // Deprecated backward-compatibility aliases
  public static listJiraTasks = BoardApi.listBoardTasks;
  public static getJiraTask = BoardApi.getBoardTask;
  public static saveJiraTask = BoardApi.saveBoardTask;
  public static deleteJiraTask = BoardApi.deleteBoardTask;
  public static moveJiraTaskStatus = BoardApi.moveBoardTaskStatus;

  // ── System Utilities ───────────────────────────────────────────────────────
  public static async processRSS(): Promise<number> {
    try {
      const kb = await WailsGetProcessRSS();
      return typeof kb === 'number' && Number.isFinite(kb) ? kb : -1;
    } catch {
      return -1;
    }
  }

  public static async browserOpenURL(url: string): Promise<void> {
    try {
      await WailsBrowserOpenURL(url);
    } catch {
      // no-op
    }
  }

  public static async getAgentModels(agentId: string): Promise<string[]> {
    try {
      const configs = await TerminalApi.listAgentCliRuntimeConfigs();
      const cleanId = agentId.toLowerCase().replace(/^agent-/, '');
      const target = (configs || []).find((c) => {
        const cId = (c.id || '').toLowerCase();
        const cExe = (c.executable || '').toLowerCase();
        if (cleanId === 'agy' || cleanId === 'antigravity') {
          return cId === 'antigravity' || cId === 'gemini' || cExe === 'agy' || cExe === 'antigravity';
        }
        if (cleanId === 'claude') {
          return cId === 'claude-code' || cId === 'claude' || cExe === 'claude';
        }
        if (cleanId === 'codex') {
          return cId === 'codex' || cExe === 'codex';
        }
        if (cleanId === 'opencode') {
          return cId === 'opencode' || cExe === 'opencode';
        }
        if (cleanId === 'commandcode' || cleanId === 'cmd') {
          return cId === 'commandcode' || cId === 'cmd' || cExe === 'cmd' || cExe === 'commandcode';
        }
        if (cleanId === 'pi' || cleanId === 'pi-acp') {
          return cId === 'pi' || cExe === 'pi' || cExe === 'pi-acp';
        }
        if (cleanId === 'omp' || cleanId === 'ohmypi') {
          return cId === 'ohmypi' || cId === 'omp' || cExe === 'omp';
        }
        return cId === cleanId || cExe === cleanId;
      });

      if (target) {
        const found = new Set<string>();
        if (target.defaultModel) found.add(target.defaultModel);
        if (Array.isArray(target.providers)) {
          for (const p of target.providers) {
            if (Array.isArray(p.models)) {
              for (const m of p.models) {
                if (m) found.add(m);
              }
            }
          }
        }
        if (found.size > 0) {
          return Array.from(found);
        }
      }
    } catch {
      // ignore
    }
    return [];
  }
}
