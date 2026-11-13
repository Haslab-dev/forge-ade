package git

import (
	"strings"
	"testing"
)

func TestParseGitRemoteGitHubHTTPS(t *testing.T) {
	src := ParseGitRemote("origin", "https://github.com/Haslab-dev/forge-ade.git")
	if src.Provider != "github" || !src.IsGitHub || src.IsGitLab {
		t.Fatalf("provider = %q", src.Provider)
	}
	if src.Host != "github.com" || src.Owner != "Haslab-dev" || src.Repo != "forge-ade" {
		t.Fatalf("host=%q owner=%q repo=%q", src.Host, src.Owner, src.Repo)
	}
	if src.WebURL != "https://github.com/Haslab-dev/forge-ade" {
		t.Fatalf("webURL = %q", src.WebURL)
	}
	if !strings.HasSuffix(src.PullsURL, "/pulls") {
		t.Fatalf("pullsURL = %q", src.PullsURL)
	}
}

func TestParseGitRemoteGitLabSSH(t *testing.T) {
	src := ParseGitRemote("origin", "git@gitlab.com:mygroup/myproject.git")
	if src.Provider != "gitlab" || !src.IsGitLab {
		t.Fatalf("provider = %q", src.Provider)
	}
	if src.Owner != "mygroup" || src.Repo != "myproject" {
		t.Fatalf("owner=%q repo=%q", src.Owner, src.Repo)
	}
	if !strings.HasSuffix(src.PullsURL, "/-/merge_requests") {
		t.Fatalf("pullsURL = %q", src.PullsURL)
	}
}

func TestParseGitRemoteSelfHostedGitLab(t *testing.T) {
	src := ParseGitRemote("origin", "ssh://git@gitlab.mycompany.com:2222/team/sub/repo.git")
	if !src.IsGitLab || src.Host != "gitlab.mycompany.com" {
		t.Fatalf("host=%q provider=%q", src.Host, src.Provider)
	}
	if src.Owner != "team/sub" || src.Repo != "repo" {
		t.Fatalf("owner=%q repo=%q", src.Owner, src.Repo)
	}
}

func TestParseGitRemoteGeneric(t *testing.T) {
	src := ParseGitRemote("upstream", "https://git.example.com/org/repo")
	if src.Provider != "generic" || src.IsGitHub || src.IsGitLab {
		t.Fatalf("provider = %q", src.Provider)
	}
	if src.Repo != "repo" || src.Owner != "org" {
		t.Fatalf("owner=%q repo=%q", src.Owner, src.Repo)
	}
}
