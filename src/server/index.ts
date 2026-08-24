import { WorkspaceManager } from "./workspace";
import { FileManager } from "./files";
import { ExplorerManager } from "./explorer";
import { GitManager } from "./git";
import { TerminalManager } from "./terminal";
import { EditorManager } from "./editor";
import { LLMManager } from "./llm";
import { AgentManager } from "./agent";
import { SearchManager } from "./search";
import { UsageManager } from "./usage";
import { MCPManager } from "./mcp";
import { SkillsManager } from "./skills";
import { SyntaxManager } from "./syntax";
import { getLanguageFromPath, languageIdFromPath, LANGUAGES } from "./language";
import { ConfigStore } from "./config";
import { listSlashCommands, executeLocalCommand } from "./slash";
import { LSPManager } from "./lsp";

export class ForgeServer {
  public workspace = new WorkspaceManager();
  public files = new FileManager();
  public explorer = new ExplorerManager();
  public git = new GitManager();
  public terminal = new TerminalManager();
  public editor = new EditorManager();
  public config = new ConfigStore();
  public llm = new LLMManager(this.config);
  public search = new SearchManager();
  public usage = new UsageManager();
  public mcp = new MCPManager();
  public skills = new SkillsManager(undefined, this.config);
  public agent = new AgentManager(this.llm, undefined, { mcp: this.mcp, skills: this.skills });
  public syntax = new SyntaxManager();
  public lsp = new LSPManager();

  public async handleMethod(method: string, params: any = {}): Promise<any> {
    const ws = this.workspace.getCurrentWorkspace();
    const workspaceFolders = ws?.folders || [process.cwd()];
    const primaryFolder = workspaceFolders[0] || process.cwd();

    switch (method) {
      // ---------------------------------------------------------------------
      // Workspace
      // ---------------------------------------------------------------------
      case "forge.OpenFolder":
      case "OpenFolder":
        return this.workspace.openFolder(params.folderPath || params.path || primaryFolder);

      case "forge.OpenWorkspace":
      case "OpenWorkspace":
        return this.workspace.openWorkspace(params.filePath || params.path);

      case "forge.SaveWorkspace":
      case "SaveWorkspace":
        return this.workspace.saveWorkspace();

      case "forge.SaveWorkspaceAs":
      case "SaveWorkspaceAs":
        return this.workspace.saveWorkspaceAs(params.filePath || params.path);

      case "forge.CloseWorkspace":
      case "CloseWorkspace":
        return this.workspace.closeWorkspace();

      case "forge.AddFolderToWorkspace":
      case "AddFolderToWorkspace":
        return this.workspace.addFolderToWorkspace(params.folderPath || params.path);

      case "forge.RemoveFolderFromWorkspace":
      case "RemoveFolderFromWorkspace":
        return this.workspace.removeFolderFromWorkspace(params.folderPath || params.path);

      case "forge.GetCurrentWorkspace":
      case "GetCurrentWorkspace":
        return this.workspace.getCurrentWorkspace();

      case "forge.GetRecentProjects":
      case "GetRecentProjects":
        return this.workspace.getRecentProjects();

      case "forge.PinRecent":
      case "PinRecent":
        return this.workspace.pinRecent(params.path, params.pinned);

      case "forge.RemoveRecent":
      case "RemoveRecent":
        return this.workspace.removeRecent(params.path);

      // ---------------------------------------------------------------------
      // Explorer & Files
      // ---------------------------------------------------------------------
      case "forge.GetFileTree":
      case "GetFileTree":
        return this.explorer.getFileTree(workspaceFolders, params.depth || 2);

      case "forge.ListDirectory":
      case "ListDirectory":
        return this.explorer.listDirectory(params.dirPath || params.path || primaryFolder);

      case "forge.ExpandPath":
      case "ExpandPath":
        return this.explorer.expandPath(params.targetPath || params.path);

      case "forge.ToggleHiddenFiles":
      case "ToggleHiddenFiles":
        return this.explorer.toggleHiddenFiles();

      case "forge.ReadFile":
      case "ReadFile":
        return this.files.readFile(params.path);

      case "forge.ReadFileBase64":
      case "ReadFileBase64":
        return this.files.readFileBase64(params.path);

      case "forge.WriteFile":
      case "WriteFile":
        return this.files.writeFile(params.path, params.content ?? "");

      case "forge.CreateFile":
      case "CreateFile":
        return this.files.createFile(params.path);

      case "forge.CreateFolder":
      case "CreateFolder":
        return this.files.createFolder(params.path);

      case "forge.DeleteFile":
      case "DeleteFile":
        return this.files.deleteFile(params.path);

      case "forge.RenameFile":
      case "RenameFile":
        return this.files.renameFile(params.oldPath, params.newPath);

      case "forge.CopyFile":
      case "CopyFile":
        return this.files.copyFile(params.src, params.dst);

      case "forge.CopyPath":
      case "CopyPath":
        return this.files.copyPath(params.src, params.dst);

      case "forge.MoveFile":
      case "MoveFile":
        return this.files.moveFile(params.src, params.dst);

      case "forge.GetClipboardFiles":
      case "GetClipboardFiles":
        return this.files.getClipboardFiles();

      case "forge.IsDir":
      case "IsDir":
        return this.files.isDir(params.path);

      case "forge.GetHomeDir":
      case "GetHomeDir":
        return this.files.getHomeDir();

      case "forge.ResolvePath":
      case "ResolvePath":
        return this.files.resolvePath(params.path);

      // ---------------------------------------------------------------------
      // Editor Tooling
      // ---------------------------------------------------------------------
      case "forge.CheckSyntax":
      case "CheckSyntax":
        return this.editor.checkSyntax(params.path, params.content ?? "");

      case "forge.FormatCode":
      case "FormatCode":
        return this.editor.formatCode(params.path, params.content ?? "");

      case "forge.GetCompletion":
      case "GetCompletion":
        return this.editor.getCompletion(params.prefix || "", params.path || "");

      case "forge.GetMembers":
      case "GetMembers":
        return this.editor.getMembers(params.instance || "", params.path || "");

      case "forge.FindSymbol":
      case "FindSymbol":
        return this.editor.findSymbol(params.name || "", workspaceFolders);

      case "forge.SearchIndexSymbols":
      case "SearchIndexSymbols":
        return this.editor.searchIndexSymbols(params.query || "", workspaceFolders);

      case "forge.GetLanguage":
      case "GetLanguage":
        return params.path ? getLanguageFromPath(params.path) : (params.id ? LANGUAGES[params.id] : null);

      case "forge.LanguageIdFromPath":
      case "LanguageIdFromPath":
        return languageIdFromPath(params.path || "");

      case "forge.GetHighlightQuery":
      case "GetHighlightQuery":
        return this.syntax.getHighlightQuery(params.languageId || params.id);

      case "forge.GetGrammar":
      case "GetGrammar":
        return this.syntax.findGrammar(params.languageId || params.id);

      case "forge.Tokenize":
      case "Tokenize":
        return this.syntax.tokenize(params.path, params.content ?? "");

      // ---------------------------------------------------------------------
      // LSP (Language Server Protocol)
      // ---------------------------------------------------------------------
      case "forge.LSPDidOpen":
      case "LSPDidOpen":
        await this.lsp.didOpen(params.path, params.content ?? "", primaryFolder);
        return true;

      case "forge.LSPDidChange":
      case "LSPDidChange":
        await this.lsp.didChange(params.path, params.content ?? "", primaryFolder);
        return true;

      case "forge.LSPDidSave":
      case "LSPDidSave":
        await this.lsp.didSave(params.path, params.content, primaryFolder);
        return true;

      case "forge.LSPDidClose":
      case "LSPDidClose":
        await this.lsp.didClose(params.path, primaryFolder);
        return true;

      case "forge.LSPGetCompletion":
      case "LSPGetCompletion":
        return this.lsp.getCompletion(params.path, params.line ?? 0, params.character ?? 0, primaryFolder);

      case "forge.LSPGetHover":
      case "LSPGetHover":
        return this.lsp.getHover(params.path, params.line ?? 0, params.character ?? 0, primaryFolder);

      case "forge.LSPGetDefinition":
      case "LSPGetDefinition":
        return this.lsp.getDefinition(params.path, params.line ?? 0, params.character ?? 0, primaryFolder);

      case "forge.LSPGetDeclaration":
      case "LSPGetDeclaration":
        return this.lsp.getDeclaration(params.path, params.line ?? 0, params.character ?? 0, primaryFolder);

      case "forge.LSPGetTypeDefinition":
      case "LSPGetTypeDefinition":
        return this.lsp.getTypeDefinition(params.path, params.line ?? 0, params.character ?? 0, primaryFolder);

      case "forge.LSPGetImplementation":
      case "LSPGetImplementation":
        return this.lsp.getImplementation(params.path, params.line ?? 0, params.character ?? 0, primaryFolder);

      case "forge.LSPGetDiagnostics":
      case "LSPGetDiagnostics":
        return this.lsp.getDiagnostics(params.path);

      case "forge.LSPListServers":
      case "LSPListServers":
        return this.lsp.listServers(primaryFolder);

      case "forge.LSPRestartServer":
      case "LSPRestartServer":
        return this.lsp.restartServer(params.languageId || params.id, primaryFolder);

      case "forge.LSPStopServer":
      case "LSPStopServer":
        return this.lsp.stopServer(params.languageId || params.id, primaryFolder);

      case "forge.LSPRestartAll":
      case "LSPRestartAll":
        return this.lsp.restartAll(primaryFolder);

      case "forge.LSPStopAll":
      case "LSPStopAll":
        return this.lsp.stopAll(primaryFolder);

      case "forge.LSPGetServerLogs":
      case "LSPGetServerLogs":
        return this.lsp.getLogs(params.languageId || params.id, primaryFolder);

      // ---------------------------------------------------------------------
      // Terminal Sessions
      // ---------------------------------------------------------------------
      case "forge.CreateShell":
      case "CreateShell":
        return this.terminal.createShell(params.name, params.cwd || primaryFolder);

      case "forge.CreateAIAgent":
      case "CreateAIAgent":
        return this.terminal.createAIAgent(params.name, params.provider, params.folder || primaryFolder);

      case "forge.WriteSession":
      case "WriteSession":
        return this.terminal.writeSession(params.id, params.data);

      case "forge.ResizeSession":
      case "ResizeSession":
        return this.terminal.resizeSession(params.id, params.rows, params.cols);

      case "forge.StopSession":
      case "StopSession":
        return this.terminal.stopSession(params.id);

      case "forge.RenameSession":
      case "RenameSession":
        return this.terminal.renameSession(params.id, params.name);

      case "forge.ListSessions":
      case "ListSessions":
        return this.terminal.listSessions();

      case "forge.ListShells":
      case "ListShells":
        return this.terminal.listByType("shell");

      case "forge.ListAIAgents":
      case "ListAIAgents":
        return this.terminal.listByType("ai");

      // ---------------------------------------------------------------------
      // Git
      // ---------------------------------------------------------------------
      case "forge.GetGitStatus":
      case "GetGitStatus":
        return this.git.getGitStatus(params.repoPath || primaryFolder);

      case "forge.GetGitCommitGraph":
      case "GetGitCommitGraph":
        return this.git.getGitCommitGraph(
          params.repoPath || primaryFolder,
          params.offset || 0,
          params.limit || 50,
          params.branch || ""
        );

      case "forge.GetGitBranches":
      case "GetGitBranches":
        return this.git.getGitBranches(params.repoPath || primaryFolder);

      case "forge.GetGitCommitDiff":
      case "GetGitCommitDiff":
        return this.git.getGitCommitDiff(params.repoPath || primaryFolder, params.hash);

      case "forge.GetGitCommitBody":
      case "GetGitCommitBody":
        return this.git.getGitCommitBody(params.repoPath || primaryFolder, params.hash);

      case "forge.GetGitFileDiff":
      case "GetGitFileDiff":
        return this.git.getGitFileDiff(params.repoPath || primaryFolder, params.path);

      case "forge.GetGitCommitFileDiff":
      case "GetGitCommitFileDiff":
        return this.git.getGitCommitFileDiff(params.repoPath || primaryFolder, params.hash, params.path);

      case "forge.GetGitFileDiffHunks":
      case "GetGitFileDiffHunks":
        return this.git.getGitFileDiffHunks(params.repoPath || primaryFolder, params.path);

      case "forge.RevertGitHunk":
      case "RevertGitHunk":
        return this.git.revertGitHunk(params.repoPath || primaryFolder, params.path, params.hunkIndex);

      case "forge.GetGitFileContentAtCommit":
      case "GetGitFileContentAtCommit":
        return this.git.getGitFileContentAtCommit(params.repoPath || primaryFolder, params.hash, params.path);

      case "forge.GitStage":
      case "GitStage":
        return this.git.gitStage(params.repoPath || primaryFolder, params.paths || []);

      case "forge.GitUnstage":
      case "GitUnstage":
        return this.git.gitUnstage(params.repoPath || primaryFolder, params.paths || []);

      case "forge.GitDiscard":
      case "GitDiscard":
        return this.git.gitDiscard(params.repoPath || primaryFolder, params.paths || []);

      case "forge.GetGitConflictStageContent":
      case "GetGitConflictStageContent":
        return this.git.getGitConflictStageContent(params.repoPath || primaryFolder, params.path, params.stage);

      case "forge.GitResolveConflict":
      case "GitResolveConflict":
        return this.git.gitResolveConflict(params.repoPath || primaryFolder, params.path, params.action);

      case "forge.GitCommit":
      case "GitCommit":
        return this.git.gitCommit(params.repoPath || primaryFolder, params.message, params.amend);

      case "forge.GitPush":
      case "GitPush":
        return this.git.gitPush(params.repoPath || primaryFolder, params.force);

      case "forge.GitFetch":
      case "GitFetch":
        return this.git.gitFetch(params.repoPath || primaryFolder);

      case "forge.GitMerge":
      case "GitMerge":
        return this.git.gitMerge(params.repoPath || primaryFolder, params.source, params.noFF, params.squash);

      case "forge.GenerateAICommitMessage":
      case "GenerateAICommitMessage":
        return await this.git.generateAICommitMessage(
          params.repoPath || primaryFolder,
          params.providerId,
          params.model,
          params.instruction,
          this.llm,
        );

      // ---------------------------------------------------------------------
      // AI Agents
      // ---------------------------------------------------------------------
      case "forge.CreateAgentSession":
      case "CreateAgentSession":
        return this.agent.createSession(params.name, params.role, params.projectFolder || primaryFolder);

      case "forge.CreateAgentSessionFromDefinition":
      case "CreateAgentSessionFromDefinition":
        return this.agent.createSessionFromDefinition(params.defId, params.projectFolder || primaryFolder);

      case "forge.ListAgentSessions":
      case "ListAgentSessions":
        return this.agent.listSessions();

      case "forge.ListAgentSessionsForFolder":
      case "ListAgentSessionsForFolder":
        return this.agent.listSessionsForFolder(params.folder || primaryFolder);

      case "forge.GetAgentSession":
      case "GetAgentSession":
        return this.agent.getSession(params.id);

      case "forge.UpdateAgentSession":
      case "UpdateAgentSession":
        return this.agent.updateSession(params.id, params.name, params.role, params.customPrompt, params.customRules);

      case "forge.DeleteAgentSession":
      case "DeleteAgentSession":
        return this.agent.deleteSession(params.id);

      case "forge.ClearAgentSession":
      case "ClearAgentSession":
        return this.agent.clearSession(params.id);

      case "forge.SetAgentDialect":
      case "SetAgentDialect":
        return this.agent.setDialect(params.id, params.dialect);

      case "forge.SetAgentAutoApprove":
      case "SetAgentAutoApprove":
        return this.agent.setAutoApprove(params.id, params.enabled);

      case "forge.ToggleAgentTask":
      case "ToggleAgentTask":
        return this.agent.toggleTask(params.id, params.taskId, params.active);

      case "forge.SendAgentMessage":
      case "SendAgentMessage":
        return this.agent.sendMessage(params.id, params.message || params.content, params.files || []);

      case "forge.RespondAgentApproval":
      case "RespondAgentApproval":
        return this.agent.respondApproval(params.id, params.approve, params.autoAll);

      case "forge.RespondAgentAsk":
      case "RespondAgentAsk":
        return this.agent.respondAsk(params.id, params.answers);

      case "forge.StopAgentTurn":
      case "StopAgentTurn":
        return this.agent.stopTurn(params.id);

      case "forge.ListAgentDefinitions":
      case "ListAgentDefinitions":
        return this.agent.listDefinitions();

      case "forge.SaveAgentDefinition":
      case "SaveAgentDefinition":
        return this.agent.saveDefinition(params.def);

      case "forge.DeleteAgentDefinition":
      case "DeleteAgentDefinition":
        return this.agent.deleteDefinition(params.id);

      case "forge.ApplyAgentDefinitionToSession":
      case "ApplyAgentDefinitionToSession":
        return this.agent.applyDefinitionToSession(params.id, params.defId);

      // ---------------------------------------------------------------------
      // LLM Config
      // ---------------------------------------------------------------------
      case "forge.GetProviderProfiles":
      case "GetProviderProfiles":
        return this.llm.getProviderProfiles();

      case "forge.SaveProviderProfiles":
      case "SaveProviderProfiles":
        return this.llm.saveProviderProfiles(params.profiles || []);

      case "forge.ListProviderProfiles":
      case "ListProviderProfiles":
        return this.llm.getProviderProfiles();

      case "forge.FetchProviderModels":
      case "FetchProviderModels":
        return this.llm.fetchProviderModels(params.apiKey, params.baseURL);

      case "forge.SetActiveModel":
      case "SetActiveModel":
        return this.llm.setActiveModel(params.providerId, params.model);

      case "forge.SaveLLMProfile":
      case "SaveLLMProfile":
        return this.llm.saveLLMProfile(params.providerId, params.apiKey, params.baseURL, params.model);

      case "forge.GetLLMConfig":
      case "GetLLMConfig":
        return this.llm.getLLMConfig();

      case "forge.ListLLMProviders":
      case "ListLLMProviders":
        return this.llm.listLLMProviders();

      // ---------------------------------------------------------------------
      // Search
      // ---------------------------------------------------------------------
      case "forge.SearchFilename":
      case "SearchFilename":
        return this.search.searchFilename(params.query || "", workspaceFolders, params.limit || 50);

      case "forge.SearchFilenameWithOptions":
      case "SearchFilenameWithOptions":
        return this.search.searchFilename(params.opts?.query || "", workspaceFolders, params.opts?.limit || 50);

      case "forge.SearchContentWithOptions":
      case "SearchContentWithOptions":
        return this.search.searchContent(params.opts || {}, workspaceFolders);

      case "forge.SearchReplaceAll":
      case "SearchReplaceAll":
        return this.search.searchReplaceAll(params.opts || {}, workspaceFolders);

      case "forge.SearchSymbols":
      case "SearchSymbols":
        return [];

      case "forge.SearchSymbolsWithOptions":
      case "SearchSymbolsWithOptions":
        return [];

      // ---------------------------------------------------------------------
      // Analytics & Usage
      // ---------------------------------------------------------------------
      case "forge.GetAllUsageRecords":
      case "GetAllUsageRecords":
        return this.usage.getAllRecords();

      case "forge.GetUsageOverview":
      case "GetUsageOverview":
        return this.usage.getOverview(params.filter || "today");

      case "forge.GetUsageTimeSeries":
      case "GetUsageTimeSeries":
        return this.usage.getTimeSeries(params.filter || "today");

      case "forge.GetUsageRequests":
      case "GetUsageRequests":
        return this.usage.getRequests(params.filter || "today", params.limit || 50);

      case "forge.GetUsageBuckets":
      case "GetUsageBuckets":
        return this.usage.getBuckets(params.dimension || "model", params.filter || "today");

      case "forge.GetUsageFilterOptions":
      case "GetUsageFilterOptions":
        return this.usage.getFilterOptions();

      // ---------------------------------------------------------------------
      // MCP & Skills
      // ---------------------------------------------------------------------
      case "forge.ListMCPServers":
      case "ListMCPServers":
        return this.mcp.listServers();

      case "forge.SaveMCPServer":
      case "SaveMCPServer":
        return this.mcp.saveServer(params.server);

      case "forge.DeleteMCPServer":
      case "DeleteMCPServer":
        return this.mcp.deleteServer(params.name);

      case "forge.ListMCPTools":
      case "ListMCPTools":
        return this.mcp.listTools();

      case "forge.ListConnectedMCPTools":
      case "ListConnectedMCPTools":
        return this.mcp.listConnectedTools();

      case "forge.ReconnectMCP":
      case "ReconnectMCP":
        return this.mcp.reconnect();

      case "forge.ListSkills":
      case "ListSkills":
        return this.skills.listSkills(primaryFolder);

      case "forge.ListAllSkills":
      case "ListAllSkills":
        return this.skills.listAllSkills(primaryFolder);

      case "forge.SetSkillEnabled":
      case "SetSkillEnabled":
        this.skills.setSkillEnabled(String(params.name ?? ""), Boolean(params.enabled));
        return true;

      case "forge.SetAllSkillsEnabled":
      case "SetAllSkillsEnabled":
        this.skills.setAllSkillsEnabled(Boolean(params.enabled), primaryFolder);
        return true;
      case "forge.ListSlashCommands":
      case "ListSlashCommands":
        return listSlashCommands(
          { skills: this.skills, mcp: this.mcp, llm: this.llm },
          params.query,
          primaryFolder,
        );

      case "forge.ExecuteSlashCommand":
      case "ExecuteSlashCommand": {
        const result = executeLocalCommand(
          String(params.text ?? ""),
          { skills: this.skills, mcp: this.mcp, llm: this.llm },
          {
            sessionId: typeof params.sessionId === "string" ? params.sessionId : undefined,
            sessionUsage: (id) => this.agent.getUsageSummary(id),
          },
        );
        if (result) return result;
        // Not a local command — hand it to the agent like a normal message.
        if (typeof params.sessionId !== "string") throw new Error("sessionId required");
        await this.agent.sendMessage(params.sessionId, String(params.text ?? ""), []);
        return { handled: false };
      }

      default:
        console.warn(`[ForgeServer] Unhandled method: ${method}`);
        return null;
    }
  }
}

export const server = new ForgeServer();
