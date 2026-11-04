package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitRemote(t *testing.T) {
	tests := []struct {
		name         string
		remoteName   string
		remoteURL    string
		wantProvider string
		wantHost     string
		wantOwner    string
		wantRepo     string
		wantIsGitHub bool
		wantIsGitLab bool
		wantWebURL   string
		wantPullsURL string
	}{
		{
			name:         "GitHub HTTPS",
			remoteName:   "origin",
			remoteURL:    "https://github.com/Haslab-dev/forge-ade.git",
			wantProvider: "github",
			wantHost:     "github.com",
			wantOwner:    "Haslab-dev",
			wantRepo:     "forge-ade",
			wantIsGitHub: true,
			wantIsGitLab: false,
			wantWebURL:   "https://github.com/Haslab-dev/forge-ade",
			wantPullsURL: "https://github.com/Haslab-dev/forge-ade/pulls",
		},
		{
			name:         "GitHub SSH SCP format",
			remoteName:   "origin",
			remoteURL:    "git@github.com:facebook/react.git",
			wantProvider: "github",
			wantHost:     "github.com",
			wantOwner:    "facebook",
			wantRepo:     "react",
			wantIsGitHub: true,
			wantIsGitLab: false,
			wantWebURL:   "https://github.com/facebook/react",
			wantPullsURL: "https://github.com/facebook/react/pulls",
		},
		{
			name:         "GitLab SaaS HTTPS",
			remoteName:   "origin",
			remoteURL:    "https://gitlab.com/gitlab-org/gitlab-runner.git",
			wantProvider: "gitlab",
			wantHost:     "gitlab.com",
			wantOwner:    "gitlab-org",
			wantRepo:     "gitlab-runner",
			wantIsGitHub: false,
			wantIsGitLab: true,
			wantWebURL:   "https://gitlab.com/gitlab-org/gitlab-runner",
			wantPullsURL: "https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests",
		},
		{
			name:         "Private GitLab SSH",
			remoteName:   "upstream",
			remoteURL:    "git@gitlab.mycompany.internal:core/services/billing-service.git",
			wantProvider: "gitlab",
			wantHost:     "gitlab.mycompany.internal",
			wantOwner:    "core/services",
			wantRepo:     "billing-service",
			wantIsGitHub: false,
			wantIsGitLab: true,
			wantWebURL:   "https://gitlab.mycompany.internal/core/services/billing-service",
			wantPullsURL: "https://gitlab.mycompany.internal/core/services/billing-service/-/merge_requests",
		},
		{
			name:         "Private GitLab HTTPS with Port",
			remoteName:   "origin",
			remoteURL:    "https://gitlab.corp.net:8443/engineering/agent.git",
			wantProvider: "gitlab",
			wantHost:     "gitlab.corp.net",
			wantOwner:    "engineering",
			wantRepo:     "agent",
			wantIsGitHub: false,
			wantIsGitLab: true,
			wantWebURL:   "https://gitlab.corp.net/engineering/agent",
			wantPullsURL: "https://gitlab.corp.net/engineering/agent/-/merge_requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGitRemote(tt.remoteName, tt.remoteURL)
			if got.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", got.Provider, tt.wantProvider)
			}
			if got.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", got.Host, tt.wantHost)
			}
			if got.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.wantOwner)
			}
			if got.Repo != tt.wantRepo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.wantRepo)
			}
			if got.IsGitHub != tt.wantIsGitHub {
				t.Errorf("IsGitHub = %v, want %v", got.IsGitHub, tt.wantIsGitHub)
			}
			if got.IsGitLab != tt.wantIsGitLab {
				t.Errorf("IsGitLab = %v, want %v", got.IsGitLab, tt.wantIsGitLab)
			}
			if got.WebURL != tt.wantWebURL {
				t.Errorf("WebURL = %q, want %q", got.WebURL, tt.wantWebURL)
			}
			if got.PullsURL != tt.wantPullsURL {
				t.Errorf("PullsURL = %q, want %q", got.PullsURL, tt.wantPullsURL)
			}
		})
	}
}

func TestResolveGitCLIOverride(t *testing.T) {
	tmpDir := t.TempDir()
	fakeGh := filepath.Join(tmpDir, "gh")
	if err := os.WriteFile(fakeGh, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FORGE_GH_PATH", fakeGh)
	resolved, err := resolveGitCLI("gh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != fakeGh {
		t.Errorf("got %q, want %q", resolved, fakeGh)
	}
}
