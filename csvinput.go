package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Row è una riga del CSV di input.
type Row struct {
	Track  string
	Artist string
	Album  string
}

// detectDelimiter sceglie il separatore contando ; , e TAB nella prima riga.
func detectDelimiter(line string) rune {
	counts := map[rune]int{';': 0, ',': 0, '\t': 0, '|': 0}
	inQuotes := false
	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ';', ',', '\t', '|':
			if !inQuotes {
				counts[r]++
			}
		}
	}
	best := ';'
	max := 0
	for _, r := range []rune{';', ',', '\t', '|'} {
		if counts[r] > max {
			max = counts[r]
			best = r
		}
	}
	return best
}

func firstLine(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	s := string(buf[:n])
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s, nil
}

func normalizeHeader(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Join(strings.Fields(s), " ")
}

func columnIndex(header []string, keys ...string) int {
	for i, h := range header {
		for _, k := range keys {
			if h == k {
				return i
			}
		}
	}
	return -1
}

func isHeader(rec []string) bool {
	if len(rec) < 2 {
		return false
	}
	h := make([]string, len(rec))
	for i, c := range rec {
		h[i] = normalizeHeader(c)
	}
	track := columnIndex(h, "track name", "track", "title", "song", "brano", "nome")
	artist := columnIndex(h, "artist name", "artist", "artista", "interprete")
	return track >= 0 && artist >= 0
}

// readCSV legge il file e restituisce le righe. Restituisce anche il separatore usato.
func readCSV(path string, delimiter string, forceNoHeader bool) ([]Row, rune, error) {
	line, err := firstLine(path)
	if err != nil {
		return nil, 0, err
	}

	var comma rune
	if delimiter != "" {
		switch delimiter {
		case "\\t", "tab", "TAB":
			delimiter = "\t"
		}
		r, size := utf8.DecodeRuneInString(delimiter)
		if r == utf8.RuneError || size == 0 {
			return nil, 0, fmt.Errorf("separatore non valido: %q", delimiter)
		}
		comma = r
	} else {
		comma = detectDelimiter(line)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, comma, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = comma
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, comma, fmt.Errorf("lettura CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, comma, errors.New("file CSV vuoto")
	}

	start := 0
	ti, ai, li := 0, 1, 2
	if !forceNoHeader && isHeader(records[0]) {
		h := make([]string, len(records[0]))
		for i, c := range records[0] {
			h[i] = normalizeHeader(c)
		}
		ti = columnIndex(h, "track name", "track", "title", "song", "brano", "nome")
		ai = columnIndex(h, "artist name", "artist", "artista", "interprete")
		li = columnIndex(h, "album", "album name", "disco")
		if ti < 0 || ai < 0 {
			return nil, comma, errors.New("intestazione non riconosciuta: servono le colonne Track name e Artist name")
		}
		start = 1
	}

	var rows []Row
	for _, rec := range records[start:] {
		if len(rec) == 0 {
			continue
		}
		row := Row{}
		if ti < len(rec) {
			row.Track = strings.TrimSpace(strings.TrimPrefix(rec[ti], "\ufeff"))
		}
		if ai < len(rec) {
			row.Artist = strings.TrimSpace(rec[ai])
		}
		if li >= 0 && li < len(rec) {
			row.Album = strings.TrimSpace(rec[li])
		}
		if row.Track == "" && row.Artist == "" {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, comma, errors.New("nessun brano trovato nel CSV")
	}
	return rows, comma, nil
}
