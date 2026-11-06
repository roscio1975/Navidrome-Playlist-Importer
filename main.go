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

// version can be overridden at build time with -ldflags "-X main.version=...".
var version = "1.0.0"

// tracks returns "1 track" or "N tracks".
func tracks(n int) string {
	if n == 1 {
		return "1 track"
	}
	return fmt.Sprintf("%d tracks", n)
}

func main() {
	if err := run(); err != nil {
		if errors.Is(err, errQuit) {
			fmt.Println("\nOperation cancelled.")
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configFlag  = flag.String("config", "", "configuration file (default: user config directory)")
		csvFlag     = flag.String("csv", "", "CSV file to import (alternative to the positional argument)")
		nameFlag    = flag.String("name", "", "playlist name (default: CSV file name)")
		delimFlag   = flag.String("delimiter", "", "CSV delimiter (default: auto-detected)")
		noHeader    = flag.Bool("no-header", false, "the CSV has no header row")
		refresh     = flag.Bool("refresh", false, "download the library again, ignoring the cache")
		dryRun      = flag.Bool("dry-run", false, "show what would be done without changing Navidrome")
		yesFlag     = flag.Bool("yes", false, "non-interactive: use automatic choices for every ambiguous track")
		reportFlag  = flag.String("report-dir", "", "report directory (default: the CSV directory)")
		setupFlag   = flag.Bool("setup", false, "run the guided setup and save the configuration file")
		versionFlag = flag.Bool("version", false, "print the version")
		noColorFlag = flag.Bool("no-color", false, "disable colors in the terminal output")
	)
	flag.Usage = usage
	flag.Parse()

	useColor = colorEnabled(*noColorFlag)

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
			return errors.New("the guided setup requires an interactive terminal")
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
		return errors.New("please specify the CSV file to import")
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
		return errors.New("non-interactive input: use --yes to run without prompts")
	}

	fmt.Println()
	field("Navidrome", client.Base)
	field("User", client.User)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("cannot connect to Navidrome: %w", err)
	}

	songs, err := getLibrary(client, *refresh)
	if err != nil {
		return err
	}

	rows, delim, err := readCSV(csvPath, *delimFlag, *noHeader)
	if err != nil {
		return err
	}
	field("CSV", fmt.Sprintf("%s · delimiter %q", plural(len(rows), "row", "rows"), string(delim)))

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
	field("Tracks", fmt.Sprintf("%s unique · %d exact · %d automatic · %d to decide · %d not found",
		num(len(unique)), counts["exact"], counts["auto"], counts["ambiguous"], counts["none"]))
	fmt.Println()

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
	fmt.Fprintln(os.Stderr, "Import a CSV playlist into Navidrome with fuzzy track matching.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  navidrome-importer [options] <file.csv>")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, `  navidrome-importer "Italian Beat.csv"`)
	fmt.Fprintln(os.Stderr, `  navidrome-importer --dry-run "Italian Beat.csv"`)
	fmt.Fprintln(os.Stderr, `  navidrome-importer --name "Metal" --refresh "Metal.csv"`)
}

func loadOrSetupConfig(path string) (*Config, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading configuration: %w", err)
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, fmt.Errorf("configuration file not found: %s\nRun `navidrome-importer --setup` from a terminal to create it", path)
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
			return nil, fmt.Errorf("incomplete configuration: %s\nurl, user and password are required", path)
		}
		return setupConfig(path)
	}
	return cfg, nil
}

func getLibrary(client *Client, refresh bool) ([]Song, error) {
	cachePath, _ := libraryCachePath(client.Base, client.User)

	if !refresh && cachePath != "" {
		if songs, fetchedAt, ok := loadLibraryCache(cachePath); ok {
			field("Library", fmt.Sprintf("%s · cached on %s · use --refresh to update",
				tracks(len(songs)), fetchedAt.Format("2006-01-02 15:04")))
			return songs, nil
		}
	}

	field("Library", "downloading the full list from Navidrome (this may take a few minutes)...")
	songs, err := fetchLibrary(client)
	if err != nil {
		return nil, err
	}
	if cachePath != "" {
		if err := saveLibraryCache(cachePath, songs); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot save the cache: %v\n", err)
		}
	}
	field("Library", tracks(len(songs))+" indexed")
	return songs, nil
}

// dedupeRows removes identical rows (after normalization) and returns the
// original row -> unique row mapping.
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
	case StatusExact, StatusAutomatic, StatusManual, StatusAmbiguousFirst:
		return true
	}
	return false
}

func resolveAll(ui *UI, songs []Song, results []MatchResult, nonInteractive bool) ([]Entry, error) {
	entries := make([]Entry, 0, len(results))
	seenKey := map[string]int{}
	solved := map[string]int{}

	for i, r := range results {
		e := Entry{RowNum: i + 1, Row: r.Row}

		// Identical row (after normalization) already resolved: reuse the choice
		// without asking the user again.
		if idx, ok := solved[rowKey(r.Row)]; ok {
			e = entries[idx]
			e.RowNum = i + 1
			e.Row = r.Row
			if e.Song != nil && songIncluded(e.Status) {
				e.Status = StatusDuplicate
			}
			trackLine(i+1, len(results), e)
			solved[rowKey(r.Row)] = len(entries)
			entries = append(entries, e)
			continue
		}

		var skip bool
		switch r.Class {
		case "exact":
			if r.Pick >= 0 {
				e.Song = &songs[r.Pick]
				e.Status = StatusExact
			} else {
				e.Status = StatusNotFound
			}

		case "auto":
			if r.Pick >= 0 {
				e.Song = &songs[r.Pick]
				e.Status = StatusAutomatic
			} else {
				e.Status = StatusNotFound
			}

		case "ambiguous":
			if nonInteractive {
				if r.Pick >= 0 {
					e.Song = &songs[r.Pick]
					e.Status = StatusAmbiguousFirst
				} else {
					e.Status = StatusAmbiguousSkip
				}
			} else {
				opts := optionsFromCandidates(songs, r.Cands)
				if r.Pick >= 0 && !optionsContain(opts, songs[r.Pick].ID) {
					opts = append([]Option{{Song: songs[r.Pick]}}, opts...)
				}
				s, sk, err := ui.AskSong(r.Row, i+1, len(results), true, opts)
				if err != nil {
					return nil, err
				}
				skip = sk
				if skip {
					e.Status = StatusSkipped
				} else {
					e.Song = s
					e.Status = StatusManual
				}
			}

		default:
			if nonInteractive {
				e.Status = StatusNotFound
			} else {
				s, sk, err := ui.AskSong(r.Row, i+1, len(results), false, nil)
				if err != nil {
					return nil, err
				}
				skip = sk
				if skip {
					e.Status = StatusSkipped
				} else {
					e.Song = s
					e.Status = StatusManual
				}
			}
		}

		if !skip && e.Song != nil {
			key := normalize(e.Song.Title) + "\x00" + normalize(e.Song.Artist)
			if prevIdx, ok := seenKey[key]; ok {
				prev := &entries[prevIdx]
				switch {
				case prev.Song == nil:
					// nothing to compare
				case prev.Song.ID == e.Song.ID:
					e.Status = StatusDuplicate
				case nonInteractive:
					e.Status = StatusDiscarded
				default:
					choice, err := ui.AskDuplicate(*prev, e)
					if err != nil {
						return nil, err
					}
					switch choice {
					case KeepFirst:
						e.Status = StatusDiscarded
					case KeepSecond:
						prev.Status = StatusDiscarded
						seenKey[key] = len(entries)
					case KeepBoth:
						// both are kept
					case SkipBoth:
						prev.Status = StatusDiscarded
						e.Status = StatusSkipped
						delete(seenKey, key)
					}
				}
			} else {
				seenKey[key] = len(entries)
			}
		}

		trackLine(i+1, len(results), e)

		solved[rowKey(r.Row)] = len(entries)
		entries = append(entries, e)
	}
	return entries, nil
}

func rowKey(r Row) string {
	return normalize(r.Track) + "\x00" + normalize(r.Artist) + "\x00" + normalize(r.Album)
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
		fmt.Printf("\n%s\n", yellow("No tracks to add: the playlist will not be created."))
		return nil
	}

	playlists, err := client.GetPlaylists()
	if err != nil {
		return fmt.Errorf("reading playlists: %w", err)
	}

	existing := findPlaylist(playlists, name)
	targetName := name
	existingID := ""

	if existing != nil {
		if dryRun {
			fmt.Printf("\n%s · would update the existing playlist %s (%s) with %s.\n",
				yellow("dry-run"), bold("«"+existing.Name+"»"), tracks(existing.SongCount), tracks(len(songIDs)))
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
						fmt.Printf("  %s\n", red(fmt.Sprintf("A playlist named «%s» already exists: choose another name.", action.NewName)))
						continue
					}
					targetName = action.NewName
				}
				break
			}
		}
	}

	if dryRun {
		fmt.Printf("\n%s · would create the playlist %s with %s.\n",
			yellow("dry-run"), bold("«"+targetName+"»"), tracks(len(songIDs)))
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
		return fmt.Errorf("saving playlist: %w", err)
	}

	id := ""
	if pl != nil {
		id = pl.ID
	}
	verb := "created"
	if existingID != "" {
		verb = "updated"
	}
	line := fmt.Sprintf("Playlist %s · %s · %s", verb, bold("«"+targetName+"»"), tracks(len(songIDs)))
	if id != "" {
		line += dim(" (ID " + id + ")")
	}
	fmt.Printf("\n  %s %s\n", green("✓"), line)
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
	return base + " (new)"
}

func printSummary(entries []Entry, songCount int, allPath, missPath string, dryRun bool) {
	order := []string{
		StatusExact, StatusAutomatic, StatusAmbiguousFirst, StatusManual,
		StatusNotFound, StatusAmbiguousSkip, StatusSkipped,
		StatusDuplicate, StatusDiscarded,
	}
	stats := map[string]int{}
	for _, e := range entries {
		stats[e.Status]++
	}

	fmt.Printf("\n%s\n", bold("Summary"))
	fmt.Println("  " + dim(strings.Repeat("─", 24)))
	for _, k := range order {
		if stats[k] > 0 {
			fmt.Printf("  %s %5s\n", statusTag(k), num(stats[k]))
		}
	}

	fmt.Printf("\n  %s  %s\n", dim(fmt.Sprintf("%-11s", "In playlist")), bold(tracks(songCount)))
	if dryRun {
		fmt.Printf("  %s  %s\n", dim(fmt.Sprintf("%-11s", "Mode")), yellow("dry-run · nothing was changed in Navidrome"))
	}
	fmt.Printf("\n  %s\n", dim("Reports"))
	fmt.Printf("    %s\n    %s\n", allPath, missPath)
}
