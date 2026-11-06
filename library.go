package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"
)

const cacheTTL = 12 * time.Hour

type libraryCache struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Songs     []Song    `json:"songs"`
}

func libraryCachePath(baseURL, user string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	sum := sha1.Sum([]byte(baseURL + "|" + user))
	name := "library-" + hex.EncodeToString(sum[:6]) + ".json"
	return filepath.Join(dir, "navidrome-import", name), nil
}

func loadLibraryCache(path string) ([]Song, time.Time, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	var c libraryCache
	if err := json.Unmarshal(b, &c); err != nil || len(c.Songs) == 0 {
		return nil, time.Time{}, false
	}
	if c.FetchedAt.IsZero() || time.Since(c.FetchedAt) > cacheTTL {
		return nil, time.Time{}, false
	}
	return c.Songs, c.FetchedAt, true
}

func saveLibraryCache(path string, songs []Song) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(libraryCache{FetchedAt: time.Now(), Songs: songs})
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// fetchLibrary scarica l'intera libreria (artisti -> album -> brani).
func fetchLibrary(c *Client) ([]Song, error) {
	artists, err := c.GetArtists()
	if err != nil {
		return nil, err
	}
	if len(artists) == 0 {
		return nil, fmt.Errorf("nessun artista trovato nella libreria")
	}
	statusf("Trovati %d artisti, recupero gli album...", len(artists))

	albums := make([][]albumRef, len(artists))
	err = runParallel(len(artists), 8, func(i int) error {
		al, err := c.GetArtist(artists[i].ID)
		if err != nil {
			return fmt.Errorf("artista %q: %w", artists[i].Name, err)
		}
		albums[i] = al
		return nil
	}, func(done int) {
		statusf("Album: %d/%d artisti", done, len(artists))
	})
	if err != nil {
		return nil, err
	}

	var all []albumRef
	for _, al := range albums {
		all = append(all, al...)
	}
	statusf("Trovati %d album, recupero i brani...", len(all))

	songLists := make([][]Song, len(all))
	err = runParallel(len(all), 8, func(i int) error {
		ss, err := c.GetAlbum(all[i].ID)
		if err != nil {
			return fmt.Errorf("album %q: %w", all[i].Name, err)
		}
		songLists[i] = ss
		return nil
	}, func(done int) {
		statusf("Brani: %d/%d album", done, len(all))
	})
	if err != nil {
		return nil, err
	}
	clearStatus()

	var songs []Song
	for _, ss := range songLists {
		for _, s := range ss {
			if s.ID == "" || s.Title == "" {
				continue
			}
			songs = append(songs, s)
		}
	}
	if len(songs) == 0 {
		return nil, fmt.Errorf("nessun brano trovato nella libreria")
	}
	return songs, nil
}

func runParallel(total, workers int, fn func(i int) error, tick func(done int)) error {
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	errCh := make(chan error, total)
	var done int64
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := fn(i); err != nil {
				errCh <- err
				return
			}
			if tick != nil {
				n := atomic.AddInt64(&done, 1)
				if n%10 == 0 || int(n) == total {
					tick(int(n))
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

var statusMu sync.Mutex

// statusTTY evita di riempire i log con sequenze ANSI quando l'output
// non è un terminale (es. redirect su file o pipe).
var statusTTY = term.IsTerminal(int(os.Stderr.Fd()))

func statusf(format string, args ...interface{}) {
	if !statusTTY {
		return
	}
	statusMu.Lock()
	defer statusMu.Unlock()
	fmt.Fprintf(os.Stderr, "\r\033[K"+format, args...)
}

func clearStatus() {
	if !statusTTY {
		return
	}
	statusMu.Lock()
	defer statusMu.Unlock()
	fmt.Fprint(os.Stderr, "\r\033[K")
}
