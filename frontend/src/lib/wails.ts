import { Events as RuntimeEvents, Clipboard as RuntimeClipboard } from "@wailsio/runtime";
import * as App from "../../bindings/github.com/hasdev/forge-ade/app.js";

// Wails v3 bridge. The generated bindings module (frontend/bindings) exposes
// one named function per App method, each returning a promise. Outside the
// Wails webview (plain vite dev in a browser) `window._wails` is absent, so
// getApp() returns an empty object and every optional-chained call falls back
// to its default — the same graceful degradation as before.

const getApp = (): any => {
  return typeof (window as any)?._wails !== "undefined" ? App : {};
};

const call = (fnName: string, fallback: any, ...args: any[]): Promise<any> => {
  const fn = getApp()[fnName];
  if (typeof fn !== "function") return Promise.resolve(fallback);
  return Promise.resolve(fn(...args));
};

export const ClipboardGetText = (): Promise<string> => {
  try {
    return RuntimeClipboard.Text();
  } catch {
    return Promise.resolve("");
  }
};

export const EventsOn = (eventName: string, callback: (data: any) => void): (() => void) => {
  return RuntimeEvents.On(eventName, (ev: any) => callback(ev?.data ?? ev));
};

// ── Workspace ───────────────────────────────────────────────────────────────
export const GetRecentProjects = (): Promise<any[]> => call("GetRecentProjects", []);
export const OpenFolder = (path: string): Promise<any> => call("OpenFolder", {}, path);
export const OpenWorkspace = (path: string): Promise<any> => call("OpenWorkspace", {}, path);
export const AddFolderToWorkspace = (path: string): Promise<any> => call("AddFolderToWorkspace", undefined, path);
export const CloseWorkspace = (): Promise<void> => call("CloseWorkspace", undefined);
export const GetCurrentWorkspace = (): Promise<any> => call("GetCurrentWorkspace", null);
export const SaveWorkspace = (): Promise<void> => call("SaveWorkspace", undefined);
export const SaveWorkspaceAs = (path: string): Promise<void> => call("SaveWorkspaceAs", undefined, path);
export const SaveWorkspaceDialog = (): Promise<string> => call("SaveWorkspaceDialog", "");
export const PinRecent = (path: string, pinned: boolean): Promise<void> => call("PinRecent", undefined, path, pinned);
export const RemoveRecent = (path: string): Promise<void> => call("RemoveRecent", undefined, path);
export const OpenFolderDialog = (): Promise<string> => call("OpenFolderDialog", "");
export const OpenWorkspaceDialog = (): Promise<string> => call("OpenWorkspaceDialog", "");
export const OpenFileDialog = (): Promise<string> => call("OpenFileDialog", "");
export const OpenNewWindow = (url: string): Promise<void> => call("OpenNewWindow", undefined, url);
export const GetHomeDir = (): Promise<string> => call("GetHomeDir", "");
export const OpenInFinder = (path: string): Promise<void> => call("OpenInFinder", undefined, path);
export const BrowserOpenURL = (url: string): Promise<void> => call("BrowserOpenURL", undefined, url);
export const IsDir = (path: string): Promise<boolean> => call("IsDir", false, path);
export const ResolvePath = (path: string): Promise<string> => call("ResolvePath", path, path);

// ── Sessions (shells + external AI CLIs) ────────────────────────────────────
export const ListSessions = (): Promise<any[]> => call("ListSessions", []);
export const ListShells = (): Promise<any[]> => call("ListShells", []);
export const ListAIAgents = (): Promise<any[]> => call("ListAIAgents", []);
export const CreateAIAgent = (name: string, provider: string, folder: string): Promise<any> =>
  call("CreateAIAgent", {}, name, provider, folder);
export const StopSession = (id: string): Promise<void> => call("StopSession", undefined, id);
export const RenameSession = (id: string, name: string): Promise<void> => call("RenameSession", undefined, id, name);
export const RenameAgentSession = (id: string, name: string): Promise<void> =>
  call("UpdateAgentSession", undefined, id, name, "", "", "");
export const CreateShell = (name: string, cwd: string): Promise<any> => call("CreateShell", {}, name, cwd);
export const WriteSession = (id: string, data: string): Promise<void> => call("WriteSession", undefined, id, data);
export const ResizeSession = (id: string, rows: number, cols: number): Promise<void> =>
  call("ResizeSession", undefined, id, rows, cols);

// ── Files ───────────────────────────────────────────────────────────────────
export const ReadFile = (path: string): Promise<string> => call("ReadFile", "", path);
export const ReadFileBase64 = (path: string): Promise<string> => call("ReadFileBase64", "", path);
export const CheckSyntax = (path: string, content: string): Promise<any[]> => call("CheckSyntax", [], path, content);
export const FormatCode = (path: string, content: string): Promise<string> =>
  call("FormatCode", content, path, content);
export const WriteFile = (path: string, content: string): Promise<void> =>
  call("WriteFile", undefined, path, content);
export const CreateFile = (path: string): Promise<void> => call("CreateFile", undefined, path);
export const CreateFolder = (path: string): Promise<void> => call("CreateFolder", undefined, path);
export const DeleteFile = (path: string): Promise<void> => call("DeleteFile", undefined, path);
export const RenameFile = (oldPath: string, newPath: string): Promise<void> =>
  call("RenameFile", undefined, oldPath, newPath);
export const CopyFile = (src: string, dst: string): Promise<void> => call("CopyFile", undefined, src, dst);
export const CopyPath = (src: string, dst: string): Promise<void> => call("CopyPath", undefined, src, dst);
export const GetClipboardFiles = (): Promise<string[]> => call("GetClipboardFiles", []);
export const MoveFile = (src: string, dst: string): Promise<void> => call("MoveFile", undefined, src, dst);
export const GetFileTree = (depth: number): Promise<any> => call("GetFileTree", "[]", depth);
export const ListDirectory = (dirPath: string): Promise<any> => call("ListDirectory", "[]", dirPath);
export const ExpandPath = (targetPath: string): Promise<any> => call("ExpandPath", "[]", targetPath);
export const ToggleHiddenFiles = (): Promise<boolean> => call("ToggleHiddenFiles", true);

// ── Git ─────────────────────────────────────────────────────────────────────
export const GetGitStatus = (repoPath: string): Promise<any> => call("GetGitStatus", null, repoPath);
export const GetGitCommitGraph = (repoPath: string, offset: number, limit: number, branch: string): Promise<any> =>
  call("GetGitCommitGraph", null, repoPath, offset, limit, branch);
export const GetGitBranches = (repoPath: string): Promise<string[]> => call("GetGitBranches", [], repoPath);
export const GetGitCommitDiff = (repoPath: string, hash: string): Promise<string> =>
  call("GetGitCommitDiff", "", repoPath, hash);
export const GetGitCommitBody = (repoPath: string, hash: string): Promise<string> =>
  call("GetGitCommitBody", "", repoPath, hash);
export const GetGitFileDiff = (repoPath: string, path: string): Promise<string> =>
  call("GetGitFileDiff", "", repoPath, path);
export const GetGitCommitFileDiff = (repoPath: string, hash: string, path: string): Promise<string> =>
  call("GetGitCommitFileDiff", "", repoPath, hash, path);
export const GetGitFileContentAtCommit = (repoPath: string, hash: string, path: string): Promise<string> =>
  call("GetGitFileContentAtCommit", "", repoPath, hash, path);
export const GitStage = (repoPath: string, paths: string[]): Promise<void> =>
  call("GitStage", undefined, repoPath, paths);
export const GitUnstage = (repoPath: string, paths: string[]): Promise<void> =>
  call("GitUnstage", undefined, repoPath, paths);
export const GitDiscard = (repoPath: string, paths: string[]): Promise<void> =>
  call("GitDiscard", undefined, repoPath, paths);
export const GetGitFileDiffHunks = (repoPath: string, path: string): Promise<any[]> =>
  call("GetGitFileDiffHunks", [], repoPath, path);
export const RevertGitHunk = (repoPath: string, path: string, hunkIndex: number): Promise<void> =>
  call("RevertGitHunk", undefined, repoPath, path, hunkIndex);
export const GetGitConflictStageContent = (repoPath: string, path: string, stage: number): Promise<string> =>
  call("GetGitConflictStageContent", "", repoPath, path, stage);
export const GitResolveConflict = (repoPath: string, path: string, action: string): Promise<void> =>
  call("GitResolveConflict", undefined, repoPath, path, action);
export const GitCommit = (repoPath: string, message: string): Promise<void> =>
  call("GitCommit", undefined, repoPath, message);
export const GitPush = (repoPath: string): Promise<void> => call("GitPush", undefined, repoPath);
export const GitFetch = (repoPath: string): Promise<string> => call("GitFetch", "", repoPath);
export const GitMerge = (repoPath: string, source: string, noFF: boolean, squash: boolean): Promise<string> =>
  call("GitMerge", "", repoPath, source, noFF, squash);
export const GenerateAICommitMessage = (repoPath: string, providerId: string, model: string, instruction?: string): Promise<string> =>
  call("GenerateAICommitMessage", "", repoPath, providerId, model, instruction || "");

// ── Agent ───────────────────────────────────────────────────────────────────
export const ListAgentSessions = (): Promise<any[]> => call("ListAgentSessions", []);
export const ListAgentSessionsForFolder = (folder: string): Promise<any[]> =>
  call("ListAgentSessionsForFolder", [], folder);
export const GetAgentSession = (id: string): Promise<any> => call("GetAgentSession", null, id);
export const CreateAgentSession = (name: string, role: string, projectFolder: string): Promise<any> =>
  call("CreateAgentSession", {}, name, role, projectFolder);
export const CreateAgentSessionFromDefinition = (defId: string, projectFolder: string): Promise<any> =>
  call("CreateAgentSessionFromDefinition", {}, defId, projectFolder);
export const SendAgentMessage = (id: string, message: string, files: string[]): Promise<void> =>
  call("SendAgentMessage", undefined, id, message, files);
export const RespondAgentApproval = (id: string, approve: boolean, autoAll: boolean): Promise<void> =>
  call("RespondAgentApproval", undefined, id, approve, autoAll);
export const RespondAgentAsk = (id: string, answers: any): Promise<void> =>
  call("RespondAgentAsk", undefined, id, answers);
export const SetAgentAutoApprove = (id: string, enabled: boolean): Promise<void> =>
  call("SetAgentAutoApprove", undefined, id, enabled);
export const ApplyAgentDefinitionToSession = (id: string, defId: string): Promise<void> =>
  call("ApplyAgentDefinitionToSession", undefined, id, defId);
export const StopAgentTurn = (id: string): Promise<void> => call("StopAgentTurn", undefined, id);
export const SetAgentDialect = (id: string, dialect: string): Promise<void> =>
  call("SetAgentDialect", undefined, id, dialect);
export const ToggleAgentTask = (id: string, taskId: string, active: boolean): Promise<void> =>
  call("ToggleAgentTask", undefined, id, taskId, active);
export const DeleteAgentSession = (id: string): Promise<void> => call("DeleteAgentSession", undefined, id);
export const ListAgentDefinitions = (): Promise<any[]> => call("ListAgentDefinitions", []);
export const SaveAgentDefinition = (def: any): Promise<any> => call("SaveAgentDefinition", def, def);
export const DeleteAgentDefinition = (id: string): Promise<void> => call("DeleteAgentDefinition", undefined, id);

// ── Search / Index ──────────────────────────────────────────────────────────
export const SearchFilename = (query: string, limit: number): Promise<any[]> =>
  call("SearchFilename", [], query, limit);
export const SearchFilenameWithOptions = (opts: any): Promise<any[]> => call("SearchFilenameWithOptions", [], opts);
export const SearchContent = (query: string, limit: number): Promise<any[]> => call("SearchContent", [], query, limit);
export const SearchContentWithOptions = (opts: any): Promise<any[]> => call("SearchContentWithOptions", [], opts);
export const SearchSymbols = (query: string, limit: number): Promise<any[]> =>
  call("SearchSymbols", [], query, limit);
export const SearchSymbolsWithOptions = (opts: any): Promise<any[]> => call("SearchSymbolsWithOptions", [], opts);
export const SearchIndexSymbols = (query: string): Promise<any[]> => call("SearchIndexSymbols", [], query);
export const SearchReplaceAll = (opts: any): Promise<any> =>
  call("SearchReplaceAll", { filesChanged: 0, totalReplacements: 0, files: [] }, opts);
export const GetCompletion = (prefix: string, path: string): Promise<any> => call("GetCompletion", null, prefix, path);
export const GetMembers = (instance: string, path: string): Promise<any> => call("GetMembers", null, instance, path);
export const GetOutline = (file: string): Promise<any> => call("GetOutline", null, file);
export const GetSymbols = (): Promise<any[]> => call("GetSymbols", []);
export const FindSymbol = (name: string): Promise<any> => call("FindSymbol", null, name);
export const GetImports = (file: string): Promise<any> => call("GetImports", null, file);
export const GetExports = (file: string): Promise<any> => call("GetExports", null, file);
export const IndexStatus = (): Promise<any> => call("IndexStatus", null);

// ── LLM / Providers ─────────────────────────────────────────────────────────
export const GetProviderProfiles = (): Promise<any[]> => call("GetProviderProfiles", []);
export const SaveProviderProfiles = (profiles: any[]): Promise<void> =>
  call("SaveProviderProfiles", undefined, profiles);
export const FetchProviderModels = (apiKey: string, baseURL: string): Promise<string[]> =>
  call("FetchProviderModels", [], apiKey, baseURL);
export const SetActiveModel = (providerId: string, model: string): Promise<void> =>
  call("SetActiveModel", undefined, providerId, model);
export const SaveLLMProfile = (providerId: string, apiKey: string, baseURL: string, model: string): Promise<void> =>
  call("SaveLLMProfile", undefined, providerId, apiKey, baseURL, model);
export const GetLLMConfig = (): Promise<any> => call("GetLLMConfig", null);
export const ListLLMProviders = (): Promise<any[]> => call("ListLLMProviders", []);

// ── MCP ─────────────────────────────────────────────────────────────────────
export const ListMCPServers = (): Promise<any[]> => call("ListMCPServers", []);
export const SaveMCPServer = (server: any): Promise<any> => call("SaveMCPServer", server, server);
export const DeleteMCPServer = (name: string): Promise<void> => call("DeleteMCPServer", undefined, name);
export const ListMCPTools = (): Promise<any[]> => call("ListMCPTools", []);
export const ListConnectedMCPTools = (): Promise<any[]> => call("ListConnectedMCPTools", []);
export const ReconnectMCP = (): Promise<void> => call("ReconnectMCP", undefined);

// ── Skills ──────────────────────────────────────────────────────────────────
export const ListSkills = (): Promise<any[]> => call("ListSkills", []);
