package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Status values shown to the user in the terminal and in the CSV reports.
const (
	StatusExact          = "EXACT"
	StatusAutomatic      = "AUTOMATIC"
	StatusManual         = "MANUAL CHOICE"
	StatusAmbiguousFirst = "AMBIGUOUS (first choice)"
	StatusAmbiguousSkip  = "AMBIGUOUS (skipped)"
	StatusNotFound       = "NOT FOUND"
	StatusSkipped        = "SKIPPED (your choice)"
	StatusDuplicate      = "DUPLICATE"
	StatusDiscarded      = "DISCARDED (duplicate)"
)

// Entry is the final outcome for one CSV row.
type Entry struct {
	RowNum int // 1-based data row in the CSV
	Row    Row
	Song   *Song
	Status string
}

var reportHeader = []string{
	"Row", "CSV Title", "CSV Artist", "CSV Album", "Status",
	"Navidrome Title", "Navidrome Artist", "Navidrome Album",
}

var missingHeader = []string{
	"Row", "CSV Title", "CSV Artist", "CSV Album", "Status",
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

// writeReports writes the two CSV files (matches and not found) using ';' as
// separator and a UTF-8 BOM, so Excel opens them correctly.
func writeReports(dir, playlistName string, entries []Entry, preview bool) (string, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	name := sanitizeName(playlistName)
	base := filepath.Join(dir, "Playlist "+name)
	if preview {
		base += " - preview"
	}
	allPath := base + " - matches.csv"
	missPath := base + " - not found.csv"

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
		rowNum := strconv.Itoa(e.RowNum)
		row := []string{rowNum, e.Row.Track, e.Row.Artist, e.Row.Album, e.Status}
		if e.Song != nil {
			row = append(row, e.Song.Title, e.Song.Artist, e.Song.Album)
		} else {
			row = append(row, "", "", "")
		}
		all = append(all, row)

		switch e.Status {
		case StatusNotFound, StatusSkipped, StatusAmbiguousSkip:
			missing = append(missing, []string{rowNum, e.Row.Track, e.Row.Artist, e.Row.Album, e.Status})
		}
	}

	if err := write(allPath, reportHeader, all); err != nil {
		return "", "", fmt.Errorf("writing %s: %w", allPath, err)
	}
	if err := write(missPath, missingHeader, missing); err != nil {
		return "", "", fmt.Errorf("writing %s: %w", missPath, err)
	}
	return allPath, missPath, nil
}
