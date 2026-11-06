package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigVariants(t *testing.T) {
	content := `# commento
; altro commento
URL = "http://server:4533"
Utente = mario
password = 'segreta'
chiave-ignorata = valore
`
	p := writeTemp(t, "config.conf", content)
	c, err := loadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.URL != "http://server:4533" || c.User != "mario" || c.Password != "segreta" {
		t.Errorf("configurazione inattesa: %+v", c)
	}
}

func TestSaveAndReloadConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "config.conf")
	in := &Config{URL: "http://127.0.0.1:45899", User: "smanocch", Password: "test"}
	if err := saveConfig(p, in); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permessi = %o, attesi 600", perm)
	}

	out, err := loadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if *out != *in {
		t.Errorf("rilettura = %+v, atteso %+v", out, in)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := loadConfig(filepath.Join(t.TempDir(), "assente.conf")); !os.IsNotExist(err) {
		t.Errorf("errore = %v, atteso os.IsNotExist", err)
	}
}
