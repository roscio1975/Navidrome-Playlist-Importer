package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestUI(input string) (*UI, *strings.Builder) {
	out := &strings.Builder{}
	return &UI{
		in:  bufio.NewReader(strings.NewReader(input)),
		out: out,
	}, out
}

func TestAskSongNumericChoice(t *testing.T) {
	ui, out := newTestUI("2\n")
	opts := []Option{
		{Song: Song{ID: "1", Title: "Gianna"}, HasScore: true, Score: 0.9},
		{Song: Song{ID: "2", Title: "Pazza idea"}, HasScore: true, Score: 0.85},
	}
	s, skip, err := ui.AskSong("riga di prova", opts)
	if err != nil {
		t.Fatal(err)
	}
	if skip || s == nil || s.ID != "2" {
		t.Fatalf("scelta inattesa: song=%+v skip=%v", s, skip)
	}
	if !strings.Contains(out.String(), "1) Gianna") || !strings.Contains(out.String(), "(0.85)") {
		t.Errorf("output inatteso:\n%s", out.String())
	}
}

func TestAskSongDefaultAndSkip(t *testing.T) {
	ui, _ := newTestUI("\n")
	s, skip, err := ui.AskSong("riga", []Option{{Song: Song{ID: "1", Title: "Gianna"}}})
	if err != nil || skip || s == nil || s.ID != "1" {
		t.Fatalf("invio non sceglie il primo candidato: song=%+v skip=%v err=%v", s, skip, err)
	}

	ui, _ = newTestUI("s\n")
	s, skip, err = ui.AskSong("riga", nil)
	if err != nil || !skip || s != nil {
		t.Fatalf("s non salta: song=%+v skip=%v err=%v", s, skip, err)
	}

	ui, _ = newTestUI("q\n")
	if _, _, err = ui.AskSong("riga", nil); err != errQuit {
		t.Errorf("q non interrompe: %v", err)
	}
}

func TestAskSongInvalidThenValid(t *testing.T) {
	ui, out := newTestUI("99\nx\n1\n")
	s, skip, err := ui.AskSong("riga", []Option{{Song: Song{ID: "1", Title: "Gianna"}}})
	if err != nil || skip || s == nil || s.ID != "1" {
		t.Fatalf("scelta inattesa: song=%+v skip=%v err=%v", s, skip, err)
	}
	if strings.Count(out.String(), "Scelta non valida") != 2 {
		t.Errorf("messaggi di errore mancanti:\n%s", out.String())
	}
}

func TestAskSongSearchWithCercaPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/search3.view") {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `{"subsonic-response":{"status":"ok","searchResult3":{"song":[{"id":"abc","title":"Gianna","artist":"Rino Gaetano","album":"Anni 70"}]}}}`)
	}))
	defer srv.Close()

	ui, out := newTestUI("Cerca Gianna\n1\n")
	ui.client = &Client{Base: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	s, skip, err := ui.AskSong("riga", nil)
	if err != nil || skip || s == nil || s.ID != "abc" {
		t.Fatalf("ricerca fallita: song=%+v skip=%v err=%v", s, skip, err)
	}
	if !strings.Contains(out.String(), "Risultati della ricerca") {
		t.Errorf("output inatteso:\n%s", out.String())
	}
}

func TestAskSongManualID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/getSong.view") {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("id"); got != "xyz" {
			fmt.Fprintf(w, `{"subsonic-response":{"status":"failed","error":{"code":70,"message":"non trovato"}}}`)
			return
		}
		io.WriteString(w, `{"subsonic-response":{"status":"ok","song":{"id":"xyz","title":"Gianna"}}}`)
	}))
	defer srv.Close()

	ui, _ := newTestUI("i xyz\n")
	ui.client = &Client{Base: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	s, skip, err := ui.AskSong("riga", nil)
	if err != nil || skip || s == nil || s.ID != "xyz" {
		t.Fatalf("ID manuale fallito: song=%+v skip=%v err=%v", s, skip, err)
	}
}

func TestAskDuplicateChoices(t *testing.T) {
	first := Entry{Row: Row{Track: "La Primavera"}, Song: &Song{Title: "La Primavera", Artist: "Manu Chao", Album: "Baionarena"}}
	second := Entry{Row: Row{Track: "La Primavera"}, Song: &Song{Title: "La Primavera", Artist: "Manu CHAO", Album: "Esperanza"}}

	cases := []struct {
		input string
		want  DupChoice
	}{
		{"\n", KeepFirst},
		{"2\n", KeepSecond},
		{"t\n", KeepBoth},
		{"s\n", SkipBoth},
	}
	for _, c := range cases {
		ui, _ := newTestUI(c.input)
		got, err := ui.AskDuplicate(first, second)
		if err != nil || got != c.want {
			t.Errorf("input %q: scelta=%v err=%v, atteso %v", c.input, got, err, c.want)
		}
	}

	ui, _ := newTestUI("x\nq\n")
	if _, err := ui.AskDuplicate(first, second); err != errQuit {
		t.Errorf("q non interrompe: %v", err)
	}
}

func TestAskExistingPlaylist(t *testing.T) {
	cases := []struct {
		input string
		kind  string
		name  string
	}{
		{"\n", "update", ""},
		{"d\n", "duplicate", ""},
		{"r\nNuovo Nome\n", "rename", "Nuovo Nome"},
		{"r\n\nr\nNome 2\n", "rename", "Nome 2"},
		{"q\n", "quit", ""},
	}
	for _, c := range cases {
		ui, out := newTestUI(c.input)
		action, err := ui.AskExistingPlaylist("Italian Beat", 75)
		if err != nil || action.Kind != c.kind || action.NewName != c.name {
			t.Errorf("input %q: azione=%+v err=%v", c.input, action, err)
		}
		if !strings.Contains(out.String(), "1 brani") && !strings.Contains(out.String(), "75 brani") {
			t.Errorf("conteggio non mostrato:\n%s", out.String())
		}
	}
}

func TestBraniPlural(t *testing.T) {
	if brani(1) != "1 brano" || brani(0) != "0 brani" || brani(75) != "75 brani" {
		t.Errorf("pluralizzazione errata: %q %q %q", brani(1), brani(0), brani(75))
	}
}
