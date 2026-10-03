package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	lockPath       = "upstream/nats-server.lock.json"
	subtreePath    = "upstream/nats-server"
	officialRemote = "https://github.com/nats-io/nats-server.git"
)

type lockFile struct {
	Repository    string `json:"repository"`
	Tag           string `json:"tag"`
	Commit        string `json:"commit"`
	Tree          string `json:"tree"`
	SubtreeCommit string `json:"subtree_commit"`
}

var (
	objectID  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	stableTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
)

func main() {
	online := flag.Bool("online", false, "fetch the locked tag from the official repository and verify its commit and tree")
	flag.Parse()
	if err := run(*online); err != nil {
		fmt.Fprintln(os.Stderr, "upstream verification failed:", err)
		os.Exit(1)
	}
}

func run(online bool) error {
	root, err := git("", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lockPath)))
	if err != nil {
		return err
	}
	var lock lockFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		return fmt.Errorf("decode %s: %w", lockPath, err)
	}
	if err := validateLock(lock); err != nil {
		return err
	}
	if err := verifyLocal(root, lock); err != nil {
		return err
	}
	if online {
		if err := verifyOfficial(lock); err != nil {
			return err
		}
	}
	fmt.Printf("verified NATS Server %s commit=%s tree=%s online=%t\n", lock.Tag, lock.Commit, lock.Tree, online)
	return nil
}

func validateLock(lock lockFile) error {
	if lock.Repository != officialRemote {
		return fmt.Errorf("repository must be %s", officialRemote)
	}
	if !stableTag.MatchString(lock.Tag) {
		return fmt.Errorf("tag %q is not a stable release tag", lock.Tag)
	}
	for name, value := range map[string]string{"commit": lock.Commit, "tree": lock.Tree, "subtree_commit": lock.SubtreeCommit} {
		if !objectID.MatchString(value) {
			return fmt.Errorf("%s must be a full lowercase Git object ID", name)
		}
	}
	return nil
}

func verifyLocal(root string, lock lockFile) error {
	actualTree, err := git(root, "rev-parse", "HEAD:"+subtreePath)
	if err != nil {
		return err
	}
	if actualTree != lock.Tree {
		return fmt.Errorf("checked-in subtree tree is %s, want %s", actualTree, lock.Tree)
	}
	squashTree, err := git(root, "rev-parse", lock.SubtreeCommit+"^{tree}")
	if err != nil {
		return fmt.Errorf("resolve subtree commit: %w", err)
	}
	if squashTree != lock.Tree {
		return fmt.Errorf("subtree commit tree is %s, want %s", squashTree, lock.Tree)
	}
	message, err := git(root, "show", "-s", "--format=%B", lock.SubtreeCommit)
	if err != nil {
		return err
	}
	if !strings.Contains(message, lock.Commit[:8]) {
		return errors.New("subtree commit message does not identify the locked official commit")
	}
	if _, err := git(root, "diff", "--quiet", "HEAD", "--", subtreePath); err != nil {
		return errors.New("working tree contains modifications under upstream/nats-server")
	}
	untracked, err := git(root, "ls-files", "--others", "--exclude-standard", "--", subtreePath)
	if err != nil {
		return err
	}
	if untracked != "" {
		return fmt.Errorf("working tree contains untracked upstream files: %s", untracked)
	}
	return nil
}

func verifyOfficial(lock lockFile) error {
	temporary, err := os.MkdirTemp("", "rabbit-jetstream-upstream-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if _, err := git(temporary, "init", "--bare"); err != nil {
		return err
	}
	if _, err := git(temporary, "fetch", "--quiet", "--depth=1", lock.Repository, "refs/tags/"+lock.Tag); err != nil {
		return fmt.Errorf("fetch official tag: %w", err)
	}
	commit, err := git(temporary, "rev-parse", "FETCH_HEAD^{commit}")
	if err != nil {
		return err
	}
	tree, err := git(temporary, "rev-parse", "FETCH_HEAD^{tree}")
	if err != nil {
		return err
	}
	if commit != lock.Commit || tree != lock.Tree {
		return fmt.Errorf("official tag resolves to commit=%s tree=%s, lock has commit=%s tree=%s", commit, tree, lock.Commit, lock.Tree)
	}
	return nil
}

func git(directory string, arguments ...string) (string, error) {
	command := exec.Command("git", arguments...)
	if directory != "" {
		command.Dir = directory
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		diagnostic := strings.TrimSpace(strings.Join([]string{string(output), stderr.String()}, "\n"))
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, diagnostic)
	}
	return strings.TrimSpace(string(output)), nil
}
