package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLibraryCacheFresh(t *testing.T) {
	dir := t.TempDir()
	path, err := libraryCachePath("http://example", "utente")
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, filepath.Base(path))

	songs := []Song{{ID: "1", Title: "Gianna", Artist: "Rino Gaetano", Album: "Anni 70"}}
	if err := saveLibraryCache(path, songs); err != nil {
		t.Fatal(err)
	}

	got, fetchedAt, ok := loadLibraryCache(path)
	if !ok {
		t.Fatal("cache appena salvata non caricata")
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Errorf("brani inattesi: %+v", got)
	}
	if time.Since(fetchedAt) > time.Minute {
		t.Errorf("timestamp inatteso: %v", fetchedAt)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permessi cache = %o, attesi 600", perm)
	}
}

func TestLibraryCacheExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	old := libraryCache{
		FetchedAt: time.Now().Add(-cacheTTL - time.Minute),
		Songs:     []Song{{ID: "1", Title: "Gianna"}},
	}
	b, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, ok := loadLibraryCache(path); ok {
		t.Error("cache scaduta usata al posto del download")
	}
}

func TestLibraryCacheCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte("non json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := loadLibraryCache(path); ok {
		t.Error("cache corrotta accettata")
	}
}
