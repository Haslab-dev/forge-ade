import { FileItem, FileDiff, ToolExecution, ThoughtStep, ACPAgent, LLMProviderConfig, MCPEntry, SkillEntry, AgentMessage } from '../types';
import { ApiBridge } from './apiBridge';
import { DEFAULT_PROVIDERS } from '../stores/agentRegistryStore';

export interface ToolContext {
  files: FileItem[];
  workspacePath: string;
  updateFileContent: (filePath: string, content: string) => void;
  createFile?: (filePath: string, content: string) => void;
  onDiffGenerated?: (diff: FileDiff) => void;
  attachedFiles?: { name: string; content: string }[];
  activeModel?: string;
  messages?: AgentMessage[];
}

export interface AgentExecutionCallbacks {
  onThought: (thought: ThoughtStep) => void;
  onToolStart: (tool: ToolExecution) => void;
  onToolComplete: (tool: ToolExecution) => void;
  onContentChunk: (chunk: string) => void;
  onDiffCreated: (diff: FileDiff) => void;
  onFinish: (finalContent: string) => void;
  onError: (err: string) => void;
}

interface ParsedToolCall {
  name: string;
  args: Record<string, any>;
  rawBlock: string;
}

export class AgentEngine {
  private isAborted = false;

  public abort() {
    this.isAborted = true;
  }

  public static getAllFiles(items: FileItem[]): FileItem[] {
    let list: FileItem[] = [];
    for (const item of items) {
      if (item.type === 'file') {
        list.push(item);
      }
      if (item.children) {
        list = list.concat(AgentEngine.getAllFiles(item.children));
      }
    }
    return list;
  }

  public static generateDiff(filePath: string, originalContent: string, newContent: string): FileDiff {
    const origLines = originalContent ? originalContent.split('\n') : [];
    const newLines = newContent ? newContent.split('\n') : [];

    let additions = 0;
    let deletions = 0;

    const maxLen = Math.max(origLines.length, newLines.length);
    for (let i = 0; i < maxLen; i++) {
      if (i >= origLines.length) additions++;
      else if (i >= newLines.length) deletions++;
      else if (origLines[i] !== newLines[i]) {
        additions++;
        deletions++;
      }
    }

    const fileName = filePath.split('/').pop() || filePath;

    return {
      id: `diff-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
      filePath,
      fileName,
      originalContent,
      modifiedContent: newContent,
      additions: Math.max(1, additions),
      deletions,
      status: 'pending',
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    };
  }

  // Strip special tokens and formatting artifacts from model output
  public static sanitizeModelOutput(text: string): string {
    if (!text) return '';
    return text
      .replace(/<\|start\|>assistant/gi, '')
      .replace(/<\|channel\|>[\s\S]*?<\|call\|>/gi, '')
      .replace(/<\|channel\|>[\s\S]*?<\|message\|>/gi, '')
      .replace(/<\|constrain\|>[^\s<]*/gi, '')
      .replace(/<\|call\|>/gi, '')
      .replace(/<\|start\|>/gi, '')
      .replace(/<\|message\|>/gi, '')
      .replace(/<\|im_start\|>assistant/gi, '')
      .replace(/<\|im_start\|>/gi, '')
      .replace(/<\|im_end\|>/gi, '')
      .replace(/<\|endoftext\|>/gi, '')
      .replace(/<think>[\s\S]*?<\/think>/gi, '')
      .replace(/<tool_call[\s\S]*?<\/tool_call>/gi, '')
      .replace(/<tool_call[\s\S]*$/gi, '')
      .replace(/<function_call[\s\S]*?<\/function_call>/gi, '')
      .replace(/```(?:tool|json)?\s*\{[\s\S]*?"(?:tool|name|action)"[\s\S]*?\}\s*```/gi, '')
      .trim();
  }

  // Parse tool calls from model output (supports XML, Qwen/DeepSeek/Hermes special tokens, JSON, or codeblocks)
  private parseToolCalls(text: string): ParsedToolCall[] {
    const calls: ParsedToolCall[] = [];

    // Pattern 1: XML tags <tool_call name="read">{"path": "foo"}</tool_call> or <tool_call>{"name": "ls", ...}</tool_call>
    const xmlRegex = /<tool_call(?:\s+name=["']([^"']+)["'])?>([\s\S]*?)<\/tool_call>/gi;
    let match: RegExpExecArray | null;
    while ((match = xmlRegex.exec(text)) !== null) {
      try {
        if (match[1]) {
          const name = match[1].toLowerCase().trim();
          const args = JSON.parse(match[2].trim() || '{}');
          calls.push({ name, args, rawBlock: match[0] });
        } else {
          const obj = JSON.parse(match[2].trim() || '{}');
          const name = (obj.name || obj.tool || '').toLowerCase().trim();
          const args = obj.arguments || obj.args || obj.parameters || obj;
          if (name) calls.push({ name, args, rawBlock: match[0] });
        }
      } catch {}
    }

    if (calls.length > 0) return calls;

    // Pattern 2: Qwen / DeepSeek / ChatML Special Tokens
    // e.g. <|start|>assistant<|channel|>commentary to=ls <|constrain|>json<|message|>{"path": ""}<|call|>
    const qwenRegex = /to=([a-zA-Z0-9_-]+)[^<]*<\|message\|>([\s\S]*?)<\|call\|>/gi;
    while ((match = qwenRegex.exec(text)) !== null) {
      try {
        const name = match[1].toLowerCase().trim();
        const args = JSON.parse(match[2].trim() || '{}');
        calls.push({ name, args, rawBlock: match[0] });
      } catch {}
    }

    if (calls.length > 0) return calls;

    // Pattern 3: <|channel|>commentary to=ls ... {"path": ""}
    const channelRegex = /<\|channel\|>.*?to=([a-zA-Z0-9_-]+)[\s\S]*?(\{[\s\S]*?\})/gi;
    while ((match = channelRegex.exec(text)) !== null) {
      try {
        const name = match[1].toLowerCase().trim();
        const args = JSON.parse(match[2].trim() || '{}');
        calls.push({ name, args, rawBlock: match[0] });
      } catch {}
    }

    if (calls.length > 0) return calls;

    // Pattern 4: ```tool:read\n{"path": "foo"}\n```
    const fenceRegex = /```(?:tool:)?(read|write|edit|bash|grep|find|ls)\s*\n([\s\S]*?)\n```/gi;
    while ((match = fenceRegex.exec(text)) !== null) {
      try {
        const name = match[1].toLowerCase().trim();
        const args = JSON.parse(match[2].trim() || '{}');
        calls.push({ name, args, rawBlock: match[0] });
      } catch {}
    }

    if (calls.length > 0) return calls;

    // Pattern 5: JSON object with "tool" or "name"
    const jsonBlockRegex = /```(?:json)?\s*\n(\{\s*"(?:tool|name|action)":[\s\S]*?\})\s*\n```/gi;
    while ((match = jsonBlockRegex.exec(text)) !== null) {
      try {
        const obj = JSON.parse(match[1]);
        const name = (obj.tool || obj.name || obj.action || '').toLowerCase().trim();
        const args = obj.args || obj.parameters || obj.input || obj;
        if (name) {
          calls.push({ name, args, rawBlock: match[0] });
        }
      } catch {}
    }

    return calls;
  }

  // Execute a Pi-compatible tool against real system
  private async executeTool(
    toolName: string,
    args: Record<string, any>,
    context: ToolContext,
    callbacks: AgentExecutionCallbacks
  ): Promise<{ output: string; diff?: FileDiff }> {
    const cleanName = toolName.toLowerCase().replace(/^(tools?_|tool:)/, '');

    // 1. read / view_file
    if (cleanName === 'read' || cleanName === 'view_file' || cleanName === 'read_file') {
      const filePath = args.path || args.filePath || args.targetFile || args.file;
      if (!filePath) return { output: 'Error: missing "path" argument' };

      const resolved = filePath.startsWith('/') ? filePath : `${context.workspacePath}/${filePath}`;
      try {
        const content = await ApiBridge.readFile(resolved);
        const lines = content.split('\n');
        const startLine = Math.max(1, parseInt(args.startLine || args.start || '1', 10));
        const endLine = args.endLine || args.end ? parseInt(args.endLine || args.end, 10) : lines.length;
        const sliced = lines.slice(startLine - 1, endLine).join('\n');

        return {
          output: `Lines ${startLine}-${Math.min(endLine, lines.length)} of ${filePath} (${lines.length} total lines):\n${sliced}`
        };
      } catch (err: any) {
        return { output: `Error reading file ${filePath}: ${err.message || err}` };
      }
    }

    // 2. write / create_file
    if (cleanName === 'write' || cleanName === 'write_file' || cleanName === 'create_file') {
      const filePath = args.path || args.filePath || args.targetFile || args.file;
      const content = args.content !== undefined ? args.content : args.codeContent || '';
      if (!filePath) return { output: 'Error: missing "path" argument' };

      const resolved = filePath.startsWith('/') ? filePath : `${context.workspacePath}/${filePath}`;
      let origContent = '';
      try {
        origContent = await ApiBridge.readFile(resolved);
      } catch {}

      try {
        await ApiBridge.createFile(resolved, content);
        context.updateFileContent(resolved, content);

        const diff = AgentEngine.generateDiff(resolved, origContent, content);
        callbacks.onDiffCreated(diff);

        return {
          output: `Successfully wrote ${content.length} bytes to ${filePath}. Diff staged for review.`,
          diff
        };
      } catch (err: any) {
        return { output: `Error writing to ${filePath}: ${err.message || err}` };
      }
    }

    // 3. edit / replace_chunk
    if (cleanName === 'edit' || cleanName === 'replace_file_content' || cleanName === 'edit_file') {
      const filePath = args.path || args.filePath || args.targetFile || args.file;
      const targetContent = args.targetContent || args.oldText || args.search || args.old_string || '';
      const replacementContent = args.replacementContent || args.newText || args.replace || args.new_string || '';

      if (!filePath) return { output: 'Error: missing "path" argument' };

      const resolved = filePath.startsWith('/') ? filePath : `${context.workspacePath}/${filePath}`;
      try {
        const origContent = await ApiBridge.readFile(resolved);
        if (!origContent.includes(targetContent)) {
          return { output: `Error: targetContent not found in ${filePath}. Please re-read the file to ensure exact match.` };
        }

        const newContent = origContent.replace(targetContent, replacementContent);
        await ApiBridge.createFile(resolved, newContent);
        context.updateFileContent(resolved, newContent);

        const diff = AgentEngine.generateDiff(resolved, origContent, newContent);
        callbacks.onDiffCreated(diff);

        return {
          output: `Successfully edited ${filePath}. Diff generated (+${diff.additions} -${diff.deletions}).`,
          diff
        };
      } catch (err: any) {
        return { output: `Error editing ${filePath}: ${err.message || err}` };
      }
    }

    // 3b. delete / delete_file / remove_file
    if (cleanName === 'delete' || cleanName === 'delete_file' || cleanName === 'remove_file' || cleanName === 'rm') {
      const filePath = args.path || args.filePath || args.targetFile || args.file;
      if (!filePath) return { output: 'Error: missing "path" argument' };

      const resolved = filePath.startsWith('/') ? filePath : `${context.workspacePath}/${filePath}`;
      try {
        let origContent = '';
        try {
          origContent = await ApiBridge.readFile(resolved);
        } catch {}

        await ApiBridge.deleteFile(resolved);
        const diff = AgentEngine.generateDiff(resolved, origContent, '');
        callbacks.onDiffCreated(diff);

        return {
          output: `Successfully deleted ${filePath}.`,
          diff
        };
      } catch (err: any) {
        return { output: `Error deleting ${filePath}: ${err.message || err}` };
      }
    }

    // 4. bash / exec_command / run
    if (cleanName === 'bash' || cleanName === 'run_command' || cleanName === 'exec' || cleanName === 'run') {
      const cmd = args.command || args.cmd || args.commandLine || (typeof args === 'string' ? args : '');
      if (!cmd) return { output: 'Error: missing "command" argument' };

      // Prevent agent from executing commands to grep/tail internal session files
      if (cmd.includes('.forge-ade/sessions') || cmd.includes('.gemini/antigravity-cli/brain')) {
        return { output: 'Session history is managed directly within conversation context.' };
      }

      try {
        const result = await ApiBridge.executeCommand(cmd, context.workspacePath);
        const out = (result.stdout || '') + (result.stderr ? `\nSTDERR:\n${result.stderr}` : '');
        return {
          output: out.trim() || `(Process exited with code ${result.exitCode})`
        };
      } catch (err: any) {
        return { output: `Command failed: ${err.message || err}` };
      }
    }

    // 5. grep / grep_search
    if (cleanName === 'grep' || cleanName === 'grep_search') {
      const pattern = args.query || args.pattern || args.search || '';
      let searchPath = args.path || args.searchPath || context.workspacePath;
      if (!searchPath || searchPath === '.' || searchPath === './') searchPath = context.workspacePath;
      if (!pattern) return { output: 'Error: missing "query" pattern' };

      try {
        const cmd = `rg -n --max-count 30 --glob "!node_modules/**" --glob "!dist/**" --glob "!.my-ade/**" --glob "!.git/**" "${pattern}" "${searchPath}"`;
        const result = await ApiBridge.executeCommand(cmd, context.workspacePath);
        return {
          output: result.stdout || 'No matching lines found.'
        };
      } catch {
        return { output: 'No matching lines found.' };
      }
    }

    // 6. find / find_files
    if (cleanName === 'find' || cleanName === 'find_by_name' || cleanName === 'find_files') {
      const pattern = args.pattern || args.name || '*';
      try {
        const cmd = `find . -maxdepth 4 -name "${pattern}" ! -path "*/node_modules/*" ! -path "*/.git/*" ! -path "*/dist/*" ! -path "*/.my-ade/*" | head -n 30`;
        const result = await ApiBridge.executeCommand(cmd, context.workspacePath);
        return {
          output: result.stdout || 'No matching files found.'
        };
      } catch (err: any) {
        return { output: `Find failed: ${err.message || err}` };
      }
    }

    // 7. ls / list_dir
    if (cleanName === 'ls' || cleanName === 'list_dir') {
      let dirPath = args.path || args.directory || context.workspacePath;
      if (!dirPath || dirPath === '.' || dirPath === './' || dirPath === '""' || dirPath === "''") {
        dirPath = context.workspacePath;
      } else if (!dirPath.startsWith('/')) {
        dirPath = `${context.workspacePath}/${dirPath}`;
      }

      try {
        const tree = await ApiBridge.readDirectoryTree(dirPath);
        const filtered = tree.filter(item => !['node_modules', '.git', '.forge-ade', 'dist', 'zig-out', '.zig-cache'].includes(item.name));
        const listing = filtered.slice(0, 40).map(item => `${item.type === 'folder' ? '[DIR]' : '[FILE]'} ${item.name}`).join('\n');
        return {
          output: listing || 'Directory is empty.'
        };
      } catch (err: any) {
        return { output: `Error listing directory: ${err.message || err}` };
      }
    }

    return { output: `Unknown tool: ${cleanName}` };
  }

  public async runSession(
    prompt: string,
    agent: ACPAgent,
    context: ToolContext,
    callbacks: AgentExecutionCallbacks
  ): Promise<void> {
    this.isAborted = false;
    const trimmedPrompt = prompt.trim();

    try {
      // 1. Refresh directory tree from real disk
      const realTree = await ApiBridge.readDirectoryTree(context.workspacePath);
      const allFiles = realTree.length > 0 ? AgentEngine.getAllFiles(realTree) : AgentEngine.getAllFiles(context.files);

      // 2. Read active skills and MCPs from localStorage
      let activeSkills: SkillEntry[] = [];
      try {
        const raw = localStorage.getItem('forge_ade_skills') || localStorage.getItem('my_ade_skills');
        if (raw) activeSkills = JSON.parse(raw);
      } catch {}

      let activeMcps: MCPEntry[] = [];
      try {
        const raw = localStorage.getItem('forge_ade_mcps') || localStorage.getItem('my_ade_mcps');
        if (raw) activeMcps = JSON.parse(raw);
      } catch {}

      // 3. Direct Command Execution Request (e.g. "run npm test", "exec git status")
      const execMatch = trimmedPrompt.match(/^(run|exec|terminal|command|execute)\s+(.+)/i);
      if (execMatch) {
        const cmd = execMatch[2].trim();
        const execTool: ToolExecution = {
          id: `tool-exec-${Date.now()}`,
          toolName: `Run ${cmd}`,
          command: cmd,
          status: 'running'
        };
        callbacks.onToolStart(execTool);

        const res = await this.executeTool('bash', { command: cmd }, context, callbacks);
        execTool.output = res.output;
        execTool.status = 'completed';
        callbacks.onToolComplete(execTool);

        const outMarkdown = `\`\`\`bash\n$ ${cmd}\n${res.output}\n\`\`\``;
        callbacks.onFinish(outMarkdown);
        return;
      }

      // 4. Direct File Reading Request (e.g. "view app.go", "read Makefile")
      const readMatch = trimmedPrompt.match(/^(read|view|show|cat|open)\s+([a-zA-Z0-9_\-\.\/]+)$/i);
      if (readMatch) {
        const targetPath = readMatch[2].trim();
        const targetFile = allFiles.find(f => f.path === targetPath || f.name === targetPath || f.path.endsWith(targetPath));
        if (targetFile) {
          const content = await ApiBridge.readFile(targetFile.path);
          const readTool: ToolExecution = {
            id: `tool-read-${Date.now()}`,
            toolName: `Read ${targetFile.name}`,
            command: `read ${targetFile.path}`,
            output: `Read ${content.length} bytes`,
            status: 'completed'
          };
          callbacks.onToolStart(readTool);
          callbacks.onToolComplete(readTool);

          const ext = targetFile.name.split('.').pop() || '';
          const outMarkdown = `### \`${targetFile.path}\`\n\n` +
            `\`\`\`${ext}\n${content.slice(0, 8000)}\n\`\`\`\n\n` +
            `Total: ${content.split('\n').length} lines (${content.length} bytes).`;
          callbacks.onFinish(outMarkdown);
          return;
        }
      }

      // 5. Retrieve user-configured providers from localStorage
      let providersConfig: LLMProviderConfig[] = [];
      try {
        const raw = localStorage.getItem('forge_ade_providers') || localStorage.getItem('my_ade_providers');
        if (raw) providersConfig = JSON.parse(raw);
      } catch {}

      if (!providersConfig || providersConfig.length === 0) {
        providersConfig = DEFAULT_PROVIDERS;
      }

      const enabledProviders = providersConfig.filter(p => p.enabled);

      // 6. Handle Internal Agent Execution with Full Multi-Turn Tool Calling (Pi Agent Core)
      if (agent.type === 'internal' || agent.id === 'agent-internal') {
        if (enabledProviders.length === 0) {
          const msg = `⚠️ **No LLM Provider Configured**\n\n` +
            `To enable reasoning with **ForgeADE Internal**, please go to **Settings > Providers & API Keys** and add your LLM provider:\n` +
            `• **Local Ollama**: Base URL \`http://localhost:11434\`\n` +
            `• **OpenAI / OpenRouter / Compatible**: Base URL & API Key\n` +
            `• **Google Gemini**: API Key\n` +
            `• **Anthropic Claude**: API Key`;
          callbacks.onFinish(msg);
          return;
        }

        let targetModel = context.activeModel || agent.model || enabledProviders[0].models?.[0] || 'default';
        const targetProvider = enabledProviders.find(p => p.models?.includes(targetModel) || p.selectedModels?.includes(targetModel)) || enabledProviders[0];

        // Ensure targetModel is genuinely supported by targetProvider
        const providerValidModels = (targetProvider.selectedModels && targetProvider.selectedModels.length > 0) ? targetProvider.selectedModels : (targetProvider.models || []);
        if (providerValidModels.length > 0 && !providerValidModels.includes(targetModel)) {
          targetModel = providerValidModels[0];
        }

        // Format System Instructions with Pi Toolset Specifications
        const fileOverview = allFiles.slice(0, 30).map(f => `- ${f.path}`).join('\n');
        const attachedContext = (context.attachedFiles || []).map(a => `[Attached file: ${a.name}]\n${a.content}`).join('\n\n');
        const skillsContext = activeSkills.filter(s => s.enabled).map(s => `Skill: ${s.name}\n${s.description}\nInstructions: ${s.instructions || ''}`).join('\n\n');
        const mcpsContext = activeMcps.filter(m => m.enabled).map(m => `MCP Server: ${m.name} (tools: ${(m.tools || []).join(', ')})`).join('\n');

        const systemPrompt = `You are ForgeADE, an autonomous software engineering assistant working in workspace: "${context.workspacePath}".
You have full access to these tools to inspect the codebase, edit files, and execute commands:

<tools>
1. read: {"path": string, "startLine"?: number, "endLine"?: number} - View file contents
2. write: {"path": string, "content": string} - Write full file
3. edit: {"path": string, "targetContent": string, "replacementContent": string} - Replace exact snippet in file
4. bash: {"command": string} - Run zsh command in workspace
5. grep: {"query": string, "path"?: string} - Search pattern with ripgrep
6. find: {"pattern": string} - Find files matching pattern
7. ls: {"path"?: string} - List directory contents
</tools>

HOW TO CALL TOOLS:
When you need to use a tool, output a tool call in this format:
<tool_call name="tool_name">
{"param": "value"}
</tool_call>

Workspace Context:
${fileOverview}

${attachedContext ? `Attached User Files:\n${attachedContext}\n` : ''}
${skillsContext ? `Active Skills:\n${skillsContext}\n` : ''}
${mcpsContext ? `Active MCPs:\n${mcpsContext}\n` : ''}

CRITICAL RULES:
1. CONVERSATIONAL QUERIES: If the user is having a conversation, greeting ("hi", "halo"), asking about previous messages ("what did I ask before?"), or asking for general explanations, answer DIRECTLY from the chat context. NEVER run tools to inspect internal session history files on disk (never search .forge-ade/sessions).
2. CODEBASE LISTING & SEARCH: When asked to list or explore files, NEVER search inside "node_modules", ".git", "dist", or "build". Focus only on source code files.
3. SUMMARIZE RESULTS: Always provide a well-structured, clear Markdown answer explaining your findings.`;

        // Build multi-turn conversation history from actual session messages
        const conversation: { role: string; content: string }[] = [];

        if (context.messages && context.messages.length > 0) {
          const past = context.messages.slice(-12);
          for (let i = 0; i < past.length; i++) {
            const m = past[i];
            if (m.role === 'user' && m.content) {
              conversation.push({ role: 'user', content: m.content });
            } else if (m.role === 'agent' && m.content && !m.content.startsWith('⚠️')) {
              const clean = AgentEngine.sanitizeModelOutput(m.content);
              if (clean) {
                conversation.push({ role: 'assistant', content: clean });
              }
            }
          }
        }

        // Ensure current prompt is at the end of conversation
        if (conversation.length === 0 || conversation[conversation.length - 1].content !== trimmedPrompt) {
          conversation.push({ role: 'user', content: trimmedPrompt });
        }

        let turnCount = 0;
        const maxTurns = 8;
        let finalResponseText = '';
        const executedToolsSummaries: string[] = [];

        while (turnCount < maxTurns && !this.isAborted) {
          turnCount++;

          // 6A. Call LLM with real-time streaming (chat, thinking & tool generation)
          let assistantReply = '';

          try {
            assistantReply = await this.streamLLMResponse(
              targetProvider,
              targetModel,
              systemPrompt,
              conversation,
              callbacks,
              turnCount
            );
          } catch (fetchErr: any) {
            if (targetProvider.baseUrl?.includes('11434') || targetProvider.name?.toLowerCase().includes('ollama')) {
              callbacks.onFinish(
                `⚠️ **Ollama is not reachable at ${targetProvider.baseUrl || 'http://localhost:11434'}**\n\n` +
                `To run local models:\n` +
                `1. Start Ollama in your terminal: \`ollama serve\`\n` +
                `2. Pull your model: \`ollama pull ${targetModel || 'qwen2.5-coder'}\`\n\n` +
                `Or configure an API key for Claude, GPT-4o, or Gemini in **Settings > Providers & Models**.`
              );
              return;
            }
            callbacks.onError(`Provider Error (${targetProvider.name}): ${fetchErr.message || fetchErr}`);
            return;
          }

          if (!assistantReply.trim()) break;

          // Check if assistant reply contains tool calls
          const toolCalls = this.parseToolCalls(assistantReply);

          if (toolCalls.length === 0) {
            // No tools called; clean up special tokens and treat as final answer!
            finalResponseText = AgentEngine.sanitizeModelOutput(assistantReply);
            break;
          }

          // Execute each tool call with live streaming status
          let toolResultsCombined = '';
          for (const call of toolCalls) {
            const toolExec: ToolExecution = {
              id: `tool-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
              toolName: `${call.name} ${call.args.path || call.args.command || call.args.query || ''}`.trim(),
              command: JSON.stringify(call.args),
              status: 'running'
            };
            callbacks.onToolStart(toolExec);

            try {
              const result = await this.executeTool(call.name, call.args, context, callbacks);

              toolExec.status = 'completed';
              toolExec.output = result.output;
              toolExec.diff = result.diff;
              callbacks.onToolComplete(toolExec);

              executedToolsSummaries.push(`**${call.name}** \`${call.args.path || call.args.command || call.args.query || ''}\`:\n\`\`\`\n${result.output}\n\`\`\``);
              toolResultsCombined += `Tool: ${call.name}\nArguments: ${JSON.stringify(call.args)}\nResult:\n${result.output}\n\n`;
            } catch (toolErr: any) {
              toolExec.status = 'failed';
              toolExec.output = `Error executing ${call.name}: ${toolErr.message || toolErr}`;
              callbacks.onToolComplete(toolExec);
              toolResultsCombined += `Tool: ${call.name}\nError:\n${toolErr.message || toolErr}\n\n`;
            }
          }

          // Append assistant message and tool outputs to conversation history for the next turn
          conversation.push({ role: 'assistant', content: assistantReply });
          conversation.push({
            role: 'user',
            content: `[TOOL EXECUTION RESULTS]\n${toolResultsCombined}\n\nPlease analyze the results above and provide a complete, clear, and well-structured response in GitHub markdown to answer the user's request.`
          });
        }

        let cleanFinal = AgentEngine.sanitizeModelOutput(finalResponseText);
        if (!cleanFinal || cleanFinal.length < 10) {
          if (executedToolsSummaries.length > 0) {
            cleanFinal = `Here are the results of the executed tools:\n\n` + executedToolsSummaries.join('\n\n');
          } else {
            cleanFinal = 'Task completed successfully.';
          }
        }
        callbacks.onFinish(cleanFinal);
        return;
      }

      // 7. Handle ACP Protocol Agents (Pi, OhMyPi, OpenCode)
      const handshake = await ApiBridge.handshakeACP(agent);
      if (!handshake.connected) {
        const errorMsg = `⚠️ **${agent.name} is Offline / Disconnected**\n\n` +
          `• **Agent Protocol**: ACP (${agent.type})\n` +
          `• **Endpoint**: \`${agent.endpoint || 'stdio'}\`\n` +
          `• **Handshake Error**: ${handshake.error || 'Server daemon not responding'}\n\n` +
          `To use this agent, please ensure the ACP server daemon is running, or switch to **ForgeADE Internal** in the agent dropdown.`;
        callbacks.onFinish(errorMsg);
        return;
      }

      // 8. If ACP connected via Pi Core
      if (handshake.connected && agent.type === 'pi') {
        const res = await ApiBridge.executeCommand(`pi "${trimmedPrompt}"`, context.workspacePath);
        callbacks.onFinish(res.stdout || res.stderr || 'Pi agent execution finished.');
        return;
      }

      callbacks.onFinish(`Agent ${agent.name} executed successfully.`);

    } catch (err: any) {
      callbacks.onError(err.message || 'Execution failed');
    }
  }

  private async streamLLMResponse(
    targetProvider: any,
    targetModel: string,
    systemPrompt: string,
    conversation: { role: string; content: string }[],
    callbacks: AgentExecutionCallbacks,
    turnCount: number
  ): Promise<string> {
    let fullAccumulated = '';
    let thinkAccumulated = '';
    let isInsideThinkTag = false;
    let didEmitChatChunk = false;
    const thoughtId = `thought-${Date.now()}-${turnCount}`;
    const startMs = Date.now();

    const handleDelta = (contentChunk: string, reasoningChunk?: string) => {
      if (this.isAborted) return;

      // 1. Direct reasoning stream from API (e.g. DeepSeek-R1 / Claude 3.7 / o1 / Gemini thought)
      if (reasoningChunk) {
        thinkAccumulated += reasoningChunk;
        const dur = Math.max(1, Math.round((Date.now() - startMs) / 1000));
        callbacks.onThought({
          id: thoughtId,
          thoughtText: thinkAccumulated,
          durationSeconds: dur,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        });
        return;
      }

      if (!contentChunk) return;
      fullAccumulated += contentChunk;

      // 2. Inline <think> tag handling (e.g. DeepSeek / Qwen / Ollama models outputting <think>...</think>)
      if (fullAccumulated.includes('<think>') && !fullAccumulated.includes('</think>')) {
        isInsideThinkTag = true;
        const afterOpen = fullAccumulated.split('<think>')[1] || '';
        thinkAccumulated = afterOpen;
        const dur = Math.max(1, Math.round((Date.now() - startMs) / 1000));
        callbacks.onThought({
          id: thoughtId,
          thoughtText: thinkAccumulated,
          durationSeconds: dur,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        });
        return;
      }

      if (fullAccumulated.includes('</think>') && isInsideThinkTag) {
        isInsideThinkTag = false;
        const inside = (fullAccumulated.match(/<think>([\s\S]*?)<\/think>/) || [])[1] || thinkAccumulated;
        thinkAccumulated = inside;
        const dur = Math.max(1, Math.round((Date.now() - startMs) / 1000));
        callbacks.onThought({
          id: thoughtId,
          thoughtText: thinkAccumulated,
          durationSeconds: dur,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        });

        // Emit text that comes after </think>
        const afterClose = fullAccumulated.split('</think>')[1] || '';
        if (afterClose && !didEmitChatChunk) {
          didEmitChatChunk = true;
          callbacks.onContentChunk(afterClose);
        }
        return;
      }

      // 3. Normal chat chunk streaming (if not inside tool call block or think tag)
      if (!isInsideThinkTag) {
        const isToolBlock = fullAccumulated.includes('<tool_call') ||
          fullAccumulated.includes('<function_call') ||
          fullAccumulated.includes('<|channel|>') ||
          fullAccumulated.includes('<|start|>') ||
          fullAccumulated.includes('```tool') ||
          fullAccumulated.includes('to=');
        if (!isToolBlock) {
          callbacks.onContentChunk(contentChunk.replace(/<\/think>/g, ''));
        }
      }
    };

    // Dispatch based on provider
    if (targetProvider.id === 'google' || targetProvider.baseUrl?.includes('googleapis.com') || (!targetProvider.baseUrl && targetProvider.apiKey?.startsWith('AIza'))) {
      if (!targetProvider.apiKey) {
        throw new Error('Gemini API Key is missing. Please set it in Settings > Providers & Models.');
      }
      const modelName = targetModel.includes('gemini') ? targetModel : 'gemini-2.0-flash';
      const endpoint = `https://generativelanguage.googleapis.com/v1beta/models/${modelName}:streamGenerateContent?alt=sse&key=${targetProvider.apiKey}`;
      const promptContent = `${systemPrompt}\n\n` + conversation.map(m => `${m.role === 'user' ? 'User' : 'Assistant'}: ${m.content}`).join('\n\n') + '\n\nAssistant:';

      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          contents: [{ parts: [{ text: promptContent }] }]
        })
      });

      if (!res.ok) {
        const err = await res.text();
        throw new Error(`Gemini API Error: ${err}`);
      }

      const reader = res.body?.getReader();
      if (reader) {
        const decoder = new TextDecoder();
        let buffer = '';
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';
          for (const line of lines) {
            const trimmed = line.trim();
            if (trimmed.startsWith('data: ')) {
              try {
                const json = JSON.parse(trimmed.slice(6));
                const parts = json.candidates?.[0]?.content?.parts || [];
                for (const p of parts) {
                  if (p.text) handleDelta(p.text, p.thought ? p.text : undefined);
                }
              } catch {}
            }
          }
        }
      }
    } else if (targetProvider.id === 'anthropic' || targetProvider.baseUrl?.includes('anthropic.com')) {
      if (!targetProvider.apiKey) {
        throw new Error('Anthropic API Key is missing. Please set it in Settings > Providers & Models.');
      }
      const res = await fetch('https://api.anthropic.com/v1/messages', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'x-api-key': targetProvider.apiKey,
          'anthropic-version': '2023-06-01'
        },
        body: JSON.stringify({
          model: targetModel,
          max_tokens: 4096,
          stream: true,
          system: systemPrompt,
          messages: conversation
        })
      });

      if (!res.ok) {
        const err = await res.text();
        throw new Error(`Anthropic API Error (${res.status}): ${err}`);
      }

      const reader = res.body?.getReader();
      if (reader) {
        const decoder = new TextDecoder();
        let buffer = '';
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';
          for (const line of lines) {
            const trimmed = line.trim();
            if (trimmed.startsWith('data: ')) {
              try {
                const json = JSON.parse(trimmed.slice(6));
                if (json.type === 'content_block_delta') {
                  if (json.delta?.type === 'thinking_delta') {
                    handleDelta('', json.delta.thinking);
                  } else if (json.delta?.type === 'text_delta') {
                    handleDelta(json.delta.text);
                  }
                }
              } catch {}
            }
          }
        }
      }
    } else {
      // Ollama / OpenAI / OpenRouter / DeepSeek / Groq compatible
      let cleanBase = (targetProvider.baseUrl || 'http://localhost:11434').replace(/\/$/, '');
      if (cleanBase.includes('11434') && !cleanBase.includes('/v1') && !cleanBase.includes('/api')) {
        cleanBase = `${cleanBase}/v1`;
      }
      const chatEndpoint = cleanBase.endsWith('/chat/completions') ? cleanBase : `${cleanBase}/chat/completions`;

      const res = await fetch(chatEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(targetProvider.apiKey ? { Authorization: `Bearer ${targetProvider.apiKey}` } : {})
        },
        body: JSON.stringify({
          model: targetModel,
          stream: true,
          messages: [
            { role: 'system', content: systemPrompt },
            ...conversation
          ]
        })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(`LLM Provider (${targetProvider.name}) Error (${res.status}): ${errText}`);
      }

      const reader = res.body?.getReader();
      if (reader) {
        const decoder = new TextDecoder();
        let buffer = '';
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';
          for (const line of lines) {
            const trimmed = line.trim();
            if (trimmed.startsWith('data: ')) {
              const dataStr = trimmed.slice(6);
              if (dataStr === '[DONE]') continue;
              try {
                const json = JSON.parse(dataStr);
                const delta = json.choices?.[0]?.delta || {};
                const token = delta.content || '';
                const reasoning = delta.reasoning_content || delta.reasoning || undefined;
                handleDelta(token, reasoning);
              } catch {}
            } else if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
              try {
                const json = JSON.parse(trimmed);
                const token = json.message?.content || json.response || '';
                handleDelta(token);
              } catch {}
            }
          }
        }
      }
    }

    return fullAccumulated;
  }
}

