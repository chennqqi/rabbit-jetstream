package contract_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryRepositoryNATSConnectionUsesSharedSecurityPolicy(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && (relative == ".git" || relative == "upstream" || strings.HasPrefix(relative, "upstream"+string(filepath.Separator))) {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(source, []byte("nats.Connect(")) && !bytes.Contains(source, []byte("natsclient.Options(")) {
			t.Errorf("%s connects to NATS without the shared credentials/TLS policy", filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
