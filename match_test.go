package main

import (
	"math"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Caffè nero bollente (2021 Remaster)", "caffe nero bollente 2021 remaster"},
		{"E' tutto un attimo", "e tutto un attimo"},
		{"Simon & Garfunkel", "simon and garfunkel"},
		{"  Spazi   multipli  ", "spazi multipli"},
		{"L’odore della morte", "lodore della morte"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVariants(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, v := range list {
			if v == want {
				return true
			}
		}
		return false
	}

	got := variants("Titolo (Live)", "Album", "Artista")
	if !has(got, "titolo") || !has(got, "titolo live") {
		t.Errorf("unexpected variants: %v", got)
	}

	got = variants("Album - Titolo", "Album", "Artista")
	if !has(got, "titolo") {
		t.Errorf("album prefix not removed: %v", got)
	}

	got = variants("Brano pt. 1", "Album", "Artista")
	if !has(got, "brano part 1") {
		t.Errorf("pt. not converted to part: %v", got)
	}

	got = variants("Brano (feat. Qualcuno)", "Album", "Artista")
	if !has(got, "brano") {
		t.Errorf("feat not removed: %v", got)
	}
}

func TestRatioAndSim(t *testing.T) {
	cases := []struct {
		a, b string
		sim  float64
	}{
		{"piazza grande", "piazza grande", 1.0},
		{"il mio canto libero", "il nostro caro angelo", 0.55},
		{"la primavera", "la primavera live", 0.9},
		{"sei bellissima", "sei come un juke box", 0.294118},
	}
	for _, c := range cases {
		if got := sim(c.a, c.b); math.Abs(got-c.sim) > 1e-5 {
			t.Errorf("sim(%q, %q) = %.6f, want %.6f", c.a, c.b, got, c.sim)
		}
	}
	if got := sim("a", ""); got != 0 {
		t.Errorf("sim with empty string = %f, want 0", got)
	}
}

func TestMatcherSynthetic(t *testing.T) {
	songs := []Song{
		{ID: "1", Title: "Piazza Grande", Artist: "Lucio Dalla", Album: "Gli anni settanta - CD1"},
		{ID: "2", Title: "La Primavera", Artist: "Manu Chao", Album: "Baionarena"},
		{ID: "3", Title: "La Primavera", Artist: "Manu Chao", Album: "Próxima Estación... Esperanza"},
		{ID: "4", Title: "...dimmi che non vuoi morire (Live)", Artist: "Patty Pravo", Album: "Bye bye patty"},
	}
	m := NewMatcher(songs)

	rows := []Row{
		{Track: "Piazza Grande", Artist: "Lucio Dalla", Album: "Il meglio della musica italiana anni '70"},
		{Track: "La Primavera", Artist: "Manu Chao", Album: "Próxima Estación... Esperanza"},
		{Track: "Brano Inesistente", Artist: "Nessuno", Album: "Mai"},
	}
	res := m.Match(rows)

	if res[0].Class != "exact" || songs[res[0].Pick].ID != "1" {
		t.Errorf("row 0: class=%s pick=%d, want exact/1", res[0].Class, res[0].Pick)
	}
	if res[1].Class != "exact" || songs[res[1].Pick].ID != "3" {
		t.Errorf("row 1: class=%s pick=%d, want exact/3", res[1].Class, res[1].Pick)
	}
	if res[2].Class != "none" || res[2].Pick != -1 {
		t.Errorf("row 2: class=%s pick=%d, want none/-1", res[2].Class, res[2].Pick)
	}
}
