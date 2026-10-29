import {
  GetGitCommitGraph as WailsGetGitCommitGraph,
  GetGitFileDiff as WailsGetGitFileDiff,
  GetGitCommitDiff as WailsGetGitCommitDiff,
  GetGitCommitFileDiff as WailsGetGitCommitFileDiff,
  GitStage as WailsGitStage,
  GitUnstage as WailsGitUnstage,
  GitDiscard as WailsGitDiscard,
  GitCheckIgnored as WailsGitCheckIgnored,
  GitCommit as WailsGitCommit,
  GitPush as WailsGitPush,
  GitPull as WailsGitPull,
  GitFetch as WailsGitFetch,
  GetGitStatus as WailsGetGitStatus,
  GetGitBranches as WailsGetGitBranches,
  GitCheckout as WailsGitCheckout,
  GitCheckoutNewBranch as WailsGitCheckoutNewBranch,
  GenerateAICommitMessage as WailsGenerateAICommitMessage,
  GetPullRequests as WailsGetPullRequests,
  GetPullRequestDetail as WailsGetPullRequestDetail,
  GetPullRequestDiff as WailsGetPullRequestDiff,
  CreatePullRequestComment as WailsCreatePullRequestComment,
  ReviewPullRequest as WailsReviewPullRequest,
  MergePullRequest as WailsMergePullRequest,
  ClosePullRequest as WailsClosePullRequest,
  ReopenPullRequest as WailsReopenPullRequest,
  CheckoutPullRequest as WailsCheckoutPullRequest,
  CreatePullRequest as WailsCreatePullRequest,
} from '../../lib/wails';

export interface PullRequestAuthor {
  login: string;
  name: string;
}

export interface PullRequestFile {
  path: string;
  additions: number;
  deletions: number;
  changeType: string;
}

export interface PullRequestComment {
  id: string;
  author: PullRequestAuthor;
  body: string;
  createdAt: string;
}

export interface PullRequestReview {
  id: string;
  author: PullRequestAuthor;
  state: string; // APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING
  body: string;
  submittedAt: string;
}

export interface PullRequestItem {
  number: number;
  title: string;
  state: 'OPEN' | 'CLOSED' | 'MERGED' | string;
  author: PullRequestAuthor;
  headRefName: string;
  baseRefName: string;
  createdAt: string;
  updatedAt: string;
  isDraft: boolean;
  url: string;
  additions: number;
  deletions: number;
  changedFiles: number;
  commentsCount: number;
  reviewsCount: number;
  latestReviews?: PullRequestReview[];
}

export interface PullRequestDetail {
  number: number;
  title: string;
  state: 'OPEN' | 'CLOSED' | 'MERGED' | string;
  body: string;
  author: PullRequestAuthor;
  headRefName: string;
  baseRefName: string;
  createdAt: string;
  updatedAt: string;
  isDraft: boolean;
  url: string;
  additions: number;
  deletions: number;
  changedFiles: number;
  mergeable: string;
  files: PullRequestFile[];
  comments: PullRequestComment[];
  reviews: PullRequestReview[];
}

export interface GitFileStatus {
  path: string;
  status: string;
  staging?: 'staged' | 'unstaged' | 'untracked';
  dir?: string;
  additions?: number;
  deletions?: number;
}

export interface GitStatusResult {
  branch: string;
  files: GitFileStatus[];
}

export class GitApi {
  public static async gitStatus(cwd?: string): Promise<GitStatusResult> {
    try {
      const res = await WailsGetGitStatus(cwd || '');
      if (res && typeof res === 'object') {
        const branch = res.branch || 'main';
        const files: GitFileStatus[] = [];
        if (Array.isArray(res.staged)) {
          res.staged.forEach((f: any) =>
            files.push({
              path: f.path || f,
              status: f.status || 'A',
              staging: 'staged',
              dir: f.dir,
              additions: f.additions,
              deletions: f.deletions,
            })
          );
        }
        if (Array.isArray(res.unstaged)) {
          res.unstaged.forEach((f: any) =>
            files.push({
              path: f.path || f,
              status: f.status || 'M',
              staging: 'unstaged',
              dir: f.dir,
              additions: f.additions,
              deletions: f.deletions,
            })
          );
        }
        if (Array.isArray(res.untracked)) {
          res.untracked.forEach((f: any) =>
            files.push({
              path: f.path || f,
              status: f.status || '?',
              staging: 'untracked',
              dir: f.dir,
              additions: f.additions,
              deletions: f.deletions,
            })
          );
        }
        if (Array.isArray(res.conflicts)) {
          res.conflicts.forEach((f: any) =>
            files.push({
              path: f.path || f,
              status: f.status || 'U',
              staging: 'unstaged',
              dir: f.dir,
              additions: f.additions,
              deletions: f.deletions,
            })
          );
        }
        return { branch, files };
      }
    } catch (err) {
      console.warn('Wails GetGitStatus error:', err);
    }
    return { branch: '', files: [] };
  }

  public static async getGitBranches(cwd?: string): Promise<string[]> {
    const branches = await WailsGetGitBranches(cwd || '');
    return Array.isArray(branches) ? branches : [];
  }

  public static async gitCheckout(repoPathOrBranch: string, branchOrRepoPath?: string): Promise<void> {
    let cwd = '';
    let branch = '';
    if (branchOrRepoPath !== undefined) {
      if (repoPathOrBranch.startsWith('/') || repoPathOrBranch.includes('\\') || repoPathOrBranch === '.') {
        cwd = repoPathOrBranch;
        branch = branchOrRepoPath;
      } else {
        branch = repoPathOrBranch;
        cwd = branchOrRepoPath;
      }
    } else {
      branch = repoPathOrBranch;
    }
    await WailsGitCheckout(cwd, branch);
  }

  public static async gitCreateBranch(repoPath: string, name: string): Promise<void> {
    await WailsGitCheckoutNewBranch(repoPath || '', name);
  }

  public static async gitCheckoutNewBranch(branch: string, cwd?: string): Promise<void> {
    await WailsGitCheckoutNewBranch(cwd || '', branch);
  }

  public static async gitDiff(filePath: string, cwd?: string, _staged?: boolean): Promise<string> {
    try {
      const diff = await WailsGetGitFileDiff(cwd || '', filePath);
      return typeof diff === 'string' ? diff : '';
    } catch {
      return '';
    }
  }

  public static async gitCommitFileDiff(hash: string, filePath: string, cwd?: string): Promise<string> {
    const cleanPath = (filePath || '').replace(/^[ab]\//, '').trim();
    try {
      const diff = await WailsGetGitCommitFileDiff(cwd || '', hash, cleanPath);
      return typeof diff === 'string' ? diff : '';
    } catch {
      return '';
    }
  }

  public static async gitCommitDiff(hash: string, cwd?: string): Promise<string> {
    try {
      const diff = await WailsGetGitCommitDiff(cwd || '', hash);
      return typeof diff === 'string' ? diff : '';
    } catch {
      return '';
    }
  }

  public static async gitLog(cwd?: string, limit = 50): Promise<any[]> {
    try {
      const graph = await WailsGetGitCommitGraph(cwd || '', 0, limit, '');
      if (graph && Array.isArray(graph.commits)) {
        return graph.commits.map((c: any) => ({
          hash: c.hash,
          short_hash: c.short_hash,
          parents: c.parents || [],
          message: c.message,
          author: c.author_name || c.author_email || 'Unknown',
          timestamp: c.timestamp,
          graph_prefix: c.graph_prefix || '',
          decorations: c.decorations || '',
          status: c.status || '',
        }));
      }
    } catch (err) {
      console.warn('gitLog error:', err);
    }
    return [];
  }

  public static async gitStage(filePath: string, cwd?: string): Promise<boolean> {
    try {
      await WailsGitStage(cwd || '', [filePath]);
      return true;
    } catch (err) {
      console.error('gitStage failed:', err);
      return false;
    }
  }

  public static async gitUnstage(filePath: string, cwd?: string): Promise<boolean> {
    try {
      await WailsGitUnstage(cwd || '', [filePath]);
      return true;
    } catch (err) {
      console.error('gitUnstage failed:', err);
      return false;
    }
  }

  public static async gitDiscard(filePath: string, cwd?: string): Promise<boolean> {
    try {
      await WailsGitDiscard(cwd || '', [filePath]);
      return true;
    } catch (err) {
      console.error('gitDiscard failed:', err);
      return false;
    }
  }

  public static async gitCheckIgnored(paths: string[], cwd?: string): Promise<string[]> {
    try {
      const res = await WailsGitCheckIgnored(cwd || '', paths);
      return Array.isArray(res) ? res : [];
    } catch {
      return [];
    }
  }

  public static async gitCommit(message: string, cwd?: string): Promise<{ success: boolean; output?: string }> {
    try {
      const output = await WailsGitCommit(cwd || '', message);
      return { success: true, output: typeof output === 'string' ? output : undefined };
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      return { success: false, output: msg };
    }
  }

  public static async gitAiCommitMessage(cwd?: string): Promise<{ message: string; files: string[] }> {
    try {
      const msg = await WailsGenerateAICommitMessage(cwd || '', '', '', '');
      if (msg && typeof msg === 'string' && msg.trim() !== '') {
        return { message: msg.trim(), files: [] };
      }
    } catch (err) {
      console.warn('gitAiCommitMessage error:', err);
    }
    return { message: '', files: [] };
  }

  public static async gitPush(_branch = 'main', cwd?: string): Promise<{ success: boolean; output?: string }> {
    try {
      const output = await WailsGitPush(cwd || '');
      return { success: true, output: typeof output === 'string' ? output : undefined };
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      return { success: false, output: msg };
    }
  }

  public static async gitFetch(cwd?: string): Promise<{ success: boolean }> {
    try {
      await WailsGitFetch(cwd || '');
      return { success: true };
    } catch (err) {
      console.error('gitFetch failed:', err);
      return { success: false };
    }
  }

  public static async gitPull(cwd?: string): Promise<{ success: boolean; output?: string }> {
    try {
      const output = await WailsGitPull(cwd || '');
      return { success: true, output: typeof output === 'string' ? output : undefined };
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      return { success: false, output: msg };
    }
  }

  public static async getGitCommitGraph(cwd?: string, offset = 0, limit = 50, branch = ''): Promise<any> {
    return WailsGetGitCommitGraph(cwd || '', offset, limit, branch);
  }

  // ── Pull Requests / Merge Requests ─────────────────────────────────────────

  public static async getPullRequests(cwd?: string, state?: string): Promise<PullRequestItem[]> {
    return WailsGetPullRequests(cwd || '', state || 'all');
  }

  public static async getPullRequestDetail(prNumber: number, cwd?: string): Promise<PullRequestDetail | null> {
    return WailsGetPullRequestDetail(cwd || '', prNumber);
  }

  public static async getPullRequestDiff(prNumber: number, cwd?: string): Promise<string> {
    return WailsGetPullRequestDiff(cwd || '', prNumber);
  }

  public static async createPullRequestComment(prNumber: number, body: string, cwd?: string): Promise<void> {
    return WailsCreatePullRequestComment(cwd || '', prNumber, body);
  }

  public static async reviewPullRequest(
    prNumber: number,
    action: 'approve' | 'request-changes' | 'comment',
    body?: string,
    cwd?: string
  ): Promise<void> {
    return WailsReviewPullRequest(cwd || '', prNumber, action, body || '');
  }

  public static async mergePullRequest(
    prNumber: number,
    method: 'merge' | 'squash' | 'rebase' = 'merge',
    cwd?: string
  ): Promise<string> {
    return WailsMergePullRequest(cwd || '', prNumber, method);
  }

  public static async closePullRequest(prNumber: number, cwd?: string): Promise<void> {
    return WailsClosePullRequest(cwd || '', prNumber);
  }

  public static async reopenPullRequest(prNumber: number, cwd?: string): Promise<void> {
    return WailsReopenPullRequest(cwd || '', prNumber);
  }

  public static async checkoutPullRequest(prNumber: number, cwd?: string): Promise<string> {
    return WailsCheckoutPullRequest(cwd || '', prNumber);
  }

  public static async createPullRequest(
    params: { title: string; body: string; base?: string; head?: string; draft?: boolean },
    cwd?: string
  ): Promise<any> {
    return WailsCreatePullRequest(cwd || '', params.title, params.body, params.base, params.head, params.draft);
  }
}
