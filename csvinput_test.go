package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadCSVWithHeader(t *testing.T) {
	content := "\ufeffTrack name;Artist name;Album\r\n" +
		"Piazza Grande;Lucio Dalla;Gli anni settanta\r\n" +
		"La Primavera;Manu Chao;Baionarena\r\n"
	p := writeTemp(t, "test.csv", content)

	rows, delim, err := readCSV(p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if delim != ';' {
		t.Errorf("separatore = %q, atteso ';'", delim)
	}
	if len(rows) != 2 {
		t.Fatalf("righe = %d, attese 2", len(rows))
	}
	if rows[0] != (Row{Track: "Piazza Grande", Artist: "Lucio Dalla", Album: "Gli anni settanta"}) {
		t.Errorf("riga 0 inattesa: %+v", rows[0])
	}
}

func TestReadCSVCommaAndHeaderVariants(t *testing.T) {
	content := "Title,Artist,Album\n" +
		"Gianna,Rino Gaetano,Anni 70\n"
	p := writeTemp(t, "test.csv", content)

	rows, delim, err := readCSV(p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if delim != ',' {
		t.Errorf("separatore = %q, atteso ','", delim)
	}
	if len(rows) != 1 || rows[0].Track != "Gianna" {
		t.Fatalf("righe inattese: %+v", rows)
	}
}

func TestReadCSVNoHeader(t *testing.T) {
	content := "Gianna;Rino Gaetano;Anni 70\nPazza idea;Patty Pravo;Le Più Belle Canzoni\n"
	p := writeTemp(t, "test.csv", content)

	rows, _, err := readCSV(p, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("righe = %d, attese 2", len(rows))
	}
	if rows[1].Track != "Pazza idea" || rows[1].Album != "Le Più Belle Canzoni" {
		t.Errorf("riga 1 inattesa: %+v", rows[1])
	}
}

func TestReadCSVTabDelimiterEscape(t *testing.T) {
	content := "Track name\tArtist name\tAlbum\nGianna\tRino Gaetano\tAnni 70\n"
	p := writeTemp(t, "test.tsv", content)

	rows, delim, err := readCSV(p, "\\t", false)
	if err != nil {
		t.Fatal(err)
	}
	if delim != '\t' {
		t.Errorf("separatore = %q, atteso TAB", delim)
	}
	if len(rows) != 1 || rows[0].Track != "Gianna" || rows[0].Album != "Anni 70" {
		t.Fatalf("righe inattese: %+v", rows)
	}
}

func TestReadCSVEmpty(t *testing.T) {
	p := writeTemp(t, "test.csv", "Track name;Artist name;Album\n")
	if _, _, err := readCSV(p, "", false); err == nil {
		t.Fatal("atteso errore per CSV senza brani")
	}
}

func TestDedupeRows(t *testing.T) {
	rows := []Row{
		{Track: "Gianna", Artist: "Rino Gaetano", Album: "Anni 70"},
		{Track: "gianna", Artist: "RINO GAETANO", Album: "anni 70"},
		{Track: "Pazza idea", Artist: "Patty Pravo", Album: "Best"},
	}
	unique, mapping := dedupeRows(rows)
	if len(unique) != 2 {
		t.Fatalf("uniche = %d, attese 2", len(unique))
	}
	if len(mapping) != 3 || mapping[0] != mapping[1] || mapping[2] == mapping[0] {
		t.Errorf("mapping inatteso: %v", mapping)
	}
}

func TestWriteReports(t *testing.T) {
	dir := t.TempDir()
	entries := []Entry{
		{Row: Row{Track: "Gianna", Artist: "Rino Gaetano", Album: "Anni 70"}, Song: &Song{Title: "Gianna", Artist: "Rino Gaetano", Album: "La storia"}, Status: "ESATTO"},
		{Row: Row{Track: "Assente", Artist: "Nessuno", Album: "Mai"}, Status: "NON TROVATO"},
		{Row: Row{Track: "Saltato", Artist: "Nessuno", Album: "Mai"}, Status: "SALTATO (tua scelta)"},
	}

	all, miss, err := writeReports(dir, "Test/Playlist", entries, false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(all) != "Playlist Test-Playlist - abbinamenti.csv" {
		t.Errorf("nome file inatteso: %s", all)
	}

	raw, err := os.ReadFile(all)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "\ufeff") {
		t.Error("BOM UTF-8 mancante")
	}
	if !strings.Contains(string(raw), "Gianna;Rino Gaetano;Anni 70;ESATTO;Gianna;Rino Gaetano;La storia") {
		t.Error("riga abbinamento mancante o formato errato")
	}
	if !strings.Contains(string(raw), "\r\n") {
		t.Error("righe non terminate con CRLF")
	}

	rawMiss, err := os.ReadFile(miss)
	if err != nil {
		t.Fatal(err)
	}
	text := string(rawMiss)
	if !strings.Contains(text, "Assente") || !strings.Contains(text, "Saltato") {
		t.Errorf("file non trovati incompleto: %s", text)
	}
	if strings.Contains(text, "Gianna") {
		t.Error("il file non trovati non deve contenere i brani abbinati")
	}
}
