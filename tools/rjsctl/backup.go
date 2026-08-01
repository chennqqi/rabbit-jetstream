package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupSchema = "rabbit-jetstream.io/backup/v1alpha1"

type commandRunner interface {
	Run(name string, args []string, stdout, stderr io.Writer) error
}

type execRunner struct{}

func (execRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	command := exec.Command(name, args...)
	command.Stdout, command.Stderr = stdout, stderr
	command.Env = natsCLIEnvironment(os.Environ(), os.Getenv)
	return command.Run()
}

func natsCLIEnvironment(base []string, getenv func(string) string) []string {
	result := append([]string(nil), base...)
	existing := make(map[string]struct{}, len(base))
	for _, entry := range base {
		if key, _, ok := strings.Cut(entry, "="); ok {
			existing[key] = struct{}{}
		}
	}
	appendMapping := func(source, destination string) {
		if _, set := existing[destination]; set {
			return
		}
		if value := getenv(source); value != "" {
			result = append(result, destination+"="+value)
		}
	}
	appendMapping("RJS_NATS_URL", "NATS_URL")
	_, standardUser := existing["NATS_USER"]
	_, standardPassword := existing["NATS_PASSWORD"]
	_, standardCreds := existing["NATS_CREDS"]
	if !standardUser && !standardPassword && !standardCreds {
		if getenv("RJS_NATS_CREDS") != "" {
			appendMapping("RJS_NATS_CREDS", "NATS_CREDS")
		} else {
			appendMapping("RJS_NATS_USER", "NATS_USER")
			appendMapping("RJS_NATS_PASSWORD", "NATS_PASSWORD")
		}
	}
	for _, mapping := range [][2]string{{"RJS_NATS_TLS_CA", "NATS_CA"}, {"RJS_NATS_TLS_CERT", "NATS_CERT"}, {"RJS_NATS_TLS_KEY", "NATS_KEY"}} {
		appendMapping(mapping[0], mapping[1])
	}
	return result
}

type backupManifest struct {
	Schema      string         `json:"schema"`
	CreatedAt   time.Time      `json:"created_at"`
	Server      string         `json:"server,omitempty"`
	NATSCLI     string         `json:"nats_cli"`
	ToolVersion string         `json:"tool_version"`
	Streams     []backupStream `json:"streams"`
	FileCount   int            `json:"file_count"`
	TotalBytes  int64          `json:"total_bytes"`
}

type backupStream struct {
	Name  string       `json:"name"`
	Path  string       `json:"path"`
	Files []backupFile `json:"files"`
}

type backupFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func createBackup(runner commandRunner, natsBin, server, output string, now time.Time, stdout, stderr io.Writer) error {
	if err := validateBackupServer(server); err != nil {
		return err
	}
	if output == "" {
		return errors.New("backup output directory is required")
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("refusing to overwrite existing path %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect backup output: %w", err)
	}
	parent := filepath.Dir(output)
	temporary, err := os.MkdirTemp(parent, ".rjs-backup-*")
	if err != nil {
		return fmt.Errorf("create backup staging directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(temporary)
		}
	}()

	var cliVersion bytesBuffer
	if err := runner.Run(natsBin, []string{"--version"}, &cliVersion, stderr); err != nil {
		return fmt.Errorf("read nats CLI version: %w", err)
	}
	var list bytesBuffer
	if err := runner.Run(natsBin, natsArgs(server, "stream", "ls", "--all", "--json"), &list, stderr); err != nil {
		return fmt.Errorf("list JetStream streams: %w", err)
	}
	var names []string
	if err := json.Unmarshal(list.Bytes(), &names); err != nil {
		return fmt.Errorf("decode JetStream stream list: %w", err)
	}
	sort.Strings(names)
	manifest := backupManifest{Schema: backupSchema, CreatedAt: now.UTC(), Server: publicURL(server), NATSCLI: strings.TrimSpace(string(cliVersion.Bytes())), ToolVersion: version, Streams: make([]backupStream, 0, len(names))}
	for _, name := range names {
		if !safeStreamName(name) {
			return fmt.Errorf("unsafe Stream name returned by server: %q", name)
		}
		relative := filepath.ToSlash(filepath.Join("streams", name))
		target := filepath.Join(temporary, filepath.FromSlash(relative))
		if err := os.MkdirAll(target, 0o700); err != nil {
			return fmt.Errorf("create Stream backup directory %s: %w", name, err)
		}
		if err := runner.Run(natsBin, natsArgs(server, "stream", "backup", "--all", "--check", "--consumers", "--no-progress", name, target), stdout, stderr); err != nil {
			return fmt.Errorf("backup Stream %s: %w", name, err)
		}
		files, err := checksumTree(temporary, target)
		if err != nil {
			return fmt.Errorf("checksum Stream %s backup: %w", name, err)
		}
		if len(files) == 0 {
			return fmt.Errorf("backup Stream %s produced no files", name)
		}
		manifest.Streams = append(manifest.Streams, backupStream{Name: name, Path: relative, Files: files})
		for _, file := range files {
			manifest.FileCount++
			manifest.TotalBytes += file.Size
		}
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode backup manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(temporary, "manifest.json"), append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write backup manifest: %w", err)
	}
	if err := os.Rename(temporary, output); err != nil {
		return fmt.Errorf("publish backup: %w", err)
	}
	committed = true
	return nil
}

func restoreBackup(runner commandRunner, natsBin, server, input, confirmation string, replicas int, stdout, stderr io.Writer) error {
	if err := validateBackupServer(server); err != nil {
		return err
	}
	if confirmation != "RESTORE" {
		return errors.New("backup restore requires --confirm RESTORE")
	}
	manifest, err := verifyBackup(input)
	if err != nil {
		return err
	}
	if replicas != 0 && replicas != 1 && replicas != 3 && replicas != 5 {
		return errors.New("replicas must be 1, 3, or 5")
	}

	var list bytesBuffer
	if err := runner.Run(natsBin, natsArgs(server, "stream", "ls", "--all", "--json"), &list, stderr); err != nil {
		return fmt.Errorf("list destination JetStream streams: %w", err)
	}
	var existing []string
	if err := json.Unmarshal(list.Bytes(), &existing); err != nil {
		return fmt.Errorf("decode destination Stream list: %w", err)
	}
	present := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		present[name] = struct{}{}
	}
	for _, stream := range manifest.Streams {
		if _, ok := present[stream.Name]; ok {
			return fmt.Errorf("refusing restore: Stream %s already exists", stream.Name)
		}
	}
	for index, stream := range manifest.Streams {
		args := natsArgs(server, "stream", "restore", "--no-progress")
		if replicas != 0 {
			args = append(args, "--replicas", fmt.Sprint(replicas))
		}
		args = append(args, filepath.Join(input, filepath.FromSlash(stream.Path)))
		if err := runner.Run(natsBin, args, stdout, stderr); err != nil {
			return fmt.Errorf("restore Stream %s (%d of %d); previous Streams may already be restored: %w", stream.Name, index+1, len(manifest.Streams), err)
		}
	}
	return nil
}

func verifyBackup(input string) (*backupManifest, error) {
	encoded, err := os.ReadFile(filepath.Join(input, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read backup manifest: %w", err)
	}
	var manifest backupManifest
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode backup manifest: %w", err)
	}
	if manifest.Schema != backupSchema {
		return nil, fmt.Errorf("unsupported backup schema %q", manifest.Schema)
	}
	seenNames, seenFiles := map[string]struct{}{}, map[string]struct{}{}
	var fileCount int
	var totalBytes int64
	for _, stream := range manifest.Streams {
		if !safeStreamName(stream.Name) || stream.Path != filepath.ToSlash(filepath.Join("streams", stream.Name)) {
			return nil, fmt.Errorf("invalid backup Stream entry %q", stream.Name)
		}
		if _, exists := seenNames[stream.Name]; exists {
			return nil, fmt.Errorf("duplicate backup Stream %s", stream.Name)
		}
		seenNames[stream.Name] = struct{}{}
		if len(stream.Files) == 0 {
			return nil, fmt.Errorf("Stream %s has no backup files", stream.Name)
		}
		for _, file := range stream.Files {
			clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(file.Path)))
			prefix := stream.Path + "/"
			if !strings.HasPrefix(clean, prefix) || clean == stream.Path {
				return nil, fmt.Errorf("invalid backup file path %q", file.Path)
			}
			if _, exists := seenFiles[clean]; exists {
				return nil, fmt.Errorf("duplicate backup file %s", clean)
			}
			seenFiles[clean] = struct{}{}
			actual, err := checksumFile(filepath.Join(input, filepath.FromSlash(clean)))
			if err != nil {
				return nil, fmt.Errorf("verify backup file %s: %w", clean, err)
			}
			if actual.Size != file.Size || actual.SHA256 != file.SHA256 {
				return nil, fmt.Errorf("backup file %s failed integrity verification", clean)
			}
			fileCount++
			totalBytes += actual.Size
		}
	}
	if fileCount != manifest.FileCount || totalBytes != manifest.TotalBytes {
		return nil, errors.New("backup manifest totals do not match file entries")
	}
	if len(manifest.Streams) > 0 {
		actualFiles, err := checksumTree(input, filepath.Join(input, "streams"))
		if err != nil {
			return nil, fmt.Errorf("inspect backup tree: %w", err)
		}
		if len(actualFiles) != len(seenFiles) {
			return nil, errors.New("backup contains files not declared by the manifest")
		}
		for _, file := range actualFiles {
			if _, exists := seenFiles[file.Path]; !exists {
				return nil, fmt.Errorf("backup file %s is not declared by the manifest", file.Path)
			}
		}
	}
	return &manifest, nil
}

func checksumTree(root, directory string) ([]backupFile, error) {
	var files []backupFile
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link is not allowed: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := checksumFile(path)
		if err != nil {
			return err
		}
		file.Path = filepath.ToSlash(relative)
		files = append(files, file)
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, err
}

func checksumFile(path string) (backupFile, error) {
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return backupFile{}, err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return backupFile{}, errors.New("symbolic link is not allowed")
	}
	file, err := os.Open(path)
	if err != nil {
		return backupFile{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return backupFile{}, err
	}
	if !info.Mode().IsRegular() {
		return backupFile{}, errors.New("not a regular file")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return backupFile{}, err
	}
	return backupFile{Size: info.Size(), SHA256: hex.EncodeToString(digest.Sum(nil))}, nil
}

func natsArgs(server string, args ...string) []string {
	if server == "" {
		return args
	}
	return append([]string{"--server", server}, args...)
}

func safeStreamName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, " .\\/>*\t\r\n")
}

func validateBackupServer(server string) error {
	if server == "" {
		return nil
	}
	parsed, err := url.Parse(server)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("server must be an absolute NATS URL")
	}
	if parsed.User != nil {
		return errors.New("server URL must not contain credentials; use NATS credentials environment or files")
	}
	return nil
}

type bytesBuffer struct{ value []byte }

func (b *bytesBuffer) Write(value []byte) (int, error) {
	b.value = append(b.value, value...)
	return len(value), nil
}
func (b *bytesBuffer) Bytes() []byte { return b.value }
