package main

import (
	"encoding/json"
	"os"
	"testing"
)

// Test di parità con la pipeline Python usata per validare la logica.
// Sono attivi solo se vengono impostate le variabili d'ambiente:
//
//	PARITY_CACHE        file JSON con l'array dei brani della libreria
//	PARITY_DIR          cartella con i file matches_*.json
//	PARITY_CSV_ITALIAN  percorso del CSV "Italian Beat.csv"
//	PARITY_CSV_ALLE     percorso del CSV "Allegrezza.csv"
//
// Senza queste variabili i test vengono saltati.

type pyResult struct {
	Track  string `json:"track"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Class  string `json:"cls"`
	Pick   *int   `json:"pick"`
}

func TestParity(t *testing.T) {
	cachePath := os.Getenv("PARITY_CACHE")
	dir := os.Getenv("PARITY_DIR")
	if cachePath == "" || dir == "" {
		t.Skip("imposta PARITY_CACHE e PARITY_DIR per eseguire i test di parità")
	}

	cases := []struct {
		name  string
		csv   string
		ref   string
		total int
	}{
		{"Italian Beat", os.Getenv("PARITY_CSV_ITALIAN"), "matches_italian.json", 100},
		{"Allegrezza", os.Getenv("PARITY_CSV_ALLE"), "matches_alle.json", 321},
	}

	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Skipf("cache non disponibile: %v", err)
	}
	var songs []Song
	if err := json.Unmarshal(raw, &songs); err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.csv == "" {
				t.Skip("CSV non impostato")
			}
			rawRef, err := os.ReadFile(dir + "/" + tc.ref)
			if err != nil {
				t.Skipf("riferimento non disponibile: %v", err)
			}
			var ref struct {
				Results []pyResult `json:"results"`
			}
			if err := json.Unmarshal(rawRef, &ref); err != nil {
				t.Fatal(err)
			}

			rows, _, err := readCSV(tc.csv, "", false)
			if err != nil {
				t.Fatal(err)
			}
			unique, _ := dedupeRows(rows)
			if len(unique) != tc.total {
				t.Fatalf("righe uniche: go=%d attese=%d", len(unique), tc.total)
			}

			m := NewMatcher(songs)
			got := m.Match(unique)

			counts := map[string]int{}
			for _, r := range got {
				counts[r.Class]++
			}
			t.Logf("go: %v", counts)

			if len(got) != len(ref.Results) {
				t.Fatalf("numero risultati diverso: go=%d python=%d", len(got), len(ref.Results))
			}
			diff := 0
			for i := range got {
				want := ref.Results[i]
				wantPick := -1
				if want.Pick != nil {
					wantPick = *want.Pick
				}
				if got[i].Class == want.Class && got[i].Pick == wantPick {
					continue
				}
				// Due ID diversi ma stesso titolo/artista/album sono lo stesso brano
				// duplicato in libreria: la scelta è equivalente.
				if got[i].Class == want.Class && got[i].Pick >= 0 && wantPick >= 0 {
					a, b := songs[got[i].Pick], songs[wantPick]
					if normalize(a.Title) == normalize(b.Title) &&
						normalize(a.Artist) == normalize(b.Artist) &&
						normalize(a.Album) == normalize(b.Album) {
						continue
					}
				}
				diff++
				t.Errorf("riga %d %q — %q: go=%s/pick=%d python=%s/pick=%d", i,
					want.Track, want.Artist, got[i].Class, got[i].Pick, want.Class, wantPick)
			}
			if diff > 0 {
				t.Fatalf("%d righe con esito diverso", diff)
			}
		})
	}
}
