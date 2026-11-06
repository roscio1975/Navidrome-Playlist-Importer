package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// version è sovrascrivibile in fase di build con -ldflags "-X main.version=..."
var version = "1.0.0"

// brani restituisce "1 brano" oppure "N brani".
func brani(n int) string {
	if n == 1 {
		return "1 brano"
	}
	return fmt.Sprintf("%d brani", n)
}

func main() {
	if err := run(); err != nil {
		if errors.Is(err, errQuit) {
			fmt.Println("\nOperazione annullata.")
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "\nErrore: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configFlag  = flag.String("config", "", "file di configurazione (default: cartella config utente)")
		csvFlag     = flag.String("csv", "", "file CSV da importare (alternativa al primo argomento)")
		nameFlag    = flag.String("name", "", "nome della playlist (default: nome del file CSV)")
		delimFlag   = flag.String("delimiter", "", "separatore CSV (default: rilevato automaticamente)")
		noHeader    = flag.Bool("no-header", false, "il CSV non ha riga di intestazione")
		refresh     = flag.Bool("refresh", false, "riscarica la libreria ignorando la cache")
		dryRun      = flag.Bool("dry-run", false, "mostra cosa verrebbe fatto senza modificare Navidrome")
		yesFlag     = flag.Bool("yes", false, "non interattivo: usa le scelte automatiche per tutti i casi dubbi")
		reportFlag  = flag.String("report-dir", "", "cartella dei report (default: cartella del CSV)")
		setupFlag   = flag.Bool("setup", false, "esegue la configurazione guidata e salva il file")
		versionFlag = flag.Bool("version", false, "mostra la versione")
	)
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Printf("navidrome-importer %s\n", version)
		return nil
	}

	cfgPath := *configFlag
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}

	if *setupFlag {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("la configurazione guidata richiede un terminale interattivo")
		}
		if _, err := setupConfig(cfgPath); err != nil {
			return err
		}
		return nil
	}

	csvPath := *csvFlag
	if csvPath == "" && flag.NArg() > 0 {
		csvPath = flag.Arg(0)
	}
	if csvPath == "" {
		flag.Usage()
		return errors.New("specifica il file CSV da importare")
	}

	cfg, err := loadOrSetupConfig(cfgPath)
	if err != nil {
		return err
	}

	client, err := NewClient(cfg.URL, cfg.User, cfg.Password)
	if err != nil {
		return err
	}

	interactive := !*yesFlag && term.IsTerminal(int(os.Stdin.Fd()))
	if !interactive && !*yesFlag {
		return errors.New("input non interattivo: usa --yes per eseguire senza domande")
	}

	fmt.Printf("Navidrome: %s (utente %s)\n", client.Base, client.User)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("impossibile connettersi a Navidrome: %w", err)
	}

	songs, err := getLibrary(client, *refresh)
	if err != nil {
		return err
	}

	rows, delim, err := readCSV(csvPath, *delimFlag, *noHeader)
	if err != nil {
		return err
	}
	fmt.Printf("CSV: %d righe, separatore %q\n", len(rows), string(delim))

	unique, mapping := dedupeRows(rows)
	matcher := NewMatcher(songs)
	uniqueResults := matcher.Match(unique)
	results := make([]MatchResult, len(rows))
	for i, u := range mapping {
		r := uniqueResults[u]
		r.Row = rows[i]
		results[i] = r
	}

	counts := map[string]int{}
	for _, r := range uniqueResults {
		counts[r.Class]++
	}
	fmt.Printf("Brani unici: %d — esatti: %d, automatici: %d, da decidere: %d, non trovati: %d\n",
		len(unique), counts["exact"], counts["auto"], counts["ambiguous"], counts["none"])

	playlistName := *nameFlag
	if playlistName == "" {
		base := filepath.Base(csvPath)
		playlistName = strings.TrimSuffix(base, filepath.Ext(base))
	}

	ui := newUI(client)
	entries, err := resolveAll(ui, songs, results, !interactive)
	if err != nil {
		return err
	}

	var songIDs []string
	seenID := map[string]bool{}
	for _, e := range entries {
		if e.Song == nil || !songIncluded(e.Status) {
			continue
		}
		if seenID[e.Song.ID] {
			continue
		}
		seenID[e.Song.ID] = true
		songIDs = append(songIDs, e.Song.ID)
	}

	reportDir := *reportFlag
	if reportDir == "" {
		reportDir = filepath.Dir(csvPath)
	}
	allPath, missPath, err := writeReports(reportDir, playlistName, entries, *dryRun)
	if err != nil {
		return err
	}

	if err := handlePlaylist(client, ui, playlistName, songIDs, *dryRun, *yesFlag); err != nil {
		return err
	}

	printSummary(entries, len(songIDs), allPath, missPath, *dryRun)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "navidrome-importer %s\n\n", version)
	fmt.Fprintln(os.Stderr, "Importa una playlist CSV in Navidrome con abbinamento fuzzy dei brani.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Uso:")
	fmt.Fprintln(os.Stderr, "  navidrome-importer [opzioni] <file.csv>")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Opzioni:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Esempi:")
	fmt.Fprintln(os.Stderr, `  navidrome-importer "Italian Beat.csv"`)
	fmt.Fprintln(os.Stderr, `  navidrome-importer --dry-run "Italian Beat.csv"`)
	fmt.Fprintln(os.Stderr, `  navidrome-importer --name "Metal" --refresh "Metal.csv"`)
}

func loadOrSetupConfig(path string) (*Config, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("lettura configurazione: %w", err)
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, fmt.Errorf("file di configurazione non trovato: %s\nEsegui `navidrome-importer --setup` da un terminale per crearlo", path)
		}
		cfg, err = setupConfig(path)
		if err != nil {
			return nil, err
		}
	}

	if v := os.Getenv("NAVIDROME_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("NAVIDROME_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("NAVIDROME_PASSWORD"); v != "" {
		cfg.Password = v
	}

	if cfg.URL == "" || cfg.User == "" || cfg.Password == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, fmt.Errorf("configurazione incompleta: %s\nServono url, user e password", path)
		}
		return setupConfig(path)
	}
	return cfg, nil
}

func getLibrary(client *Client, refresh bool) ([]Song, error) {
	cachePath, _ := libraryCachePath(client.Base, client.User)

	if !refresh && cachePath != "" {
		if songs, fetchedAt, ok := loadLibraryCache(cachePath); ok {
			fmt.Printf("Libreria: %s (cache del %s, usa --refresh per aggiornare)\n",
				brani(len(songs)), fetchedAt.Format("02/01/2006 15:04"))
			return songs, nil
		}
	}

	fmt.Println("Libreria: scarico l'elenco completo da Navidrome (può richiedere qualche minuto)...")
	songs, err := fetchLibrary(client)
	if err != nil {
		return nil, err
	}
	if cachePath != "" {
		if err := saveLibraryCache(cachePath, songs); err != nil {
			fmt.Fprintf(os.Stderr, "Attenzione: impossibile salvare la cache: %v\n", err)
		}
	}
	fmt.Printf("Libreria: %s indicizzati\n", brani(len(songs)))
	return songs, nil
}

// dedupeRows elimina le righe identiche (dopo normalizzazione) e restituisce
// la mappa riga originale -> riga unica.
func dedupeRows(rows []Row) ([]Row, []int) {
	unique := make([]Row, 0, len(rows))
	mapping := make([]int, len(rows))
	index := map[string]int{}
	for i, r := range rows {
		key := rowKey(r)
		if j, ok := index[key]; ok {
			mapping[i] = j
			continue
		}
		index[key] = len(unique)
		mapping[i] = len(unique)
		unique = append(unique, r)
	}
	return unique, mapping
}

func songIncluded(status string) bool {
	switch status {
	case "ESATTO", "AUTOMATICO", "SCELTO DA TE", "AMBIGUO (prima scelta)":
		return true
	}
	return false
}

func resolveAll(ui *UI, songs []Song, results []MatchResult, nonInteractive bool) ([]Entry, error) {
	entries := make([]Entry, 0, len(results))
	seenKey := map[string]int{}
	solved := map[string]int{}

	for i, r := range results {
		header := fmt.Sprintf("[%d/%d] «%s» — %s [ %s ]", i+1, len(results), r.Row.Track, r.Row.Artist, r.Row.Album)
		e := Entry{Row: r.Row}

		// Riga identica (dopo normalizzazione) già risolta: riuso la scelta
		// senza chiedere di nuovo all'utente.
		if idx, ok := solved[rowKey(r.Row)]; ok {
			e = entries[idx]
			e.Row = r.Row
			if e.Song != nil && songIncluded(e.Status) {
				e.Status = "DUPLICATO"
			}
			if e.Song != nil {
				fmt.Printf("%s %s\n", shortLabel(e.Status), formatSong(e.Song))
			} else {
				fmt.Printf("%s «%s» — %s [ %s ]\n", shortLabel(e.Status), r.Row.Track, r.Row.Artist, r.Row.Album)
			}
			solved[rowKey(r.Row)] = len(entries)
			entries = append(entries, e)
			continue
		}

		var skip bool
		switch r.Class {
		case "exact":
			if r.Pick >= 0 {
				e.Song = &songs[r.Pick]
				e.Status = "ESATTO"
			} else {
				e.Status = "NON TROVATO"
			}

		case "auto":
			if r.Pick >= 0 {
				e.Song = &songs[r.Pick]
				e.Status = "AUTOMATICO"
			} else {
				e.Status = "NON TROVATO"
			}

		case "ambiguous":
			if nonInteractive {
				if r.Pick >= 0 {
					e.Song = &songs[r.Pick]
					e.Status = "AMBIGUO (prima scelta)"
				} else {
					e.Status = "AMBIGUO (saltato)"
				}
			} else {
				opts := optionsFromCandidates(songs, r.Cands)
				if r.Pick >= 0 && !optionsContain(opts, songs[r.Pick].ID) {
					opts = append([]Option{{Song: songs[r.Pick]}}, opts...)
				}
				s, sk, err := ui.AskSong(header+"  → da decidere", opts)
				if err != nil {
					return nil, err
				}
				skip = sk
				if skip {
					e.Status = "SALTATO (tua scelta)"
				} else {
					e.Song = s
					e.Status = "SCELTO DA TE"
				}
			}

		default:
			if nonInteractive {
				e.Status = "NON TROVATO"
			} else {
				s, sk, err := ui.AskSong(header+"  → non trovato", nil)
				if err != nil {
					return nil, err
				}
				skip = sk
				if skip {
					e.Status = "SALTATO (tua scelta)"
				} else {
					e.Song = s
					e.Status = "SCELTO DA TE"
				}
			}
		}

		if !skip && e.Song != nil {
			key := normalize(e.Song.Title) + "\x00" + normalize(e.Song.Artist)
			if prevIdx, ok := seenKey[key]; ok {
				prev := &entries[prevIdx]
				switch {
				case prev.Song == nil:
					// niente da confrontare
				case prev.Song.ID == e.Song.ID:
					e.Status = "DUPLICATO"
				case nonInteractive:
					e.Status = "SCARTATO (doppione)"
				default:
					choice, err := ui.AskDuplicate(*prev, e)
					if err != nil {
						return nil, err
					}
					switch choice {
					case KeepFirst:
						e.Status = "SCARTATO (doppione)"
					case KeepSecond:
						prev.Status = "SCARTATO (doppione)"
						seenKey[key] = len(entries)
					case KeepBoth:
						// entrambi restano
					case SkipBoth:
						prev.Status = "SCARTATO (doppione)"
						e.Status = "SALTATO (tua scelta)"
						delete(seenKey, key)
					}
				}
			} else {
				seenKey[key] = len(entries)
			}
		}

		if e.Song != nil {
			fmt.Printf("%s %s\n", shortLabel(e.Status), formatSong(e.Song))
		} else {
			fmt.Printf("%s «%s» — %s [ %s ]\n", shortLabel(e.Status), r.Row.Track, r.Row.Artist, r.Row.Album)
		}

		solved[rowKey(r.Row)] = len(entries)
		entries = append(entries, e)
	}
	return entries, nil
}

func rowKey(r Row) string {
	return normalize(r.Track) + "\x00" + normalize(r.Artist) + "\x00" + normalize(r.Album)
}

func shortLabel(status string) string {
	switch status {
	case "ESATTO":
		return "✓ ESATTO      "
	case "AUTOMATICO":
		return "≈ AUTOMATICO  "
	case "SCELTO DA TE":
		return "★ SCELTO      "
	case "AMBIGUO (prima scelta)":
		return "≈ AMBIGUO     "
	case "AMBIGUO (saltato)":
		return "? AMBIGUO     "
	case "NON TROVATO":
		return "✗ NON TROVATO "
	case "SALTATO (tua scelta)", "SALTATO":
		return "- SALTATO     "
	case "DUPLICATO":
		return "= DUPLICATO   "
	case "SCARTATO (doppione)":
		return "- DOPPIONE    "
	default:
		return "  " + status
	}
}

func optionsContain(opts []Option, id string) bool {
	for _, o := range opts {
		if o.Song.ID == id {
			return true
		}
	}
	return false
}

func handlePlaylist(client *Client, ui *UI, name string, songIDs []string, dryRun, nonInteractive bool) error {
	if len(songIDs) == 0 {
		fmt.Println("\nNessun brano da inserire: la playlist non verrà creata.")
		return nil
	}

	playlists, err := client.GetPlaylists()
	if err != nil {
		return fmt.Errorf("lettura playlist: %w", err)
	}

	existing := findPlaylist(playlists, name)
	targetName := name
	existingID := ""

	if existing != nil {
		if dryRun {
			fmt.Printf("\nDRY-RUN: aggiornerei la playlist esistente «%s» (%s) con %s.\n",
				existing.Name, brani(existing.SongCount), brani(len(songIDs)))
			return nil
		}
		if nonInteractive {
			existingID = existing.ID
			targetName = existing.Name
		} else {
			for {
				action, err := ui.AskExistingPlaylist(existing.Name, existing.SongCount)
				if err != nil {
					return err
				}
				switch action.Kind {
				case "quit":
					return errQuit
				case "update":
					existingID = existing.ID
					targetName = existing.Name
				case "duplicate":
					targetName = freePlaylistName(playlists, name)
				case "rename":
					if findPlaylist(playlists, action.NewName) != nil {
						fmt.Printf("Esiste già una playlist «%s»: scegli un altro nome.\n", action.NewName)
						continue
					}
					targetName = action.NewName
				}
				break
			}
		}
	}

	if dryRun {
		fmt.Printf("\nDRY-RUN: creerei la playlist «%s» con %s.\n", targetName, brani(len(songIDs)))
		return nil
	}

	var pl *PlaylistRef
	if existingID != "" {
		pl, err = client.UpdatePlaylist(existingID, targetName, songIDs)
	} else {
		if findPlaylist(playlists, targetName) != nil {
			targetName = freePlaylistName(playlists, targetName)
		}
		pl, err = client.CreatePlaylist(targetName, songIDs)
	}
	if err != nil {
		return fmt.Errorf("creazione playlist: %w", err)
	}

	id := ""
	if pl != nil {
		id = pl.ID
	}
	verb := "creata"
	if existingID != "" {
		verb = "aggiornata"
	}
	fmt.Printf("\nPlaylist %s: «%s» — %s", verb, targetName, brani(len(songIDs)))
	if id != "" {
		fmt.Printf(" (ID %s)", id)
	}
	fmt.Println()
	return nil
}

func findPlaylist(list []PlaylistRef, name string) *PlaylistRef {
	for i := range list {
		if strings.EqualFold(strings.TrimSpace(list[i].Name), strings.TrimSpace(name)) {
			return &list[i]
		}
	}
	return nil
}

func freePlaylistName(list []PlaylistRef, base string) string {
	for n := 2; n < 1000; n++ {
		candidate := fmt.Sprintf("%s (%d)", base, n)
		if findPlaylist(list, candidate) == nil {
			return candidate
		}
	}
	return base + " (nuova)"
}

func printSummary(entries []Entry, songCount int, allPath, missPath string, dryRun bool) {
	order := []string{
		"ESATTO", "AUTOMATICO", "AMBIGUO (prima scelta)", "SCELTO DA TE",
		"NON TROVATO", "AMBIGUO (saltato)", "SALTATO (tua scelta)",
		"DUPLICATO", "SCARTATO (doppione)",
	}
	stats := map[string]int{}
	for _, e := range entries {
		stats[e.Status]++
	}

	fmt.Println("\nRiepilogo")
	fmt.Println("---------")
	for _, k := range order {
		if stats[k] > 0 {
			fmt.Printf("  %-26s %d\n", k, stats[k])
		}
	}
	fmt.Printf("\nBrani nella playlist: %d\n", songCount)
	if dryRun {
		fmt.Println("Modalità dry-run: nessuna modifica effettuata su Navidrome.")
	}
	fmt.Printf("Report:\n  %s\n  %s\n", allPath, missPath)
}
