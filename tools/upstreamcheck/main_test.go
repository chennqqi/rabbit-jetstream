package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLock(t *testing.T) {
	valid := lockFile{
		Repository:    officialRemote,
		Tag:           "v2.14.1",
		Commit:        "cb557cd5d3ee8b4a253071d9923730d7d5588f99",
		Tree:          "e86e9e29fba760657c1a71bd5507bb05b3f7e104",
		SubtreeCommit: "a51b751575024febc15b89aa7116208b025cc654",
	}
	if err := validateLock(valid); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*lockFile)
	}{
		{"unofficial repository", func(lock *lockFile) { lock.Repository = "https://example.invalid/nats-server.git" }},
		{"prerelease tag", func(lock *lockFile) { lock.Tag = "v2.14.1-RC.1" }},
		{"abbreviated commit", func(lock *lockFile) { lock.Commit = lock.Commit[:8] }},
		{"invalid tree", func(lock *lockFile) { lock.Tree = "not-a-tree" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := validateLock(candidate); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestVerifyLocalRejectsModifiedSubtree(t *testing.T) {
	repository := t.TempDir()
	mustGit := func(arguments ...string) string {
		t.Helper()
		value, err := git(repository, arguments...)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	mustGit("init", "--quiet")
	mustGit("config", "user.name", "upstreamcheck test")
	mustGit("config", "user.email", "upstreamcheck@example.invalid")
	content := []byte("official source\n")
	if err := os.WriteFile(filepath.Join(repository, "source.txt"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "source.txt")
	mustGit("commit", "--quiet", "-m", "Squashed content from commit aaaaaaaa")
	squashCommit := mustGit("rev-parse", "HEAD")
	tree := mustGit("rev-parse", "HEAD^{tree}")

	if err := os.Remove(filepath.Join(repository, "source.txt")); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(repository, filepath.FromSlash(subtreePath))
	if err := os.MkdirAll(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(destination, "source.txt")
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "--all")
	mustGit("commit", "--quiet", "-m", "Import locked subtree")
	lock := lockFile{Commit: strings.Repeat("a", 40), Tree: tree, SubtreeCommit: squashCommit}
	if err := verifyLocal(repository, lock); err != nil {
		t.Fatalf("clean subtree rejected: %v", err)
	}
	if err := os.WriteFile(file, []byte("locally patched\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyLocal(repository, lock); err == nil {
		t.Fatal("modified subtree was accepted")
	}
	mustGit("add", filepath.ToSlash(filepath.Join(subtreePath, "source.txt")))
	if err := verifyLocal(repository, lock); err == nil {
		t.Fatal("staged subtree modification was accepted")
	}
}
