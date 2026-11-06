package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Entry è l'esito finale di una riga del CSV.
type Entry struct {
	Row    Row
	Song   *Song
	Status string
}

var reportHeader = []string{
	"Brano CSV", "Artista CSV", "Album CSV", "Stato",
	"Brano Navidrome", "Artista Navidrome", "Album Navidrome",
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, ":", "-")
	if s == "" {
		s = "playlist"
	}
	return s
}

// writeReports scrive i due CSV (abbinamenti e non trovati) con separatore ';'
// e BOM UTF-8, così Excel li apre correttamente.
func writeReports(dir, playlistName string, entries []Entry, preview bool) (string, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	name := sanitizeName(playlistName)
	base := filepath.Join(dir, "Playlist "+name)
	if preview {
		base += " - anteprima"
	}
	allPath := base + " - abbinamenti.csv"
	missPath := base + " - non trovati.csv"

	write := func(path string, header []string, rows [][]string) error {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		if _, err := f.WriteString("\ufeff"); err != nil {
			f.Close()
			return err
		}
		w := csv.NewWriter(f)
		w.Comma = ';'
		w.UseCRLF = true
		if err := w.Write(header); err != nil {
			f.Close()
			return err
		}
		if err := w.WriteAll(rows); err != nil {
			f.Close()
			return err
		}
		w.Flush()
		if err := w.Error(); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}

	all := make([][]string, 0, len(entries))
	missing := make([][]string, 0)
	for _, e := range entries {
		row := []string{e.Row.Track, e.Row.Artist, e.Row.Album, e.Status}
		if e.Song != nil {
			row = append(row, e.Song.Title, e.Song.Artist, e.Song.Album)
		} else {
			row = append(row, "", "", "")
		}
		all = append(all, row)

		switch e.Status {
		case "NON TROVATO", "SALTATO (tua scelta)", "AMBIGUO (saltato)":
			missing = append(missing, []string{e.Row.Track, e.Row.Artist, e.Row.Album})
		}
	}

	if err := write(allPath, reportHeader, all); err != nil {
		return "", "", fmt.Errorf("scrittura %s: %w", allPath, err)
	}
	if err := write(missPath, []string{"Brano", "Artista", "Album"}, missing); err != nil {
		return "", "", fmt.Errorf("scrittura %s: %w", missPath, err)
	}
	return allPath, missPath, nil
}
