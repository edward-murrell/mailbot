package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ekm/mailbot/internal/config"
	"github.com/ekm/mailbot/internal/submission"
)

// FileStore writes each submission as a plain-text file in a directory.
type FileStore struct {
	cfg config.StorageConfig
}

// NewFileStore assembles a FileStore. The caller is responsible for ensuring
// cfg.Dir exists before calling Save (see bootstrap.MakeFileStore).
func NewFileStore(cfg config.StorageConfig) *FileStore {
	return &FileStore{cfg: cfg}
}

// Save writes the submission to a new file in the storage directory.
// The write is atomic: content is first written to a .tmp file and then
// renamed to the final path, ensuring no partial file is ever visible.
func (fs *FileStore) Save(ctx context.Context, s submission.Submission) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("store save: %w", err)
	}

	name := submission.Filename(s)
	finalPath := filepath.Join(fs.cfg.Dir, name)
	tmpPath := finalPath + ".tmp"

	if err := os.WriteFile(tmpPath, []byte(submission.Format(s)), 0o644); err != nil {
		return fmt.Errorf("store write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath) // best-effort cleanup
		return fmt.Errorf("store rename: %w", err)
	}
	return nil
}
