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
		t.Errorf("delimiter = %q, want ';'", delim)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0] != (Row{Track: "Piazza Grande", Artist: "Lucio Dalla", Album: "Gli anni settanta"}) {
		t.Errorf("unexpected row 0: %+v", rows[0])
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
		t.Errorf("delimiter = %q, want ','", delim)
	}
	if len(rows) != 1 || rows[0].Track != "Gianna" {
		t.Fatalf("unexpected rows: %+v", rows)
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
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[1].Track != "Pazza idea" || rows[1].Album != "Le Più Belle Canzoni" {
		t.Errorf("unexpected row 1: %+v", rows[1])
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
		t.Errorf("delimiter = %q, want TAB", delim)
	}
	if len(rows) != 1 || rows[0].Track != "Gianna" || rows[0].Album != "Anni 70" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestReadCSVEmpty(t *testing.T) {
	p := writeTemp(t, "test.csv", "Track name;Artist name;Album\n")
	if _, _, err := readCSV(p, "", false); err == nil {
		t.Fatal("expected an error for a CSV with no tracks")
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
		t.Fatalf("unique rows = %d, want 2", len(unique))
	}
	if len(mapping) != 3 || mapping[0] != mapping[1] || mapping[2] == mapping[0] {
		t.Errorf("unexpected mapping: %v", mapping)
	}
}

func TestWriteReports(t *testing.T) {
	dir := t.TempDir()
	entries := []Entry{
		{RowNum: 1, Row: Row{Track: "Gianna", Artist: "Rino Gaetano", Album: "Anni 70"}, Song: &Song{Title: "Gianna", Artist: "Rino Gaetano", Album: "La storia"}, Status: StatusExact},
		{RowNum: 2, Row: Row{Track: "Assente", Artist: "Nessuno", Album: "Mai"}, Status: StatusNotFound},
		{RowNum: 3, Row: Row{Track: "Saltato", Artist: "Nessuno", Album: "Mai"}, Status: StatusSkipped},
	}

	all, miss, err := writeReports(dir, "Test/Playlist", entries, false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(all) != "Playlist Test-Playlist - matches.csv" {
		t.Errorf("unexpected file name: %s", all)
	}

	raw, err := os.ReadFile(all)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "\ufeff") {
		t.Error("missing UTF-8 BOM")
	}
	if !strings.Contains(string(raw), "Row;CSV Title;CSV Artist;CSV Album;Status") {
		t.Error("missing header row")
	}
	if !strings.Contains(string(raw), "1;Gianna;Rino Gaetano;Anni 70;EXACT;Gianna;Rino Gaetano;La storia") {
		t.Error("missing match row or wrong format")
	}
	if !strings.Contains(string(raw), "\r\n") {
		t.Error("rows not terminated with CRLF")
	}

	rawMiss, err := os.ReadFile(miss)
	if err != nil {
		t.Fatal(err)
	}
	text := string(rawMiss)
	if !strings.Contains(text, "Assente") || !strings.Contains(text, "Saltato") {
		t.Errorf("not-found file is incomplete: %s", text)
	}
	if strings.Contains(text, "Gianna") {
		t.Error("the not-found file must not contain matched tracks")
	}
	if !strings.Contains(text, "2;Assente;Nessuno;Mai;NOT FOUND") {
		t.Errorf("not-found file is missing the row number or the status: %s", text)
	}
}
