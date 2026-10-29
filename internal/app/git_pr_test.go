package app

import (
	"testing"
)

func TestPullRequests_LiveGh(t *testing.T) {
	app := &App{}
	repoPath := "/data/workspace/forge-ade"

	prs, err := app.GetPullRequests(repoPath, "all")
	if err != nil {
		t.Fatalf("GetPullRequests failed: %v", err)
	}

	if len(prs) == 0 {
		t.Fatalf("expected at least 1 PR, got 0")
	}

	t.Logf("Fetched %d pull requests", len(prs))
	first := prs[0]
	t.Logf("PR #%d: %s (%s) by %s", first.Number, first.Title, first.State, first.Author.Login)

	detail, err := app.GetPullRequestDetail(repoPath, first.Number)
	if err != nil {
		t.Fatalf("GetPullRequestDetail failed: %v", err)
	}

	if detail.Number != first.Number {
		t.Fatalf("expected detail.Number == %d, got %d", first.Number, detail.Number)
	}
	t.Logf("PR detail: body len %d, files %d, comments %d, reviews %d", len(detail.Body), len(detail.Files), len(detail.Comments), len(detail.Reviews))

	diff, err := app.GetPullRequestDiff(repoPath, first.Number)
	if err != nil {
		t.Fatalf("GetPullRequestDiff failed: %v", err)
	}
	if len(diff) == 0 {
		t.Fatalf("expected non-empty diff for PR #%d", first.Number)
	}
	t.Logf("PR diff len: %d bytes", len(diff))
}
