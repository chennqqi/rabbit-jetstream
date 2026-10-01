package qualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	adminui "github.com/chennqqi/rabbit-jetstream/admin-ui"
)

const (
	manifestSchema  = "rabbit-jetstream.io/release-bundle/v1alpha1"
	maxManifestSize = 1 << 20
)

var revisionPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

// RuntimeIdentity is the closed set of executable identities a release
// manifest is allowed to bind. Clean must come from the release build marker,
// not from ambient repository state.
type RuntimeIdentity struct {
	Version  string
	Revision string
	Clean    bool
	UIAssets adminui.AssetIdentity
}

// Report is safe to expose to authenticated console users. It deliberately
// excludes the local path and the manifest's artifact inventory.
type Report struct {
	Statement      string
	ManifestDigest string
}

type manifest struct {
	Schema          string          `json:"schema"`
	Version         string          `json:"version"`
	GeneratedAt     string          `json:"generated_at"`
	ServerRevision  string          `json:"server_revision"`
	SDK             sdkIdentity     `json:"sdk"`
	ContractVersion string          `json:"contract_version"`
	NATSVersion     string          `json:"nats_version"`
	WebUI           webUIIdentity   `json:"webui"`
	Platforms       []string        `json:"platforms"`
	Qualification   string          `json:"qualification"`
	Artifacts       []artifactEntry `json:"artifacts"`
}

type sdkIdentity struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

type webUIIdentity struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
	FileCount int    `json:"file_count"`
}

type artifactEntry struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// Load verifies an explicitly configured local release manifest against the
// running executable. An empty path means that qualification is unreported.
func Load(path string, runtime RuntimeIdentity) (*Report, error) {
	if path == "" {
		return nil, nil
	}
	if !runtime.Clean || !revisionPattern.MatchString(runtime.Revision) {
		return nil, errors.New("release manifest requires a revision-bound clean release build")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open release manifest: %w", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxManifestSize+1))
	if err != nil {
		return nil, fmt.Errorf("read release manifest: %w", err)
	}
	if len(raw) > maxManifestSize {
		return nil, errors.New("release manifest exceeds 1 MiB")
	}
	manifestBytes := raw
	decodeBytes := bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	decoder := json.NewDecoder(bytes.NewReader(decodeBytes))
	decoder.DisallowUnknownFields()
	var value manifest
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode release manifest: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, err
	}
	if value.Schema != manifestSchema || value.Version != runtime.Version || value.ServerRevision != runtime.Revision {
		return nil, errors.New("release manifest identity does not match this executable")
	}
	if value.WebUI.Algorithm != runtime.UIAssets.Algorithm || value.WebUI.Digest != runtime.UIAssets.Digest || value.WebUI.FileCount != runtime.UIAssets.FileCount {
		return nil, errors.New("release manifest WebUI identity does not match embedded assets")
	}
	if err := validateStructure(value); err != nil {
		return nil, err
	}
	if value.Qualification == "" || value.Qualification != strings.TrimSpace(value.Qualification) || len(value.Qualification) > 512 {
		return nil, errors.New("release manifest qualification statement is invalid")
	}
	digest := sha256.Sum256(manifestBytes)
	return &Report{Statement: value.Qualification, ManifestDigest: hex.EncodeToString(digest[:])}, nil
}

func validateStructure(value manifest) error {
	if _, err := time.Parse(time.RFC3339Nano, value.GeneratedAt); err != nil {
		return errors.New("release manifest generation time is invalid")
	}
	if value.SDK.Version == "" || !revisionPattern.MatchString(value.SDK.Revision) || value.ContractVersion == "" || value.NATSVersion == "" {
		return errors.New("release manifest dependency identity is invalid")
	}
	if len(value.Platforms) == 0 || len(value.Artifacts) == 0 {
		return errors.New("release manifest platforms and artifacts are required")
	}
	seenPlatforms := make(map[string]struct{}, len(value.Platforms))
	for _, platform := range value.Platforms {
		if platform == "" {
			return errors.New("release manifest contains an invalid platform")
		}
		if _, exists := seenPlatforms[platform]; exists {
			return errors.New("release manifest contains a duplicate platform")
		}
		seenPlatforms[platform] = struct{}{}
	}
	seenPaths := make(map[string]struct{}, len(value.Artifacts))
	for _, artifact := range value.Artifacts {
		if artifact.Path == "" || strings.HasPrefix(artifact.Path, "/") || strings.Contains(artifact.Path, "\\") || strings.Contains("/"+artifact.Path+"/", "/../") || artifact.Bytes < 0 || len(artifact.SHA256) != 64 {
			return errors.New("release manifest contains an invalid artifact")
		}
		if _, err := hex.DecodeString(artifact.SHA256); err != nil {
			return errors.New("release manifest contains an invalid artifact digest")
		}
		if _, exists := seenPaths[artifact.Path]; exists {
			return errors.New("release manifest contains a duplicate artifact path")
		}
		seenPaths[artifact.Path] = struct{}{}
	}
	return nil
}

func requireEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("release manifest must contain exactly one JSON document")
		}
		return fmt.Errorf("decode release manifest trailing data: %w", err)
	}
	return nil
}
