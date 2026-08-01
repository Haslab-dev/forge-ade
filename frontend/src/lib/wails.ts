import { EventsOn as wailsEventsOn, EventsOff as wailsEventsOff } from "../../wailsjs/runtime/runtime";

// Wails API wrapper for safe invocation
const getApp = (): any => {
  return (window as any)?.go?.main?.App || {};
};

const getRuntime = (): any => {
  return (window as any)?.runtime || {
    EventsOn: () => {},
    EventsOff: () => {},
    ClipboardGetText: async () => "",
  };
};

export const ClipboardGetText = (): Promise<string> => getRuntime().ClipboardGetText();

export const EventsOn = (eventName: string, callback: (data: any) => void): any => {
  wailsEventsOn(eventName, callback);
  return () => wailsEventsOff(eventName);
};

export const GetRecentProjects = (): Promise<any[]> => getApp().GetRecentProjects?.() || Promise.resolve([]);
export const OpenFolder = (path: string): Promise<any> => getApp().OpenFolder?.(path) || Promise.resolve({});
export const OpenWorkspace = (path: string): Promise<any> => getApp().OpenWorkspace?.(path) || Promise.resolve({});
export const CloseWorkspace = (): Promise<void> => getApp().CloseWorkspace?.() || Promise.resolve();
export const GetCurrentWorkspace = (): Promise<any> => getApp().GetCurrentWorkspace?.() || Promise.resolve(null);
export const SaveWorkspace = (): Promise<void> => getApp().SaveWorkspace?.() || Promise.resolve();
export const SaveWorkspaceAs = (path: string): Promise<void> => getApp().SaveWorkspaceAs?.(path) || Promise.resolve();
export const SaveWorkspaceDialog = (): Promise<string> => getApp().SaveWorkspaceDialog?.() || Promise.resolve("");
export const PinRecent = (path: string, pinned: boolean): Promise<void> => getApp().PinRecent?.(path, pinned) || Promise.resolve();
export const RemoveRecent = (path: string): Promise<void> => getApp().RemoveRecent?.(path) || Promise.resolve();
export const OpenFolderDialog = (): Promise<string> => getApp().OpenFolderDialog?.() || Promise.resolve("");
export const OpenWorkspaceDialog = (): Promise<string> => getApp().OpenWorkspaceDialog?.() || Promise.resolve("");
export const OpenFileDialog = (): Promise<string> => getApp().OpenFileDialog?.() || Promise.resolve("");
export const OpenNewWindow = (url: string): Promise<void> => getApp().OpenNewWindow?.(url) || Promise.resolve();
export const ListSessions = (): Promise<any[]> => getApp().ListSessions?.() || Promise.resolve([]);
export const StopSession = (id: string): Promise<void> => getApp().StopSession?.(id) || Promise.resolve();
export const RenameSession = (id: string, name: string): Promise<void> => getApp().RenameSession?.(id, name) || Promise.resolve();
export const CreateShell = (name: string, cwd: string): Promise<any> => getApp().CreateShell?.(name, cwd) || Promise.resolve({});
export const ReadFile = (path: string): Promise<string> => getApp().ReadFile?.(path) || Promise.resolve("");
export const ReadFileBase64 = (path: string): Promise<string> => getApp().ReadFileBase64?.(path) || Promise.resolve("");
export const CheckSyntax = (path: string, content: string): Promise<any[]> => getApp().CheckSyntax?.(path, content) || Promise.resolve([]);
export const FormatCode = (path: string, content: string): Promise<string> => getApp().FormatCode?.(path, content) || Promise.resolve(content);
export const WriteFile = (path: string, content: string): Promise<void> => getApp().WriteFile?.(path, content) || Promise.resolve();
export const CreateFile = (path: string): Promise<void> => getApp().CreateFile?.(path) || Promise.resolve();
export const DeleteFile = (path: string): Promise<void> => getApp().DeleteFile?.(path) || Promise.resolve();
export const RenameFile = (oldPath: string, newPath: string): Promise<void> => getApp().RenameFile?.(oldPath, newPath) || Promise.resolve();
export const CopyFile = (src: string, dst: string): Promise<void> => getApp().CopyFile?.(src, dst) || Promise.resolve();
export const MoveFile = (src: string, dst: string): Promise<void> => getApp().MoveFile?.(src, dst) || Promise.resolve();
export const GetFileTree = (depth: number): Promise<string> => getApp().GetFileTree?.(depth) || Promise.resolve("[]");
export const ListDirectory = (dirPath: string): Promise<string> => getApp().ListDirectory?.(dirPath) || Promise.resolve("[]");
export const ExpandPath = (targetPath: string): Promise<string> => getApp().ExpandPath?.(targetPath) || Promise.resolve("[]");
export const ToggleHiddenFiles = (): Promise<boolean> => getApp().ToggleHiddenFiles?.() || Promise.resolve(true);
export const GetGitStatus = (repoPath: string): Promise<any> => getApp().GetGitStatus?.(repoPath) || Promise.resolve(null);
export const GetGitCommitGraph = (repoPath: string, offset: number, limit: number): Promise<any> => getApp().GetGitCommitGraph?.(repoPath, offset, limit) || Promise.resolve(null);
export const GetGitCommitDiff = (repoPath: string, hash: string): Promise<string> => getApp().GetGitCommitDiff?.(repoPath, hash) || Promise.resolve("");
export const GetGitFileDiff = (repoPath: string, path: string): Promise<string> => getApp().GetGitFileDiff?.(repoPath, path) || Promise.resolve("");
export const GetGitCommitFileDiff = (repoPath: string, hash: string, path: string): Promise<string> => getApp().GetGitCommitFileDiff?.(repoPath, hash, path) || Promise.resolve("");
export const GetGitFileContentAtCommit = (repoPath: string, hash: string, path: string): Promise<string> => getApp().GetGitFileContentAtCommit?.(repoPath, hash, path) || Promise.resolve("");
export const GitStage = (repoPath: string, paths: string[]): Promise<void> => getApp().GitStage?.(repoPath, paths) || Promise.resolve();
export const GitUnstage = (repoPath: string, paths: string[]): Promise<void> => getApp().GitUnstage?.(repoPath, paths) || Promise.resolve();
export const GitDiscard = (repoPath: string, paths: string[]): Promise<void> => getApp().GitDiscard?.(repoPath, paths) || Promise.resolve();
export const GitCommit = (repoPath: string, message: string): Promise<void> => getApp().GitCommit?.(repoPath, message) || Promise.resolve();
export const GitPush = (repoPath: string): Promise<void> => getApp().GitPush?.(repoPath) || Promise.resolve();
export const GenerateAICommitMessage = (repoPath: string, providerId: string, model: string, instruction?: string): Promise<string> => 
  getApp().GenerateAICommitMessage?.(repoPath, providerId, model, instruction || "") || Promise.resolve("");
export const WriteSession = (id: string, data: string): Promise<void> => getApp().WriteSession?.(id, data) || Promise.resolve();
export const ResizeSession = (id: string, rows: number, cols: number): Promise<void> => getApp().ResizeSession?.(id, rows, cols) || Promise.resolve();
export const ListAgentSessions = (): Promise<any[]> => getApp().ListAgentSessions?.() || Promise.resolve([]);
export const CreateAgentSession = (name: string, role: string, projectFolder: string): Promise<any> => 
  getApp().CreateAgentSession?.(name, role, projectFolder) || Promise.resolve({});
export const CreateAgentSessionFromDefinition = (defId: string, projectFolder: string): Promise<any> => 
  getApp().CreateAgentSessionFromDefinition?.(defId, projectFolder) || Promise.resolve({});
export const SendAgentMessage = (id: string, message: string, files: string[]): Promise<void> => 
  getApp().SendAgentMessage?.(id, message, files) || Promise.resolve();
export const RespondAgentApproval = (id: string, approve: boolean, autoAll: boolean): Promise<void> => 
  getApp().RespondAgentApproval?.(id, approve, autoAll) || Promise.resolve();
export const ToggleAgentTask = (id: string, taskId: string, active: boolean): Promise<void> => 
  getApp().ToggleAgentTask?.(id, taskId, active) || Promise.resolve();
export const DeleteAgentSession = (id: string): Promise<void> => 
  getApp().DeleteAgentSession?.(id) || Promise.resolve();
export const SearchFilename = (query: string, limit: number): Promise<any[]> => 
  getApp().SearchFilename?.(query, limit) || Promise.resolve([]);
export const SearchFilenameWithOptions = (opts: any): Promise<any[]> =>
  getApp().SearchFilenameWithOptions?.(opts) || Promise.resolve([]);
export const SearchContentWithOptions = (opts: any): Promise<any[]> =>
  getApp().SearchContentWithOptions?.(opts) || Promise.resolve([]);
export const GetProviderProfiles = (): Promise<any[]> => getApp().GetProviderProfiles?.() || Promise.resolve([]);
export const SaveProviderProfiles = (profiles: any[]): Promise<void> => getApp().SaveProviderProfiles?.(profiles) || Promise.resolve();
export const FetchProviderModels = (apiKey: string, baseURL: string): Promise<string[]> => 
  getApp().FetchProviderModels?.(apiKey, baseURL) || Promise.resolve([]);
export const SetActiveModel = (providerId: string, model: string): Promise<void> => 
  getApp().SetActiveModel?.(providerId, model) || Promise.resolve();
export const SaveLLMProfile = (providerId: string, apiKey: string, baseURL: string, model: string): Promise<void> => 
  getApp().SaveLLMProfile?.(providerId, apiKey, baseURL, model) || Promise.resolve();
export const GetLLMConfig = (): Promise<any> => getApp().GetLLMConfig?.() || Promise.resolve(null);
export const ListLLMProviders = (): Promise<any[]> => getApp().ListLLMProviders?.() || Promise.resolve([]);
export const ListAgentDefinitions = (): Promise<any[]> => getApp().ListAgentDefinitions?.() || Promise.resolve([]);
export const SaveAgentDefinition = (def: any): Promise<any> => getApp().SaveAgentDefinition?.(def) || Promise.resolve(def);
export const DeleteAgentDefinition = (id: string): Promise<void> => getApp().DeleteAgentDefinition?.(id) || Promise.resolve();
export const ListMCPServers = (): Promise<any[]> => getApp().ListMCPServers?.() || Promise.resolve([]);
export const SaveMCPServer = (server: any): Promise<any> => getApp().SaveMCPServer?.(server) || Promise.resolve(server);
export const DeleteMCPServer = (name: string): Promise<void> => getApp().DeleteMCPServer?.(name) || Promise.resolve();
export const ListMCPTools = (): Promise<any[]> => getApp().ListMCPTools?.() || Promise.resolve([]);
export const ListSkills = (): Promise<any[]> => getApp().ListSkills?.() || Promise.resolve([]);
export const GetHomeDir = (): Promise<string> => getApp().GetHomeDir?.() || Promise.resolve("");
export const OpenInFinder = (path: string): Promise<void> => getApp().OpenInFinder?.(path) || Promise.resolve();
export const IsDir = (path: string): Promise<boolean> => getApp().IsDir?.(path) || Promise.resolve(false);
