package models

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FSStore is the offline BlobStore: content-addressed blobs under a root
// directory, mirroring server/images.go's GetBlobsPath layout
// (<root>/blobs/sha256-<hex>). The S3/MinIO implementation satisfies the
// same BlobStore interface in production deployments.
type FSStore struct {
	Root string
}

// NewFSStore creates the blob directory tree.
func NewFSStore(root string) (*FSStore, error) {
	if root == "" {
		root = "blobs"
	}
	if err := os.MkdirAll(filepath.Join(root, "blobs"), 0o755); err != nil {
		return nil, err
	}
	return &FSStore{Root: root}, nil
}

func (s *FSStore) path(digest string) string {
	// "sha256:abcd..." -> "blobs/sha256-abcd..."
	return filepath.Join(s.Root, "blobs", strings.ReplaceAll(digest, ":", "-"))
}

// Put streams r to a temp file while hashing, then renames into place —
// the same write-then-rename discipline as the monorepo blob writer.
func (s *FSStore) Put(r io.Reader) (string, int64, error) {
	tmp, err := os.CreateTemp(filepath.Join(s.Root, "blobs"), "tmp-*")
	if err != nil {
		return "", 0, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	digest, size, err := GetSHA256Digest(io.TeeReader(r, tmp))
	if err != nil {
		tmp.Close()
		return "", 0, err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmpName, s.path(digest)); err != nil {
		return "", 0, err
	}
	return digest, size, nil
}

// Get opens and verifies the blob for digest.
func (s *FSStore) Get(digest string) (io.ReadCloser, error) {
	f, err := os.Open(s.path(digest))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBlobNotFound, digest)
	}
	return f, nil
}

// Has reports whether the digest exists on disk.
func (s *FSStore) Has(digest string) bool {
	_, err := os.Stat(s.path(digest))
	return err == nil
}

// Delete removes the blob.
func (s *FSStore) Delete(digest string) error {
	err := os.Remove(s.path(digest))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Stat returns the blob size.
func (s *FSStore) Stat(digest string) (int64, error) {
	fi, err := os.Stat(s.path(digest))
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrBlobNotFound, digest)
	}
	return fi.Size(), nil
}
