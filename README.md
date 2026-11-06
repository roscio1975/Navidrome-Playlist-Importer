# navidrome-importer

Import a CSV playlist into **Navidrome**, matching tracks against your library automatically.

The program downloads the full library through Navidrome's Subsonic API, compares every CSV row
with the tracks in your collection (tolerating accents, punctuation, letter case, live/remaster
versions, feat., different albums, and so on) and only asks for a decision when a case is truly
ambiguous.

- **Interface**: terminal-based, with numbered menus.
- **No runtime dependencies**: a single executable for macOS, Linux and Windows.
- **Reports**: two CSV files (matches and tracks not found) ready for Excel.
- **Configuration**: a simple `key = value` text file.

---

## Installation

### Prebuilt binary

Download the executable for your system from the repository's **Releases** page and put it in a
directory on your `PATH` (or call it with its full path).

| System | File |
|---|---|
| macOS Apple Silicon | `navidrome-importer-darwin-arm64` |
| macOS Intel | `navidrome-importer-darwin-amd64` |
| Linux x86_64 | `navidrome-importer-linux-amd64` |
| Linux ARM64 | `navidrome-importer-linux-arm64` |
| Windows x86_64 | `navidrome-importer-windows-amd64.exe` |

On macOS you may need to allow **local network** access on first launch (see
[Troubleshooting](#no-route-to-host-on-macos)).

### Building from source

Go 1.23 or newer is required.

```sh
git clone https://github.com/roscio1975/Navidrome-Playlist-Importer.git
cd Navidrome-Playlist-Importer
make build          # creates ./navidrome-importer
make test           # runs the tests
make dist           # cross-compiles every platform into dist/ + SHA256SUMS
```

---

## First run

```sh
./navidrome-importer "Italian Beat.csv"
```

On first launch the program asks for:

```
Server URL (e.g. http://192.168.1.10:4533):
Username:
Password:
```

and saves the values in a plain-text file with `600` permissions:

| System | Path |
|---|---|
| macOS | `~/Library/Application Support/navidrome-import/config.conf` |
| Linux | `~/.config/navidrome-import/config.conf` |
| Windows | `%AppData%\navidrome-import\config.conf` |

The file can be edited with any text editor:

```ini
# navidrome-import configuration
url = http://192.168.1.10:4533
user = mario
password = my-password
```

To run the setup again: `./navidrome-importer --setup`.
To use a different file: `./navidrome-importer --config /path/to/config.conf "playlist.csv"`.

Credentials can also be provided through the environment variables `NAVIDROME_URL`,
`NAVIDROME_USER` and `NAVIDROME_PASSWORD` (they take precedence over the file).
The configuration file path can be set with `NAVIDROME_IMPORT_CONFIG`.

> The URL can be written with or without `http://`. If Navidrome is reachable over HTTPS, use
> `https://...`.

---

## CSV format

The file must contain at least track titles and artists; the album is recommended but optional.

```csv
Track name;Artist name;Album
Un'emozione da poco;Anna Oxa;Best of Anna Oxa
Sei bellissima;Loredana Bertè;Sei bellissima
Piazza Grande;Lucio Dalla;Il meglio della musica italiana anni '70
```

- The delimiter (`;`, `,`, TAB, `|`) is detected automatically.
- Recognized headers include `Track name`, `Title`, `Song`, `Brano`, `Artist name`, `Artist`,
  `Artista`, `Album`.
- If the file has no header, use `--no-header`.
- UTF-8 encoding is recommended (with or without BOM).
- Identical rows are imported only once.

---

## How matching works

Every row goes through three levels:

1. **Exact** — title and artist match (ignoring accents, case and punctuation). The match is
   applied automatically.
2. **Automatic** — the title is very close and the artist is compatible, with coherent album or
   tags (live, remaster, feat., and so on). The match is applied automatically.
3. **Needs a decision** — there are several plausible candidates, or none. The program shows the
   options and you choose.

Ambiguous cases are displayed like this:

```
[1/100] «Piazza Grande» — Lucio Dalla [ Il meglio della musica italiana anni '70 ]  → needs a decision
  1) Piazza grande — Dalla - De Gregori [ Banana Republic ]
  2) Piazza Grande): Piazza Grande — Lucio Dalla [ Gli anni settanta - CD1 ]  (0.84)
  s = skip   c <text> = search   i <id> = enter ID   q = quit
Choice [1]:
```

What you can type:

| Command | Meaning |
|---|---|
| `1`, `2`, ... | choose the corresponding candidate |
| Enter | choose the first candidate (or skip, if there are none) |
| `s` | skip this track |
| `c <text>` or `search <text>` | search for another text in the Navidrome library |
| `i <id>` | enter the Subsonic ID of a track directly |
| `q` | stop without changing anything |

### Duplicates

If the same title + artist appears more than once in the CSV on **different albums**, the program
asks every time which version to keep:

```
Duplicate: «La Primavera» — Manu Chao
  1) La Primavera — Manu Chao [ Baionarena ]
     already in the playlist (CSV row: «La Primavera» — Manu Chao [ Baionarena ])
  2) La Primavera — Manu CHAO [ Próxima Estación... Esperanza ]
     new (CSV row: «La Primavera» — Manu Chao [ Próxima Estación... Esperanza ])
  Choice: 1 = keep 1, 2 = keep 2, t = keep both, s = skip both, q = quit
```

### Existing playlist

If a playlist with the same name already exists:

```
a = update (replaces the tracks)
d = duplicate (creates a new playlist «name (2)»)
r = rename the new playlist
q = quit without changes
```

The default playlist name is the CSV file name without its extension; it can be changed with
`--name`.

---

## Options

```
navidrome-importer [options] <file.csv>
```

| Option | Description |
|---|---|
| `--name <name>` | playlist name (default: CSV file name) |
| `--config <file>` | alternative configuration file |
| `--setup` | run the guided setup and exit |
| `--csv <file>` | CSV file (alternative to the positional argument) |
| `--delimiter <char>` | force the CSV delimiter (e.g. `--delimiter ";"` or `--delimiter "\t"`) |
| `--no-header` | the CSV has no header row |
| `--refresh` | download the library again, ignoring the cache |
| `--dry-run` | show what would be done without creating/updating the playlist |
| `--yes` | non-interactive: use automatic choices (see below) |
| `--report-dir <dir>` | report directory (default: the CSV directory) |
| `--version` | print the version |
| `-h`, `--help` | show the help |

Examples:

```sh
./navidrome-importer "Metal.csv"
./navidrome-importer --dry-run "Allegrezza.csv"
./navidrome-importer --name "Italian Beat" --refresh "Italian Beat.csv"
./navidrome-importer --report-dir ~/Desktop "Estate.csv"
```

### `--yes` mode

For scripts and automation. It asks no questions and applies these rules:

- ambiguous cases → the candidate with the highest score is chosen;
- duplicates → the first version encountered is kept;
- existing playlist → it is **updated** (previous tracks are replaced).

All choices are still recorded in the reports. `--yes` does not require an interactive terminal.

---

## Reports

Next to the CSV (or in the directory given with `--report-dir`) the program creates:

- `Playlist <Name> - matches.csv` — one row for every CSV row, with the status and the matching
  Navidrome track;
- `Playlist <Name> - not found.csv` — only tracks that were not matched or were skipped.

The files use `;` as separator, a UTF-8 BOM and CRLF line endings, so Excel opens them correctly.

With `--dry-run` the names become `Playlist <Name> - preview - ...` to avoid overwriting the
reports of a real import.

### Statuses

| Status | Meaning |
|---|---|
| `EXACT` | title and artist match |
| `AUTOMATIC` | automatic match with high confidence |
| `AMBIGUOUS (first choice)` | automatic choice of the first candidate (only with `--yes`) |
| `MANUAL CHOICE` | manual choice (candidate, search or ID) |
| `NOT FOUND` | no candidate |
| `SKIPPED (your choice)` | skipped during review |
| `DUPLICATE` | the same track is already in the playlist |
| `DISCARDED (duplicate)` | version discarded as a duplicate |

---

## Library cache

Downloading the whole library can take a few minutes the first time. The data is cached for
**12 hours**:

| System | Path |
|---|---|
| macOS | `~/Library/Caches/navidrome-import/` |
| Linux | `~/.cache/navidrome-import/` |
| Windows | `%LocalAppData%\navidrome-import\` |

The cache is tied to URL + user. Use `--refresh` to force a new download.

---

## Troubleshooting

### `no route to host` on macOS

Since macOS 15, the system asks for permission to access the **local network**. If the binary is
not authorized, connections to the server fail with `no route to host` while `curl` keeps working.

Open **System Settings → Privacy & Security → Local Network** and enable the terminal you launch
the program from (Terminal, iTerm, etc.). If the binary does not appear there, start it from the
terminal and accept the authorization prompt when it shows up.

### `401` / `Wrong username or password`

Check `user` and `password` in the configuration file. The password is the Navidrome account
password (not an API token).

### The library looks stale

Run with `--refresh` or delete the cache file.

### A track exists but is not found

At the prompt type `c <part of the title>` and choose the result, or `i <ID>` using the track's
Subsonic ID (visible in the Navidrome web interface on the track page).

---

## Technical notes

- Subsonic token authentication: `t = md5(password + salt)`. The password is never sent in
  plaintext, but if the server is reached over HTTP the traffic is not encrypted: only use trusted
  networks or configure HTTPS.
- Candidate search uses an inverted index over title and artist tokens, Python's `SequenceMatcher`
  similarity (reimplemented) and a set of normalizations (accents, brackets, `feat.`, `pt.` →
  `part`, album/artist prefixes, live/reprise/dub/remix/... tags).
- Very long requests (playlists with many tracks) are sent with POST.
- The configuration file is created with `600` permissions.

---

## License

GNU General Public License v3.0 — see [LICENSE](LICENSE).
