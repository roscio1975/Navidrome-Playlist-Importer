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

var errQuit = errors.New("cancelled by the user")

// Option is a choice shown to the user.
type Option struct {
	Song     Song
	Score    float64
	HasScore bool
}

// UI handles terminal interaction.
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
		fmt.Fprintf(u.out, "    %s  %s", dim(fmt.Sprintf("%2d", i+1)), formatSong(&o.Song))
		if o.HasScore {
			fmt.Fprintf(u.out, "  %s", dim(fmt.Sprintf("%.2f", o.Score)))
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

// AskSong shows the candidates for a CSV row and returns the chosen track.
// skip=true means the user chose to skip this row.
func (u *UI) AskSong(row Row, n, total int, ambiguous bool, opts []Option) (*Song, bool, error) {
	const indent = "             "

	if n > 1 {
		fmt.Fprintln(u.out)
	}
	fmt.Fprintf(u.out, "  %s  %s\n", progressTag(n, total), bold(describeRow(row)))
	if ambiguous {
		fmt.Fprintf(u.out, "%s%s %s\n", indent, yellow("needs a decision"),
			dim(fmt.Sprintf("· %d candidates", len(opts))))
	} else {
		fmt.Fprintf(u.out, "%s%s %s\n", indent, red("not found"), dim("· no candidates in the library"))
	}
	if len(opts) > 0 {
		u.printOptions(opts)
	}
	fmt.Fprintf(u.out, "%s%s\n", indent, dim("s = skip   c <text> = search   i <id> = enter ID   q = quit"))

	for {
		if len(opts) > 0 {
			fmt.Fprintf(u.out, "Choice [1]: ")
		} else {
			fmt.Fprintf(u.out, "Choice [s]: ")
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

		case low == "s" || low == "skip":
			return nil, true, nil

		case low == "q" || low == "quit":
			return nil, false, errQuit

		case low == "c" || low == "search" || strings.HasPrefix(low, "c ") || strings.HasPrefix(low, "search "):
			var query string
			switch {
			case strings.HasPrefix(low, "search "):
				query = strings.TrimSpace(line[len("search "):])
			case strings.HasPrefix(low, "c "):
				query = strings.TrimSpace(line[len("c "):])
			}
			if query == "" {
				query, err = u.readLine("Search text: ")
				if err != nil {
					return nil, false, err
				}
			}
			if query == "" {
				continue
			}
			found, err := u.client.Search(query)
			if err != nil {
				fmt.Fprintf(u.out, "    %s\n", red("Search error: "+err.Error()))
				continue
			}
			if len(found) == 0 {
				fmt.Fprintf(u.out, "    %s\n", dim("No results."))
				continue
			}
			opts = make([]Option, 0, len(found))
			for _, s := range found {
				opts = append(opts, Option{Song: s})
			}
			fmt.Fprintf(u.out, "\n    %s\n", cyan("Search results"))
			u.printOptions(opts)

		case low == "i" || strings.HasPrefix(low, "i "):
			id := strings.TrimSpace(line[1:])
			if id == "" {
				id, err = u.readLine("Track ID: ")
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
				fmt.Fprintf(u.out, "    %s\n", red("Invalid ID: "+err.Error()))
				continue
			}
			return s, false, nil

		default:
			n, err := strconv.Atoi(line)
			if err == nil && n >= 1 && n <= len(opts) {
				s := opts[n-1].Song
				return &s, false, nil
			}
			fmt.Fprintf(u.out, "    %s\n", red("Invalid choice."))
		}
	}
}

// DupChoice is the choice made for a duplicate track.
type DupChoice int

const (
	KeepFirst DupChoice = iota
	KeepSecond
	KeepBoth
	SkipBoth
)

// AskDuplicate asks which version to keep when the same title+artist appears
// on different albums.
func (u *UI) AskDuplicate(first, second Entry) (DupChoice, error) {
	fmt.Fprintln(u.out)
	fmt.Fprintf(u.out, "  %s  %s\n", yellow("duplicate"),
		bold(fmt.Sprintf("«%s» — %s", first.Song.Title, first.Song.Artist)))
	fmt.Fprintf(u.out, "    %s  %s\n", dim("1"), formatSong(first.Song))
	fmt.Fprintf(u.out, "       %s\n", dim("already in the playlist · CSV row "+describeRow(first.Row)))
	fmt.Fprintf(u.out, "    %s  %s\n", dim("2"), formatSong(second.Song))
	fmt.Fprintf(u.out, "       %s\n", dim("new · CSV row "+describeRow(second.Row)))
	fmt.Fprintf(u.out, "    %s\n", dim("1 = keep 1   2 = keep 2   t = keep both   s = skip both   q = quit"))

	for {
		line, err := u.readLine("Choice [1]: ")
		if err != nil {
			return KeepFirst, err
		}
		switch strings.ToLower(line) {
		case "", "1":
			return KeepFirst, nil
		case "2":
			return KeepSecond, nil
		case "t", "both":
			return KeepBoth, nil
		case "s", "skip":
			return SkipBoth, nil
		case "q", "quit":
			return KeepFirst, errQuit
		default:
			fmt.Fprintln(u.out, "  Invalid choice.")
		}
	}
}

// ExistingAction is the action chosen for an existing playlist.
type ExistingAction struct {
	Kind    string // update | duplicate | rename | quit
	NewName string
}

func (u *UI) AskExistingPlaylist(name string, count int) (ExistingAction, error) {
	fmt.Fprintln(u.out)
	fmt.Fprintf(u.out, "  %s  %s%s\n", yellow("playlist exists"),
		bold("«"+name+"»"), dim(" · "+tracks(count)))
	fmt.Fprintf(u.out, "    %s  %s\n", dim("a"), "update (replaces the tracks)")
	fmt.Fprintf(u.out, "    %s  %s\n", dim("d"), "duplicate (creates a new playlist «name (2)»)")
	fmt.Fprintf(u.out, "    %s  %s\n", dim("r"), "rename the new playlist")
	fmt.Fprintf(u.out, "    %s  %s\n", dim("q"), "quit without changes")

	for {
		line, err := u.readLine("Choice [a]: ")
		if err != nil {
			return ExistingAction{}, err
		}
		switch strings.ToLower(line) {
		case "", "a", "update":
			return ExistingAction{Kind: "update"}, nil
		case "d", "duplicate":
			return ExistingAction{Kind: "duplicate"}, nil
		case "r", "rename":
			newName, err := u.readLine("New playlist name: ")
			if err != nil {
				return ExistingAction{}, err
			}
			if strings.TrimSpace(newName) == "" {
				fmt.Fprintln(u.out, "  Invalid name.")
				continue
			}
			return ExistingAction{Kind: "rename", NewName: strings.TrimSpace(newName)}, nil
		case "q", "quit":
			return ExistingAction{Kind: "quit"}, nil
		default:
			fmt.Fprintln(u.out, "  Invalid choice.")
		}
	}
}
