package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var errQuit = errors.New("operazione annullata dall'utente")

// Option è una scelta mostrata all'utente.
type Option struct {
	Song     Song
	Score    float64
	HasScore bool
}

// UI gestisce l'interazione da terminale.
type UI struct {
	in     *bufio.Reader
	out    io.Writer
	client *Client
}

func newUI(client *Client) *UI {
	return &UI{
		in:     bufio.NewReader(os.Stdin),
		out:    os.Stdout,
		client: client,
	}
}

func (u *UI) readLine(prompt string) (string, error) {
	fmt.Fprint(u.out, prompt)
	line, err := u.in.ReadString('\n')
	if err != nil && line == "" {
		if errors.Is(err, io.EOF) {
			return "", errQuit
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func formatSong(s *Song) string {
	var b strings.Builder
	b.WriteString(s.Title)
	if s.Artist != "" {
		b.WriteString(" — ")
		b.WriteString(s.Artist)
	}
	if s.Album != "" {
		b.WriteString(" [ ")
		b.WriteString(s.Album)
		b.WriteString(" ]")
	}
	return b.String()
}

func (u *UI) printOptions(opts []Option) {
	for i, o := range opts {
		fmt.Fprintf(u.out, "  %d) %s", i+1, formatSong(&o.Song))
		if o.HasScore {
			fmt.Fprintf(u.out, "  (%.2f)", o.Score)
		}
		fmt.Fprintln(u.out)
	}
}

func optionsFromCandidates(songs []Song, cands []Candidate) []Option {
	opts := make([]Option, 0, len(cands))
	for _, c := range cands {
		if c.Index < 0 || c.Index >= len(songs) {
			continue
		}
		opts = append(opts, Option{Song: songs[c.Index], Score: c.Score, HasScore: true})
	}
	return opts
}

// AskSong mostra i candidati e restituisce il brano scelto.
// skip=true significa che l'utente ha scelto di saltare la riga.
func (u *UI) AskSong(header string, opts []Option) (*Song, bool, error) {
	fmt.Fprintln(u.out)
	fmt.Fprintln(u.out, header)
	if len(opts) == 0 {
		fmt.Fprintln(u.out, "  Nessun candidato trovato in libreria.")
	} else {
		u.printOptions(opts)
	}
	fmt.Fprintln(u.out, "  s = salta   c <testo> = cerca   i <id> = inserisci ID   q = esci")

	for {
		if len(opts) > 0 {
			fmt.Fprintf(u.out, "Scelta [1]: ")
		} else {
			fmt.Fprintf(u.out, "Scelta [s]: ")
		}
		line, err := u.in.ReadString('\n')
		if err != nil && line == "" {
			if errors.Is(err, io.EOF) {
				return nil, false, errQuit
			}
			return nil, false, err
		}
		line = strings.TrimSpace(line)
		low := strings.ToLower(line)

		switch {
		case line == "":
			if len(opts) > 0 {
				s := opts[0].Song
				return &s, false, nil
			}
			return nil, true, nil

		case low == "s" || low == "salta" || low == "skip":
			return nil, true, nil

		case low == "q" || low == "esci" || low == "quit":
			return nil, false, errQuit

		case low == "c" || low == "cerca" || strings.HasPrefix(low, "c ") || strings.HasPrefix(low, "cerca "):
			var query string
			switch {
			case strings.HasPrefix(low, "cerca "):
				query = strings.TrimSpace(line[len("cerca "):])
			case strings.HasPrefix(low, "c "):
				query = strings.TrimSpace(line[len("c "):])
			}
			if query == "" {
				query, err = u.readLine("Testo da cercare: ")
				if err != nil {
					return nil, false, err
				}
			}
			if query == "" {
				continue
			}
			found, err := u.client.Search(query)
			if err != nil {
				fmt.Fprintf(u.out, "  Errore nella ricerca: %v\n", err)
				continue
			}
			if len(found) == 0 {
				fmt.Fprintln(u.out, "  Nessun risultato.")
				continue
			}
			opts = make([]Option, 0, len(found))
			for _, s := range found {
				opts = append(opts, Option{Song: s})
			}
			fmt.Fprintln(u.out, "  Risultati della ricerca:")
			u.printOptions(opts)

		case low == "i" || strings.HasPrefix(low, "i "):
			id := strings.TrimSpace(line[1:])
			if id == "" {
				id, err = u.readLine("ID del brano: ")
				if err != nil {
					return nil, false, err
				}
			}
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			s, err := u.client.GetSong(id)
			if err != nil {
				fmt.Fprintf(u.out, "  ID non valido: %v\n", err)
				continue
			}
			return s, false, nil

		default:
			n, err := strconv.Atoi(line)
			if err == nil && n >= 1 && n <= len(opts) {
				s := opts[n-1].Song
				return &s, false, nil
			}
			fmt.Fprintln(u.out, "  Scelta non valida.")
		}
	}
}

// DupChoice è la scelta fatta su un doppione.
type DupChoice int

const (
	KeepFirst DupChoice = iota
	KeepSecond
	KeepBoth
	SkipBoth
)

// AskDuplicate chiede quale versione tenere quando lo stesso titolo+artista
// compare su album diversi.
func (u *UI) AskDuplicate(first, second Entry) (DupChoice, error) {
	fmt.Fprintln(u.out)
	fmt.Fprintf(u.out, "Doppione: «%s» — %s\n", first.Song.Title, first.Song.Artist)
	fmt.Fprintf(u.out, "  1) %s\n     già in playlist (riga CSV: «%s» — %s [ %s ])\n",
		formatSong(first.Song), first.Row.Track, first.Row.Artist, first.Row.Album)
	fmt.Fprintf(u.out, "  2) %s\n     nuovo (riga CSV: «%s» — %s [ %s ])\n",
		formatSong(second.Song), second.Row.Track, second.Row.Artist, second.Row.Album)
	fmt.Fprintln(u.out, "  Scelta: 1 = tieni 1, 2 = tieni 2, t = tieni entrambi, s = salta entrambi, q = esci")

	for {
		line, err := u.readLine("Scelta [1]: ")
		if err != nil {
			return KeepFirst, err
		}
		switch strings.ToLower(line) {
		case "", "1":
			return KeepFirst, nil
		case "2":
			return KeepSecond, nil
		case "t", "tutti", "entrambi":
			return KeepBoth, nil
		case "s", "salta":
			return SkipBoth, nil
		case "q", "esci":
			return KeepFirst, errQuit
		default:
			fmt.Fprintln(u.out, "  Scelta non valida.")
		}
	}
}

// ExistingAction è l'azione scelta per una playlist già esistente.
type ExistingAction struct {
	Kind    string // update | duplicate | rename | quit
	NewName string
}

func (u *UI) AskExistingPlaylist(name string, count int) (ExistingAction, error) {
	fmt.Fprintln(u.out)
	fmt.Fprintf(u.out, "Esiste già una playlist chiamata «%s» (%s).\n", name, brani(count))
	fmt.Fprintln(u.out, "  a = aggiorna (sostituisce i brani)")
	fmt.Fprintln(u.out, "  d = duplica (crea una nuova playlist «nome (2)»)")
	fmt.Fprintln(u.out, "  r = rinomina la nuova playlist")
	fmt.Fprintln(u.out, "  q = esci senza modificare")

	for {
		line, err := u.readLine("Scelta [a]: ")
		if err != nil {
			return ExistingAction{}, err
		}
		switch strings.ToLower(line) {
		case "", "a", "aggiorna":
			return ExistingAction{Kind: "update"}, nil
		case "d", "duplica":
			return ExistingAction{Kind: "duplicate"}, nil
		case "r", "rinomina":
			newName, err := u.readLine("Nuovo nome della playlist: ")
			if err != nil {
				return ExistingAction{}, err
			}
			if strings.TrimSpace(newName) == "" {
				fmt.Fprintln(u.out, "  Nome non valido.")
				continue
			}
			return ExistingAction{Kind: "rename", NewName: strings.TrimSpace(newName)}, nil
		case "q", "esci":
			return ExistingAction{Kind: "quit"}, nil
		default:
			fmt.Fprintln(u.out, "  Scelta non valida.")
		}
	}
}
