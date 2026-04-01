package store_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ekm/mailbot/internal/config"
	"github.com/ekm/mailbot/internal/store"
	"github.com/ekm/mailbot/internal/submission"
)

var fixedTime = time.Date(2026, 3, 31, 14, 5, 22, 0, time.UTC)

func TestFileStore_Save(t *testing.T) {
	dir := t.TempDir()
	fs := store.NewFileStore(config.StorageConfig{Dir: dir})
	sub := submission.New("Jane Smith", "jane@example.com", "+61400000000", "Website inquiry", "Hello there", "Support", "", fixedTime)

	if err := fs.Save(context.Background(), sub); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	name := entries[0].Name()
	if matched, _ := regexp.MatchString(`^\d{8}-\d{6}-[a-z0-9]{6}\.txt$`, name); !matched {
		t.Errorf("unexpected filename %q", name)
	}

	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != submission.Format(sub) {
		t.Errorf("file content mismatch\ngot:\n%s\nwant:\n%s", content, submission.Format(sub))
	}
}

func TestFileStore_Save_NoTmpFileLeft(t *testing.T) {
	dir := t.TempDir()
	fs := store.NewFileStore(config.StorageConfig{Dir: dir})
	sub := submission.New("", "jane@example.com", "", "Hi", "", "", "", fixedTime)

	if err := fs.Save(context.Background(), sub); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("unexpected .tmp file left behind: %s", e.Name())
		}
	}
}

func TestFileStore_Save_ContextCancelled(t *testing.T) {
	fs := store.NewFileStore(config.StorageConfig{Dir: t.TempDir()})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sub := submission.New("", "jane@example.com", "", "Hi", "", "", "", fixedTime)
	if err := fs.Save(ctx, sub); err == nil {
		t.Error("expected error with cancelled context, got nil")
	}
}
