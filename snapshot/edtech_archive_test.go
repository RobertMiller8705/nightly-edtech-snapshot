package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveDirectoryIsDeterministic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "enrollments.jsonl")
	if err := os.WriteFile(path, []byte("{\"student_id\":\"s-17\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := ArchiveDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	changed := time.Date(2026, time.August, 3, 2, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatal(err)
	}
	second, err := ArchiveDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("archive changed when only file timestamps changed")
	}
}

func TestObjectKeyUsesUTCDate(t *testing.T) {
	date := time.Date(2026, time.August, 3, 23, 30, 0, 0, time.FixedZone("local", 8*60*60))
	key, err := ObjectKey("learning-records", date)
	if err != nil {
		t.Fatal(err)
	}
	if key != "snapshots/learning-records/2026-08-03.tar.gz" {
		t.Fatalf("unexpected key: %s", key)
	}
}
