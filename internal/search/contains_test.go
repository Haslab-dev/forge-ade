package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilenameContainsCaseInsensitive(t *testing.T) {
	sm := NewSearchManager()
	tmpDir := t.TempDir()

	// Files matching "survey" as a substring, in any case/position.
	names := []string{
		"SurveyQuiz.tsx",
		"survey.txt",
		"surveyquitattachment.go",
		"CustomerSurvey.js",
		"MY_SURVEY_MODULE.rb",
		"unrelated.py",
		"notes.md",
	}
	for _, n := range names {
		_ = os.WriteFile(filepath.Join(tmpDir, n), []byte("package x\n"), 0644)
	}
	sm.SetDirectories([]string{tmpDir})

	// Default options: no MatchCase, no regex, no whole word.
	got := sm.SearchFilenameWithOptions(SearchOptions{Query: "Survey", Limit: 50})
	var gotNames []string
	for _, r := range got {
		gotNames = append(gotNames, r.Filename)
	}
	t.Logf("SearchFilenameWithOptions('Survey') => %v", gotNames)

	expected := []string{
		"SurveyQuiz.tsx",
		"survey.txt",
		"surveyquitattachment.go",
		"CustomerSurvey.js",
		"MY_SURVEY_MODULE.rb",
	}
	for _, e := range expected {
		found := false
		for _, g := range gotNames {
			if g == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected '%s' in results, got %v", e, gotNames)
		}
	}
	for _, g := range gotNames {
		if g == "unrelated.py" || g == "notes.md" {
			t.Errorf("unexpected result '%s'", g)
		}
	}

	// Legacy wrapper should behave the same.
	got2 := sm.SearchFilename("SURVEY", 50)
	for _, r := range got2 {
		if r.Filename == "unrelated.py" {
			t.Errorf("legacy search returned unexpected '%s'", r.Filename)
		}
	}
	if len(got2) != 5 {
		t.Errorf("legacy search returned %d results, want 5", len(got2))
	}

	// Substring in the middle of a long name, no case match on the letters.
	mid := sm.SearchFilenameWithOptions(SearchOptions{Query: "itattach", Limit: 50})
	if len(mid) != 1 || !strings.HasSuffix(mid[0].Filename, "surveyquitattachment.go") {
		t.Errorf("expected only surveyquitattachment.go for 'itattach', got %+v", mid)
	}

	// Strict contains: non-contiguous letter matches must NOT appear.
	scattered := sm.SearchFilenameWithOptions(SearchOptions{Query: "rvqu", Limit: 50})
	if len(scattered) != 0 {
		t.Errorf("expected no results for non-contiguous 'rvqu', got %+v", scattered)
	}
}
