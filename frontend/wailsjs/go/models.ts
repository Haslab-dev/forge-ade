export namespace agent {
	
	export class AgentDefinition {
	    id: string;
	    name: string;
	    description?: string;
	    role_filter?: string;
	    model?: string;
	    prompt?: string;
	    rules?: string;
	    color?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new AgentDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.role_filter = source["role_filter"];
	        this.model = source["model"];
	        this.prompt = source["prompt"];
	        this.rules = source["rules"];
	        this.color = source["color"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ContentBlock {
	    type: string;
	    text?: string;
	    tool_call_id?: string;
	    name?: string;
	    arguments?: Record<string, any>;
	    is_error?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContentBlock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.text = source["text"];
	        this.tool_call_id = source["tool_call_id"];
	        this.name = source["name"];
	        this.arguments = source["arguments"];
	        this.is_error = source["is_error"];
	    }
	}
	export class AgentMessage {
	    id: string;
	    role: string;
	    content: ContentBlock[];
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new AgentMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.content = this.convertValues(source["content"], ContentBlock);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Observation {
	    source: string;
	    kind: string;
	    summary: string;
	    confidence?: number;
	
	    static createFrom(source: any = {}) {
	        return new Observation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.kind = source["kind"];
	        this.summary = source["summary"];
	        this.confidence = source["confidence"];
	    }
	}
	export class SessionProgress {
	    current_goal?: string;
	    completed_steps?: string[];
	    active_todos?: string[];
	
	    static createFrom(source: any = {}) {
	        return new SessionProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_goal = source["current_goal"];
	        this.completed_steps = source["completed_steps"];
	        this.active_todos = source["active_todos"];
	    }
	}
	export class TaskItem {
	    id: string;
	    title: string;
	    completed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TaskItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.completed = source["completed"];
	    }
	}
	export class Session {
	    id: string;
	    name: string;
	    role_filter: string;
	    state: string;
	    folder: string;
	    project_name?: string;
	    messages: AgentMessage[];
	    tasks: TaskItem[];
	    progress?: SessionProgress;
	    token_usage: llm.TokenStats;
	    auto_approve: boolean;
	    pending_tools?: ContentBlock[];
	    pending_questions?: tools.AskQuestion[];
	    dialect?: string;
	    system_prompt?: string;
	    custom_prompt?: string;
	    custom_rules?: string;
	    summary?: string;
	    observations?: Observation[];
	    // Go type: time
	    created_at?: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role_filter = source["role_filter"];
	        this.state = source["state"];
	        this.folder = source["folder"];
	        this.project_name = source["project_name"];
	        this.messages = this.convertValues(source["messages"], AgentMessage);
	        this.tasks = this.convertValues(source["tasks"], TaskItem);
	        this.progress = this.convertValues(source["progress"], SessionProgress);
	        this.token_usage = this.convertValues(source["token_usage"], llm.TokenStats);
	        this.auto_approve = source["auto_approve"];
	        this.pending_tools = this.convertValues(source["pending_tools"], ContentBlock);
	        this.pending_questions = this.convertValues(source["pending_questions"], tools.AskQuestion);
	        this.dialect = source["dialect"];
	        this.system_prompt = source["system_prompt"];
	        this.custom_prompt = source["custom_prompt"];
	        this.custom_rules = source["custom_rules"];
	        this.summary = source["summary"];
	        this.observations = this.convertValues(source["observations"], Observation);
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace git {
	
	export class CommitNode {
	    hash: string;
	    short_hash: string;
	    parents: string[];
	    author_name: string;
	    author_email: string;
	    // Go type: time
	    timestamp: any;
	    message: string;
	    graph_prefix: string;
	    decorations: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.short_hash = source["short_hash"];
	        this.parents = source["parents"];
	        this.author_name = source["author_name"];
	        this.author_email = source["author_email"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.message = source["message"];
	        this.graph_prefix = source["graph_prefix"];
	        this.decorations = source["decorations"];
	        this.status = source["status"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CommitGraphResult {
	    commits: CommitNode[];
	    total_count: number;
	    has_more: boolean;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new CommitGraphResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commits = this.convertValues(source["commits"], CommitNode);
	        this.total_count = source["total_count"];
	        this.has_more = source["has_more"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class DiffHunk {
	    oldStart: number;
	    oldLines: number;
	    newStart: number;
	    newLines: number;
	    header: string;
	    body: string[];
	
	    static createFrom(source: any = {}) {
	        return new DiffHunk(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldStart = source["oldStart"];
	        this.oldLines = source["oldLines"];
	        this.newStart = source["newStart"];
	        this.newLines = source["newLines"];
	        this.header = source["header"];
	        this.body = source["body"];
	    }
	}
	export class FileStatus {
	    path: string;
	    dir: string;
	    staging: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new FileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.dir = source["dir"];
	        this.staging = source["staging"];
	        this.status = source["status"];
	    }
	}
	export class GitStatusResult {
	    branch: string;
	    staged: FileStatus[];
	    unstaged: FileStatus[];
	    untracked: FileStatus[];
	    conflicts: FileStatus[];
	
	    static createFrom(source: any = {}) {
	        return new GitStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.staged = this.convertValues(source["staged"], FileStatus);
	        this.unstaged = this.convertValues(source["unstaged"], FileStatus);
	        this.untracked = this.convertValues(source["untracked"], FileStatus);
	        this.conflicts = this.convertValues(source["conflicts"], FileStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace index {
	
	export class Export {
	    FileID: number;
	    Name: string;
	    Line: number;
	    Column: number;
	
	    static createFrom(source: any = {}) {
	        return new Export(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FileID = source["FileID"];
	        this.Name = source["Name"];
	        this.Line = source["Line"];
	        this.Column = source["Column"];
	    }
	}
	export class Import {
	    FileID: number;
	    Path: string;
	    Names: string[];
	    Line: number;
	    Column: number;
	
	    static createFrom(source: any = {}) {
	        return new Import(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FileID = source["FileID"];
	        this.Path = source["Path"];
	        this.Names = source["Names"];
	        this.Line = source["Line"];
	        this.Column = source["Column"];
	    }
	}
	export class Symbol {
	    ID: string;
	    Name: string;
	    Kind: number;
	    FileID: number;
	    File: string;
	    Line: number;
	    Column: number;
	    EndLine: number;
	    EndColumn: number;
	    Scope: string;
	    Exported: boolean;
	    Module: string;
	
	    static createFrom(source: any = {}) {
	        return new Symbol(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Kind = source["Kind"];
	        this.FileID = source["FileID"];
	        this.File = source["File"];
	        this.Line = source["Line"];
	        this.Column = source["Column"];
	        this.EndLine = source["EndLine"];
	        this.EndColumn = source["EndColumn"];
	        this.Scope = source["Scope"];
	        this.Exported = source["Exported"];
	        this.Module = source["Module"];
	    }
	}

}

export namespace llm {
	
	export class Profile {
	    provider_id: string;
	    api_key: string;
	    base_url: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider_id = source["provider_id"];
	        this.api_key = source["api_key"];
	        this.base_url = source["base_url"];
	        this.model = source["model"];
	    }
	}
	export class ProviderConfig {
	    id: string;
	    name: string;
	    base_url: string;
	    env_key: string;
	    default_model: string;
	    requires_key: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.base_url = source["base_url"];
	        this.env_key = source["env_key"];
	        this.default_model = source["default_model"];
	        this.requires_key = source["requires_key"];
	    }
	}
	export class ProviderProfile {
	    id: string;
	    name: string;
	    api_key: string;
	    base_url: string;
	    enabled: boolean;
	    available_models: string[];
	    selected_models: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.api_key = source["api_key"];
	        this.base_url = source["base_url"];
	        this.enabled = source["enabled"];
	        this.available_models = source["available_models"];
	        this.selected_models = source["selected_models"];
	    }
	}
	export class TokenStats {
	    prompt_tokens: number;
	    completion_tokens: number;
	    cached_tokens: number;
	    prompt_cache_hit_tokens: number;
	    prompt_cache_miss_tokens: number;
	    total_tokens: number;
	
	    static createFrom(source: any = {}) {
	        return new TokenStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt_tokens = source["prompt_tokens"];
	        this.completion_tokens = source["completion_tokens"];
	        this.cached_tokens = source["cached_tokens"];
	        this.prompt_cache_hit_tokens = source["prompt_cache_hit_tokens"];
	        this.prompt_cache_miss_tokens = source["prompt_cache_miss_tokens"];
	        this.total_tokens = source["total_tokens"];
	    }
	}

}

export namespace main {
	
	export class SyntaxDiagnostic {
	    line: number;
	    column: number;
	    end_line?: number;
	    end_column?: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new SyntaxDiagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.column = source["column"];
	        this.end_line = source["end_line"];
	        this.end_column = source["end_column"];
	        this.message = source["message"];
	    }
	}

}

export namespace mcp {
	
	export class ServerConfig {
	    name: string;
	    command: string;
	    args: string[];
	    env: Record<string, string>;
	    type?: string;
	    url?: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.enabled = source["enabled"];
	    }
	}
	export class Tool {
	    server_name: string;
	    name: string;
	    description: string;
	    input_schema: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Tool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_name = source["server_name"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.input_schema = source["input_schema"];
	    }
	}

}

export namespace search {
	
	export class RankedResult {
	    path: string;
	    filename: string;
	    score: number;
	    line?: number;
	    content?: string;
	
	    static createFrom(source: any = {}) {
	        return new RankedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.filename = source["filename"];
	        this.score = source["score"];
	        this.line = source["line"];
	        this.content = source["content"];
	    }
	}
	export class ReplaceOptions {
	    query: string;
	    matchCase: boolean;
	    matchWholeWord: boolean;
	    useRegex: boolean;
	    glob?: string;
	    limit: number;
	    replacement: string;
	    preserveCase: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReplaceOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.matchCase = source["matchCase"];
	        this.matchWholeWord = source["matchWholeWord"];
	        this.useRegex = source["useRegex"];
	        this.glob = source["glob"];
	        this.limit = source["limit"];
	        this.replacement = source["replacement"];
	        this.preserveCase = source["preserveCase"];
	    }
	}
	export class ReplaceResult {
	    filesChanged: number;
	    totalReplacements: number;
	    files: string[];
	
	    static createFrom(source: any = {}) {
	        return new ReplaceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filesChanged = source["filesChanged"];
	        this.totalReplacements = source["totalReplacements"];
	        this.files = source["files"];
	    }
	}
	export class SearchOptions {
	    query: string;
	    matchCase: boolean;
	    matchWholeWord: boolean;
	    useRegex: boolean;
	    glob?: string;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.matchCase = source["matchCase"];
	        this.matchWholeWord = source["matchWholeWord"];
	        this.useRegex = source["useRegex"];
	        this.glob = source["glob"];
	        this.limit = source["limit"];
	    }
	}

}

export namespace skills {
	
	export class Skill {
	    name: string;
	    description: string;
	    path: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.path = source["path"];
	        this.body = source["body"];
	    }
	}

}

export namespace terminal {
	
	export class Session {
	    id: string;
	    name: string;
	    type: string;
	    provider: string;
	    folder: string;
	    command: string;
	    status: string;
	    pid: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.provider = source["provider"];
	        this.folder = source["folder"];
	        this.command = source["command"];
	        this.status = source["status"];
	        this.pid = source["pid"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace tools {
	
	export class AskQuestion {
	    id: string;
	    question: string;
	    header?: string;
	    options: string[];
	    multi?: boolean;
	    recommended?: number;
	
	    static createFrom(source: any = {}) {
	        return new AskQuestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.question = source["question"];
	        this.header = source["header"];
	        this.options = source["options"];
	        this.multi = source["multi"];
	        this.recommended = source["recommended"];
	    }
	}

}

export namespace usage {
	
	export class Bucket {
	    key: string;
	    requests: number;
	    input_tokens: number;
	    output_tokens: number;
	    cached_tokens: number;
	    cache_hit_rate: number;
	    cost_usd: number;
	    failures: number;
	    avg_latency_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new Bucket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.requests = source["requests"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cached_tokens = source["cached_tokens"];
	        this.cache_hit_rate = source["cache_hit_rate"];
	        this.cost_usd = source["cost_usd"];
	        this.failures = source["failures"];
	        this.avg_latency_ms = source["avg_latency_ms"];
	    }
	}
	export class DayPoint {
	    date: string;
	    requests: number;
	    input_tokens: number;
	    output_tokens: number;
	    cached_tokens: number;
	    cost_usd: number;
	    failures: number;
	
	    static createFrom(source: any = {}) {
	        return new DayPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.requests = source["requests"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cached_tokens = source["cached_tokens"];
	        this.cost_usd = source["cost_usd"];
	        this.failures = source["failures"];
	    }
	}
	export class FilterOptions {
	    workspaces: string[];
	    agents: string[];
	    providers: string[];
	    models: string[];
	
	    static createFrom(source: any = {}) {
	        return new FilterOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaces = source["workspaces"];
	        this.agents = source["agents"];
	        this.providers = source["providers"];
	        this.models = source["models"];
	    }
	}
	export class Overview {
	    requests: number;
	    input_tokens: number;
	    output_tokens: number;
	    cached_tokens: number;
	    cache_hit_rate: number;
	    latency_p95_ms: number;
	    avg_tool_calls: number;
	    cost_usd: number;
	
	    static createFrom(source: any = {}) {
	        return new Overview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requests = source["requests"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cached_tokens = source["cached_tokens"];
	        this.cache_hit_rate = source["cache_hit_rate"];
	        this.latency_p95_ms = source["latency_p95_ms"];
	        this.avg_tool_calls = source["avg_tool_calls"];
	        this.cost_usd = source["cost_usd"];
	    }
	}
	export class RequestRow {
	    id: string;
	    // Go type: time
	    timestamp: any;
	    workspace: string;
	    agent: string;
	    provider: string;
	    model: string;
	    input_tokens: number;
	    output_tokens: number;
	    cached_tokens: number;
	    latency_ms: number;
	    tool_calls: number;
	    cost_usd: number;
	    success: boolean;
	    retry_count: number;
	
	    static createFrom(source: any = {}) {
	        return new RequestRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.workspace = source["workspace"];
	        this.agent = source["agent"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cached_tokens = source["cached_tokens"];
	        this.latency_ms = source["latency_ms"];
	        this.tool_calls = source["tool_calls"];
	        this.cost_usd = source["cost_usd"];
	        this.success = source["success"];
	        this.retry_count = source["retry_count"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace workspace {
	
	export class AgentConfig {
	    provider: string;
	    model?: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	    }
	}
	export class GitConfig {
	    autoFetch: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GitConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoFetch = source["autoFetch"];
	    }
	}
	export class RecentEntry {
	    path: string;
	    name: string;
	    isWorkspace: boolean;
	    // Go type: time
	    lastOpened: any;
	    pinned: boolean;
	    favorite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RecentEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.isWorkspace = source["isWorkspace"];
	        this.lastOpened = this.convertValues(source["lastOpened"], null);
	        this.pinned = source["pinned"];
	        this.favorite = source["favorite"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	    }
	}
	export class TerminalConfig {
	    shell: string;
	    cwd?: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shell = source["shell"];
	        this.cwd = source["cwd"];
	    }
	}
	export class Workspace {
	    filePath?: string;
	    version: number;
	    name: string;
	    folders: string[];
	    settings: Settings;
	    git: GitConfig;
	    agents: Record<string, AgentConfig>;
	    terminals: Record<string, TerminalConfig>;
	    isTemporary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.version = source["version"];
	        this.name = source["name"];
	        this.folders = source["folders"];
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.git = this.convertValues(source["git"], GitConfig);
	        this.agents = this.convertValues(source["agents"], AgentConfig, true);
	        this.terminals = this.convertValues(source["terminals"], TerminalConfig, true);
	        this.isTemporary = source["isTemporary"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

