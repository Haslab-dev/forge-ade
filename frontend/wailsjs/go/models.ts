export namespace git {
	
	export class Branch {
	    name: string;
	    isHead: boolean;
	    isRemote: boolean;
	    isActive: boolean;
	    commitHash: string;
	
	    static createFrom(source: any = {}) {
	        return new Branch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.isHead = source["isHead"];
	        this.isRemote = source["isRemote"];
	        this.isActive = source["isActive"];
	        this.commitHash = source["commitHash"];
	    }
	}
	export class Commit {
	    hash: string;
	    author: string;
	    email: string;
	    message: string;
	    timestamp: number;
	    parents: number;
	
	    static createFrom(source: any = {}) {
	        return new Commit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.author = source["author"];
	        this.email = source["email"];
	        this.message = source["message"];
	        this.timestamp = source["timestamp"];
	        this.parents = source["parents"];
	    }
	}
	export class CommitGraphEntry {
	    graphLine: string;
	    hash: string;
	    subject: string;
	    author: string;
	    refs: string;
	    timestamp: number;
	
	    static createFrom(source: any = {}) {
	        return new CommitGraphEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.graphLine = source["graphLine"];
	        this.hash = source["hash"];
	        this.subject = source["subject"];
	        this.author = source["author"];
	        this.refs = source["refs"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class DiffLine {
	    type: string;
	    content: string;
	    oldLine: number;
	    newLine: number;
	
	    static createFrom(source: any = {}) {
	        return new DiffLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.content = source["content"];
	        this.oldLine = source["oldLine"];
	        this.newLine = source["newLine"];
	    }
	}
	export class DiffHunk {
	    oldStart: number;
	    oldCount: number;
	    newStart: number;
	    newCount: number;
	    header: string;
	    lines: DiffLine[];
	
	    static createFrom(source: any = {}) {
	        return new DiffHunk(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldStart = source["oldStart"];
	        this.oldCount = source["oldCount"];
	        this.newStart = source["newStart"];
	        this.newCount = source["newCount"];
	        this.header = source["header"];
	        this.lines = this.convertValues(source["lines"], DiffLine);
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
	
	export class FileDiff {
	    oldPath: string;
	    newPath: string;
	    hunks: DiffHunk[];
	
	    static createFrom(source: any = {}) {
	        return new FileDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldPath = source["oldPath"];
	        this.newPath = source["newPath"];
	        this.hunks = this.convertValues(source["hunks"], DiffHunk);
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

export namespace main {
	
	export class WorkspaceInfo {
	    name: string;
	    folders: string[];
	    isTemporary: boolean;
	    filePath?: string;
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.folders = source["folders"];
	        this.isTemporary = source["isTemporary"];
	        this.filePath = source["filePath"];
	        this.theme = source["theme"];
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

