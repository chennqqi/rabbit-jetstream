package adminui

import (
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"
)

//go:embed dist/*
var assets embed.FS

// AssetIdentity identifies the exact closed file set embedded in the
// management binary. The digest frames each sorted relative path and its
// contents, so neither concatenation ambiguity nor filesystem walk order can
// change its meaning.
type AssetIdentity struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
	FileCount int    `json:"fileCount"`
}

func identifyAssets(files fs.FS) (AssetIdentity, error) {
	var names []string
	if err := fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			names = append(names, strings.TrimPrefix(path, "./"))
		}
		return nil
	}); err != nil {
		return AssetIdentity{}, err
	}
	sort.Strings(names)
	hash := sha256.New()
	var size [8]byte
	for _, name := range names {
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return AssetIdentity{}, err
		}
		binary.BigEndian.PutUint64(size[:], uint64(len(name)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(name))
		binary.BigEndian.PutUint64(size[:], uint64(len(content)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write(content)
	}
	return AssetIdentity{Algorithm: "sha256-framed-files-v1", Digest: fmt.Sprintf("%x", hash.Sum(nil)), FileCount: len(names)}, nil
}

var assetIdentity = sync.OnceValue(func() AssetIdentity {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	identity, err := identifyAssets(root)
	if err != nil {
		panic(err)
	}
	return identity
})

// EmbeddedAssetIdentity returns the identity of the files served by Handler.
func EmbeddedAssetIdentity() AssetIdentity { return assetIdentity() }

// Handler serves immutable embedded console assets and falls back to the app
// shell for client-side routes below /admin/.
func Handler() http.Handler {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/admin/")
		if path == "" {
			path = "index.html"
		}
		if strings.Contains(path, "..") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(root, path); err != nil {
			path = "index.html"
		}
		clone := r.Clone(r.Context())
		servePath := path
		if path == "index.html" {
			servePath = ""
		}
		clone.URL.Path = "/" + servePath
		if path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		files.ServeHTTP(w, clone)
	})
}
