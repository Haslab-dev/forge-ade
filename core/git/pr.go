package git

// pr.go implements the pull-request / merge-request layer shared by the
// webview and native apps. It shells out to the gh / glab CLIs (which carry
// the user's authentication) and parses their JSON output. Ported from
// app/webview/internal/app/git_api.go so both delivery targets share one
// implementation.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PullRequestAuthor represents an author or reviewer on GitHub / GitLab.
type PullRequestAuthor struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// PullRequestFile represents a changed file within a pull request.
type PullRequestFile struct {
	Path       string `json:"path"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	ChangeType string `json:"changeType"`
}

// PullRequestComment represents a discussion comment on a pull request.
type PullRequestComment struct {
	ID        string            `json:"id"`
	Author    PullRequestAuthor `json:"author"`
	Body      string            `json:"body"`
	CreatedAt string            `json:"createdAt"`
}

// PullRequestReview represents a formal review submitted for a pull request.
type PullRequestReview struct {
	ID          string            `json:"id"`
	Author      PullRequestAuthor `json:"author"`
	State       string            `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING
	Body        string            `json:"body"`
	SubmittedAt string            `json:"submittedAt"`
}

// PullRequestItem represents a pull request summary in a list.
type PullRequestItem struct {
	Number        int                 `json:"number"`
	Title         string              `json:"title"`
	State         string              `json:"state"` // OPEN, CLOSED, MERGED
	Author        PullRequestAuthor   `json:"author"`
	HeadRefName   string              `json:"headRefName"`
	BaseRefName   string              `json:"baseRefName"`
	CreatedAt     string              `json:"createdAt"`
	UpdatedAt     string              `json:"updatedAt"`
	IsDraft       bool                `json:"isDraft"`
	URL           string              `json:"url"`
	Additions     int                 `json:"additions"`
	Deletions     int                 `json:"deletions"`
	ChangedFiles  int                 `json:"changedFiles"`
	CommentsCount int                 `json:"commentsCount"`
	ReviewsCount  int                 `json:"reviewsCount"`
	LatestReviews []PullRequestReview `json:"latestReviews,omitempty"`
}

// PullRequestDetail represents detailed information for a single pull request.
type PullRequestDetail struct {
	Number       int                  `json:"number"`
	Title        string               `json:"title"`
	State        string               `json:"state"`
	Body         string               `json:"body"`
	Author       PullRequestAuthor    `json:"author"`
	HeadRefName  string               `json:"headRefName"`
	BaseRefName  string               `json:"baseRefName"`
	CreatedAt    string               `json:"createdAt"`
	UpdatedAt    string               `json:"updatedAt"`
	IsDraft      bool                 `json:"isDraft"`
	URL          string               `json:"url"`
	Additions    int                  `json:"additions"`
	Deletions    int                  `json:"deletions"`
	ChangedFiles int                  `json:"changedFiles"`
	Mergeable    string               `json:"mergeable"`
	Files        []PullRequestFile    `json:"files"`
	Comments     []PullRequestComment `json:"comments"`
	Reviews      []PullRequestReview  `json:"reviews"`
}

// GitRemoteSource holds metadata about the detected remote host and repository.
type GitRemoteSource struct {
	Provider   string `json:"provider"`   // "github", "gitlab", "generic"
	RemoteName string `json:"remoteName"` // "origin", etc.
	RemoteURL  string `json:"remoteUrl"`  // raw remote URL
	Host       string `json:"host"`       // "github.com", "gitlab.com", "gitlab.mycompany.com"
	Owner      string `json:"owner"`      // "Haslab-dev", "mygroup"
	Repo       string `json:"repo"`       // "forge-ade", "myproject"
	WebURL     string `json:"webUrl"`     // "https://github.com/Haslab-dev/forge-ade"
	PullsURL   string `json:"pullsUrl"`   // web URL for PRs or MRs
	IsGitHub   bool   `json:"isGitHub"`
	IsGitLab   bool   `json:"isGitLab"`
	GhPath     string `json:"ghPath,omitempty"`
	GlabPath   string `json:"glabPath,omitempty"`
}

// resolveGitCLI locates an executable (gh, glab, git) searching system PATH,
// custom env overrides, and enriched directories (Homebrew, Linuxbrew,
// ~/.local/bin, Nix, etc.).
func resolveGitCLI(name string) (string, error) {
	if name == "gh" {
		if p := os.Getenv("FORGE_GH_PATH"); p != "" {
			if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
				return p, nil
			}
		}
		if p := os.Getenv("GH_PATH"); p != "" {
			if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
				return p, nil
			}
		}
	} else if name == "glab" {
		if p := os.Getenv("FORGE_GLAB_PATH"); p != "" {
			if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
				return p, nil
			}
		}
		if p := os.Getenv("GLAB_PATH"); p != "" {
			if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
				return p, nil
			}
		}
	}

	for _, dir := range filepath.SplitList(EnrichedPath()) {
		if dir == "" {
			continue
		}
		cand := filepath.Join(dir, name)
		if stat, err := os.Stat(cand); err == nil && !stat.IsDir() {
			return cand, nil
		}
		candExe := cand + ".exe"
		if stat, err := os.Stat(candExe); err == nil && !stat.IsDir() {
			return candExe, nil
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("%s is not installed or not found in PATH", name)
}

// ParseGitRemote parses git remote URL formats (HTTPS, SSH, SCP-like) into
// structured metadata.
func ParseGitRemote(remoteName, remoteURL string) *GitRemoteSource {
	clean := strings.TrimSpace(remoteURL)
	clean = strings.TrimSuffix(clean, ".git")

	host := ""
	pathStr := ""

	if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") || strings.HasPrefix(clean, "ssh://") || strings.HasPrefix(clean, "git://") {
		u, err := url.Parse(clean)
		if err == nil {
			host = u.Hostname()
			pathStr = strings.TrimPrefix(u.Path, "/")
		} else {
			idx := strings.Index(clean, "://")
			rest := clean[idx+3:]
			if at := strings.Index(rest, "@"); at != -1 {
				rest = rest[at+1:]
			}
			if slash := strings.Index(rest, "/"); slash != -1 {
				host = strings.Split(rest[:slash], ":")[0]
				pathStr = rest[slash+1:]
			}
		}
	} else if strings.Contains(clean, ":") {
		// SCP-like format: git@host:owner/repo
		parts := strings.SplitN(clean, ":", 2)
		prefix := parts[0]
		if at := strings.Index(prefix, "@"); at != -1 {
			host = prefix[at+1:]
		} else {
			host = prefix
		}
		pathStr = parts[1]
	}

	pathStr = strings.Trim(pathStr, "/")
	pathParts := strings.Split(pathStr, "/")
	owner := ""
	repo := ""
	if len(pathParts) > 1 {
		owner = strings.Join(pathParts[:len(pathParts)-1], "/")
		repo = pathParts[len(pathParts)-1]
	} else if len(pathParts) == 1 {
		repo = pathParts[0]
	}

	lowerHost := strings.ToLower(host)
	isGitHub := strings.Contains(lowerHost, "github.com")
	isGitLab := strings.Contains(lowerHost, "gitlab")

	provider := "generic"
	if isGitHub {
		provider = "github"
	} else if isGitLab {
		provider = "gitlab"
	}

	webURL := ""
	pullsURL := ""
	if host != "" && pathStr != "" {
		webURL = fmt.Sprintf("https://%s/%s", host, pathStr)
		if isGitLab {
			pullsURL = fmt.Sprintf("https://%s/%s/-/merge_requests", host, pathStr)
		} else if isGitHub {
			pullsURL = fmt.Sprintf("https://%s/%s/pulls", host, pathStr)
		} else {
			pullsURL = webURL
		}
	}

	ghPath, _ := resolveGitCLI("gh")
	glabPath, _ := resolveGitCLI("glab")

	return &GitRemoteSource{
		Provider:   provider,
		RemoteName: remoteName,
		RemoteURL:  remoteURL,
		Host:       host,
		Owner:      owner,
		Repo:       repo,
		WebURL:     webURL,
		PullsURL:   pullsURL,
		IsGitHub:   isGitHub,
		IsGitLab:   isGitLab,
		GhPath:     ghPath,
		GlabPath:   glabPath,
	}
}

// GetRemoteSource detects the active git remote (origin, upstream, ...) and
// parses its host/provider.
func (e *Engine) GetRemoteSource(ctx context.Context, repoPath string) (*GitRemoteSource, error) {
	env := append(os.Environ(), "PATH="+EnrichedPath(), "GIT_TERMINAL_PROMPT=0")

	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	cmd.Env = env
	if out, err := cmd.Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return ParseGitRemote("origin", strings.TrimSpace(string(out))), nil
	}

	cmdAll := exec.CommandContext(ctx, "git", "remote", "-v")
	cmdAll.Dir = repoPath
	cmdAll.Env = env
	out, err := cmdAll.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return ParseGitRemote(fields[0], fields[1]), nil
			}
		}
	}

	ghPath, _ := resolveGitCLI("gh")
	glabPath, _ := resolveGitCLI("glab")
	return &GitRemoteSource{
		Provider: "generic",
		GhPath:   ghPath,
		GlabPath: glabPath,
	}, nil
}

func runGhCommand(ctx context.Context, repoPath string, args ...string) ([]byte, error) {
	ghExe, err := resolveGitCLI("gh")
	if err != nil {
		return nil, fmt.Errorf("GitHub CLI (gh) is not installed or not found in PATH. Install gh ('brew install gh') or authenticate with 'gh auth login' to manage pull requests")
	}

	cmd := exec.CommandContext(ctx, ghExe, args...)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "PATH="+EnrichedPath(), "GIT_TERMINAL_PROMPT=0", "GH_NO_UPDATE_NOTIFIER=1", "CLICOLOR=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		if errStr == "" {
			errStr = err.Error()
		}
		return nil, fmt.Errorf("%s", errStr)
	}
	return stdout.Bytes(), nil
}

func runGlabCommand(ctx context.Context, repoPath string, host string, args ...string) ([]byte, error) {
	glabExe, err := resolveGitCLI("glab")
	if err != nil {
		return nil, fmt.Errorf("GitLab CLI (glab) is not installed or not found in PATH. Install glab ('brew install glab') or authenticate with 'glab auth login' to manage Merge Requests")
	}

	cmd := exec.CommandContext(ctx, glabExe, args...)
	cmd.Dir = repoPath
	env := append(os.Environ(), "PATH="+EnrichedPath(), "GIT_TERMINAL_PROMPT=0", "NO_COLOR=1")
	if host != "" && !strings.Contains(host, "gitlab.com") {
		env = append(env, "GITLAB_HOST="+host)
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		if errStr == "" {
			errStr = err.Error()
		}
		return nil, fmt.Errorf("%s", errStr)
	}
	return stdout.Bytes(), nil
}

// GetPullRequests lists pull requests or GitLab merge requests for the repository.
func (e *Engine) GetPullRequests(ctx context.Context, repoPath string, state string) ([]PullRequestItem, error) {
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		state = "all"
	}

	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		glabState := state
		if glabState == "open" {
			glabState = "opened"
		}
		args := []string{"mr", "list", "--output", "json", "--per-page", "50"}
		if glabState != "all" {
			args = append(args, "--state", glabState)
		}
		out, err := runGlabCommand(ctx, repoPath, source.Host, args...)
		if err != nil {
			return nil, err
		}

		var rawGitlabItems []struct {
			IID          int    `json:"iid"`
			Title        string `json:"title"`
			State        string `json:"state"`
			Description  string `json:"description"`
			CreatedAt    string `json:"created_at"`
			UpdatedAt    string `json:"updated_at"`
			WebURL       string `json:"web_url"`
			SourceBranch string `json:"source_branch"`
			TargetBranch string `json:"target_branch"`
			Draft        bool   `json:"draft"`
			Author       struct {
				Name      string `json:"name"`
				Username  string `json:"username"`
				AvatarURL string `json:"avatar_url"`
			} `json:"author"`
		}
		if err := json.Unmarshal(out, &rawGitlabItems); err != nil {
			return nil, fmt.Errorf("failed to parse GitLab merge requests: %w", err)
		}

		items := make([]PullRequestItem, len(rawGitlabItems))
		for i, r := range rawGitlabItems {
			st := strings.ToUpper(r.State)
			if st == "OPENED" {
				st = "OPEN"
			}
			items[i] = PullRequestItem{
				Number:      r.IID,
				Title:       r.Title,
				State:       st,
				Author:      PullRequestAuthor{Login: r.Author.Username, Name: r.Author.Name, AvatarURL: r.Author.AvatarURL},
				HeadRefName: r.SourceBranch,
				BaseRefName: r.TargetBranch,
				CreatedAt:   r.CreatedAt,
				UpdatedAt:   r.UpdatedAt,
				IsDraft:     r.Draft,
				URL:         r.WebURL,
			}
		}
		return items, nil
	}

	fields := "number,title,state,author,headRefName,baseRefName,createdAt,updatedAt,isDraft,url,comments,reviews,additions,deletions,changedFiles"
	out, err := runGhCommand(ctx, repoPath, "pr", "list", "--state", state, "--limit", "50", "--json", fields)
	if err != nil {
		return nil, err
	}

	var rawItems []struct {
		Number       int                  `json:"number"`
		Title        string               `json:"title"`
		State        string               `json:"state"`
		Author       PullRequestAuthor    `json:"author"`
		HeadRefName  string               `json:"headRefName"`
		BaseRefName  string               `json:"baseRefName"`
		CreatedAt    string               `json:"createdAt"`
		UpdatedAt    string               `json:"updatedAt"`
		IsDraft      bool                 `json:"isDraft"`
		URL          string               `json:"url"`
		Additions    int                  `json:"additions"`
		Deletions    int                  `json:"deletions"`
		ChangedFiles int                  `json:"changedFiles"`
		Comments     []PullRequestComment `json:"comments"`
		Reviews      []PullRequestReview  `json:"reviews"`
	}
	if err := json.Unmarshal(out, &rawItems); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests: %w", err)
	}

	items := make([]PullRequestItem, len(rawItems))
	for i, r := range rawItems {
		items[i] = PullRequestItem{
			Number:        r.Number,
			Title:         r.Title,
			State:         r.State,
			Author:        r.Author,
			HeadRefName:   r.HeadRefName,
			BaseRefName:   r.BaseRefName,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
			IsDraft:       r.IsDraft,
			URL:           r.URL,
			Additions:     r.Additions,
			Deletions:     r.Deletions,
			ChangedFiles:  r.ChangedFiles,
			CommentsCount: len(r.Comments),
			ReviewsCount:  len(r.Reviews),
			LatestReviews: r.Reviews,
		}
	}
	return items, nil
}

// GetPullRequestDetail returns full metadata, body, comments, reviews, and
// changed files for a PR or MR.
func (e *Engine) GetPullRequestDetail(ctx context.Context, repoPath string, prNumber int) (*PullRequestDetail, error) {
	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		out, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "view", fmt.Sprintf("%d", prNumber), "--output", "json")
		if err == nil {
			var rawMr struct {
				IID          int    `json:"iid"`
				Title        string `json:"title"`
				Description  string `json:"description"`
				State        string `json:"state"`
				SourceBranch string `json:"source_branch"`
				TargetBranch string `json:"target_branch"`
				CreatedAt    string `json:"created_at"`
				UpdatedAt    string `json:"updated_at"`
				WebURL       string `json:"web_url"`
				Draft        bool   `json:"draft"`
				Author       struct {
					Name      string `json:"name"`
					Username  string `json:"username"`
					AvatarURL string `json:"avatar_url"`
				} `json:"author"`
			}
			if err := json.Unmarshal(out, &rawMr); err == nil {
				st := strings.ToUpper(rawMr.State)
				if st == "OPENED" {
					st = "OPEN"
				}
				return &PullRequestDetail{
					Number:      rawMr.IID,
					Title:       rawMr.Title,
					State:       st,
					Body:        rawMr.Description,
					HeadRefName: rawMr.SourceBranch,
					BaseRefName: rawMr.TargetBranch,
					CreatedAt:   rawMr.CreatedAt,
					UpdatedAt:   rawMr.UpdatedAt,
					URL:         rawMr.WebURL,
					IsDraft:     rawMr.Draft,
					Author:      PullRequestAuthor{Login: rawMr.Author.Username, Name: rawMr.Author.Name, AvatarURL: rawMr.Author.AvatarURL},
					Files:       []PullRequestFile{},
					Comments:    []PullRequestComment{},
					Reviews:     []PullRequestReview{},
				}, nil
			}
		}
		// Fallback without json
		outText, errText := runGlabCommand(ctx, repoPath, source.Host, "mr", "view", fmt.Sprintf("%d", prNumber))
		if errText != nil {
			return nil, errText
		}
		return &PullRequestDetail{
			Number: prNumber,
			Title:  fmt.Sprintf("Merge Request !%d", prNumber),
			State:  "OPEN",
			Body:   string(outText),
			Files:  []PullRequestFile{},
		}, nil
	}

	fields := "number,title,state,body,author,headRefName,baseRefName,createdAt,updatedAt,isDraft,url,comments,reviews,files,additions,deletions,changedFiles,mergeable"
	out, err := runGhCommand(ctx, repoPath, "pr", "view", fmt.Sprintf("%d", prNumber), "--json", fields)
	if err != nil {
		return nil, err
	}

	var detail PullRequestDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, fmt.Errorf("failed to parse PR detail: %w", err)
	}
	return &detail, nil
}

// GetPullRequestDiff returns the raw unified diff for a pull request or merge request.
func (e *Engine) GetPullRequestDiff(ctx context.Context, repoPath string, prNumber int) (string, error) {
	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		out, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "diff", fmt.Sprintf("%d", prNumber))
		if err != nil {
			return "", err
		}
		return string(out), nil
	}

	out, err := runGhCommand(ctx, repoPath, "pr", "diff", fmt.Sprintf("%d", prNumber))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CreatePullRequestComment posts a general comment or note on a pull request
// or merge request.
func (e *Engine) CreatePullRequestComment(ctx context.Context, repoPath string, prNumber int, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("comment body cannot be empty")
	}

	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		_, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "note", fmt.Sprintf("%d", prNumber), "-m", body)
		return err
	}

	_, err := runGhCommand(ctx, repoPath, "pr", "comment", fmt.Sprintf("%d", prNumber), "--body", body)
	return err
}

// ReviewPullRequest submits an approval, request changes, or review comment
// for a pull request.
func (e *Engine) ReviewPullRequest(ctx context.Context, repoPath string, prNumber int, action string, body string) error {
	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		switch strings.ToLower(strings.TrimSpace(action)) {
		case "approve":
			_, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "approve", fmt.Sprintf("%d", prNumber))
			return err
		default:
			if strings.TrimSpace(body) != "" {
				_, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "note", fmt.Sprintf("%d", prNumber), "-m", body)
				return err
			}
			return nil
		}
	}

	args := []string{"pr", "review", fmt.Sprintf("%d", prNumber)}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "approve":
		args = append(args, "--approve")
		if strings.TrimSpace(body) != "" {
			args = append(args, "-b", body)
		}
	case "request-changes", "request_changes":
		args = append(args, "--request-changes")
		if strings.TrimSpace(body) != "" {
			args = append(args, "-b", body)
		} else {
			args = append(args, "-b", "Changes requested.")
		}
	default:
		args = append(args, "--comment")
		if strings.TrimSpace(body) != "" {
			args = append(args, "-b", body)
		} else {
			args = append(args, "-b", "Review comment.")
		}
	}
	_, err := runGhCommand(ctx, repoPath, args...)
	return err
}

// MergePullRequest merges the pull request with the chosen strategy
// ("merge", "squash", "rebase").
func (e *Engine) MergePullRequest(ctx context.Context, repoPath string, prNumber int, method string) (string, error) {
	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		args := []string{"mr", "merge", fmt.Sprintf("%d", prNumber), "--yes"}
		if strings.ToLower(strings.TrimSpace(method)) == "squash" {
			args = append(args, "--squash")
		} else if strings.ToLower(strings.TrimSpace(method)) == "rebase" {
			args = append(args, "--rebase")
		}
		out, err := runGlabCommand(ctx, repoPath, source.Host, args...)
		if err == nil {
			e.Invalidate(repoPath)
		}
		return string(out), err
	}

	args := []string{"pr", "merge", fmt.Sprintf("%d", prNumber)}
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--merge")
	}
	out, err := runGhCommand(ctx, repoPath, args...)
	if err == nil {
		e.Invalidate(repoPath)
	}
	return string(out), err
}

// ClosePullRequest closes an open pull request or merge request.
func (e *Engine) ClosePullRequest(ctx context.Context, repoPath string, prNumber int) error {
	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		_, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "close", fmt.Sprintf("%d", prNumber))
		return err
	}
	_, err := runGhCommand(ctx, repoPath, "pr", "close", fmt.Sprintf("%d", prNumber))
	return err
}

// ReopenPullRequest reopens a closed pull request or merge request.
func (e *Engine) ReopenPullRequest(ctx context.Context, repoPath string, prNumber int) error {
	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		_, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "reopen", fmt.Sprintf("%d", prNumber))
		return err
	}
	_, err := runGhCommand(ctx, repoPath, "pr", "reopen", fmt.Sprintf("%d", prNumber))
	return err
}

// CheckoutPullRequest checks out the pull request or merge request branch
// locally.
func (e *Engine) CheckoutPullRequest(ctx context.Context, repoPath string, prNumber int) (string, error) {
	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		out, err := runGlabCommand(ctx, repoPath, source.Host, "mr", "checkout", fmt.Sprintf("%d", prNumber))
		if err == nil {
			e.Invalidate(repoPath)
		}
		return string(out), err
	}

	out, err := runGhCommand(ctx, repoPath, "pr", "checkout", fmt.Sprintf("%d", prNumber))
	if err == nil {
		e.Invalidate(repoPath)
	}
	return string(out), err
}

// CreatePullRequest opens a new pull request or merge request on the
// repository remote.
func (e *Engine) CreatePullRequest(ctx context.Context, repoPath string, title string, body string, base string, head string, draft bool) (*PullRequestItem, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("pull request title is required")
	}

	source, _ := e.GetRemoteSource(ctx, repoPath)
	if source != nil && source.IsGitLab {
		args := []string{"mr", "create", "--title", title, "--description", body, "--yes"}
		if strings.TrimSpace(base) != "" {
			args = append(args, "--target-branch", strings.TrimSpace(base))
		}
		if strings.TrimSpace(head) != "" {
			args = append(args, "--source-branch", strings.TrimSpace(head))
		}
		if draft {
			args = append(args, "--draft")
		}
		out, err := runGlabCommand(ctx, repoPath, source.Host, args...)
		if err != nil {
			return nil, err
		}
		return &PullRequestItem{
			Title: title,
			URL:   strings.TrimSpace(string(out)),
			State: "OPEN",
		}, nil
	}

	args := []string{"pr", "create", "--title", title, "--body", body}
	if strings.TrimSpace(base) != "" {
		args = append(args, "--base", strings.TrimSpace(base))
	}
	if strings.TrimSpace(head) != "" {
		args = append(args, "--head", strings.TrimSpace(head))
	}
	if draft {
		args = append(args, "--draft")
	}

	out, err := runGhCommand(ctx, repoPath, args...)
	if err != nil {
		return nil, err
	}
	return &PullRequestItem{
		Title: title,
		URL:   strings.TrimSpace(string(out)),
		State: "OPEN",
	}, nil
}
