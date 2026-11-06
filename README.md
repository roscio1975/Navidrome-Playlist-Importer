# navidrome-importer

Importa una playlist in formato CSV dentro **Navidrome**, abbinando automaticamente i brani
della tua libreria.

Il programma scarica l'elenco completo della libreria tramite l'API Subsonic di Navidrome,
confronta ogni riga del CSV con i brani presenti (anche con differenze di accenti, punteggiatura,
maiuscole, versioni live/remaster, feat., album diversi...) e ti chiede una decisione solo per i
casi davvero dubbi.

- **Interfaccia**: da terminale, in italiano, con menu numerati.
- **Nessuna dipendenza esterna**: un singolo eseguibile per macOS, Linux e Windows.
- **Report**: due file CSV (abbinamenti e brani non trovati) pronti per Excel.
- **Configurazione**: semplice file di testo `chiave = valore`.

---

## Installazione

### Binario pronto

Scarica l'eseguibile per il tuo sistema dalla pagina **Releases** del repository e mettilo in una
cartella del `PATH` (oppure richiamalo con il percorso completo).

| Sistema | File |
|---|---|
| macOS Apple Silicon | `navidrome-importer-darwin-arm64` |
| macOS Intel | `navidrome-importer-darwin-amd64` |
| Linux x86_64 | `navidrome-importer-linux-amd64` |
| Linux ARM64 | `navidrome-importer-linux-arm64` |
| Windows x86_64 | `navidrome-importer-windows-amd64.exe` |

Su macOS, al primo avvio potresti dover autorizzare l'accesso alla **rete locale** (vedi
[Risoluzione problemi](#macos-no-route-to-host)).

### Compilazione dai sorgenti

Serve Go 1.23 o successivo.

```sh
git clone <repo>
cd navidrome-importer
make build          # crea ./navidrome-importer
make test           # esegue i test
make dist           # crea i binari per tutte le piattaforme in dist/
```

---

## Primo avvio

```sh
./navidrome-importer "Italian Beat.csv"
```

Al primo avvio il programma chiede:

```
URL del server (es. http://192.168.1.10:4533):
Username:
Password:
```

e salva i dati in un file di testo con permessi `600`:

| Sistema | Percorso |
|---|---|
| macOS | `~/Library/Application Support/navidrome-import/config.conf` |
| Linux | `~/.config/navidrome-import/config.conf` |
| Windows | `%AppData%\navidrome-import\config.conf` |

Il file è modificabile con un normale editor di testo:

```ini
# Configurazione navidrome-importer
url = http://192.168.1.10:4533
user = mario
password = la-mia-password
```

Per rifare la configurazione: `./navidrome-importer --setup`.
Per usare un altro file: `./navidrome-importer --config /percorso/config.conf "playlist.csv"`.

Le credenziali possono anche essere passate con le variabili d'ambiente
`NAVIDROME_URL`, `NAVIDROME_USER`, `NAVIDROME_PASSWORD` (hanno la precedenza sul file).
Il percorso del file di configurazione può essere impostato con `NAVIDROME_IMPORT_CONFIG`.

> L'URL può essere scritto con o senza `http://`. Se Navidrome è raggiungibile in HTTPS, usa
> `https://...`.

---

## Formato del CSV

Il file deve contenere almeno i titoli dei brani e gli artisti; l'album è consigliato ma
facoltativo.

```csv
Track name;Artist name;Album
Un'emozione da poco;Anna Oxa;Best of Anna Oxa
Sei bellissima;Loredana Bertè;Sei bellissima
Piazza Grande;Lucio Dalla;Il meglio della musica italiana anni '70
```

- Il separatore (`;`, `,`, TAB, `|`) viene rilevato automaticamente.
- Le intestazioni riconosciute includono `Track name`, `Title`, `Brano`, `Artist name`,
  `Artist`, `Artista`, `Album`.
- Se il file non ha intestazione, usa `--no-header`.
- È consigliata la codifica UTF-8 (con o senza BOM).
- Le righe identiche vengono considerate una sola volta.

---

## Come funziona l'abbinamento

Ogni riga passa attraverso tre livelli:

1. **Esatto** — titolo e artista coincidono (ignorando accenti, maiuscole e punteggiatura).
   L'abbinamento è automatico.
2. **Automatico** — titolo molto simile e artista compatibile, con album o tag coerenti
   (live, remaster, feat., ecc.). L'abbinamento è automatico.
3. **Da decidere** — ci sono più candidati plausibili, oppure nessuno. Il programma ti mostra
   le opzioni e scegli tu.

I casi dubbi vengono mostrati così:

```
[1/100] «Piazza Grande» — Lucio Dalla [ Il meglio della musica italiana anni '70 ]  → da decidere
  1) Piazza grande — Dalla - De Gregori [ Banana Republic ]
  2) Piazza Grande): Piazza Grande — Lucio Dalla [ Gli anni settanta - CD1 ]  (0.84)
  s = salta   c <testo> = cerca   i <id> = inserisci ID   q = esci
Scelta [1]:
```

Cosa puoi digitare:

| Comando | Significato |
|---|---|
| `1`, `2`, ... | scegli il candidato corrispondente |
| Invio | scegli il primo candidato (o salta, se non ce ne sono) |
| `s` | salta questo brano |
| `c <testo>` | cerca un altro testo nella libreria Navidrome |
| `i <id>` | inserisci direttamente l'ID Subsonic di un brano |
| `q` | interrompi senza modificare nulla |

### Doppioni

Se lo stesso titolo + artista compare più volte nel CSV su **album diversi**, il programma chiede
ogni volta quale versione tenere:

```
Doppione: «La Primavera» — Manu Chao
  1) La Primavera — Manu Chao [ Baionarena ]
  2) La Primavera — Manu CHAO [ Próxima Estación... Esperanza ]
  Scelta: 1 = tieni 1, 2 = tieni 2, t = tieni entrambi, s = salta entrambi, q = esci
```

### Playlist già esistente

Se esiste già una playlist con lo stesso nome:

```
a = aggiorna (sostituisce i brani)
d = duplica (crea una nuova playlist «nome (2)»)
r = rinomina la nuova playlist
q = esci senza modificare
```

Il nome predefinito della playlist è il nome del file CSV senza estensione; si può cambiare con
`--name`.

---

## Opzioni

```
navidrome-importer [opzioni] <file.csv>
```

| Opzione | Descrizione |
|---|---|
| `--name <nome>` | nome della playlist (default: nome del file CSV) |
| `--config <file>` | file di configurazione alternativo |
| `--setup` | esegue la configurazione guidata |
| `--delimiter <car>` | forza il separatore CSV (es. `--delimiter ";"`) |
| `--no-header` | il CSV non ha riga di intestazione |
| `--refresh` | riscarica la libreria ignorando la cache |
| `--dry-run` | mostra cosa verrebbe fatto, senza creare/modificare la playlist |
| `--yes` | non interattivo: usa le scelte automatiche (vedi sotto) |
| `--report-dir <dir>` | cartella dei report (default: quella del CSV) |
| `--version` | mostra la versione |
| `-h`, `--help` | mostra l'aiuto |

Esempi:

```sh
./navidrome-importer "Metal.csv"
./navidrome-importer --dry-run "Allegrezza.csv"
./navidrome-importer --name "Italian Beat" --refresh "Italian Beat.csv"
./navidrome-importer --report-dir ~/Desktop "Estate.csv"
```

### Modalità `--yes`

Serve per script e automazioni. Non fa domande e applica queste regole:

- casi dubbi → viene scelto il candidato con il punteggio più alto;
- doppioni → viene tenuta la prima versione incontrata;
- playlist già esistente → viene **aggiornata** (i brani precedenti vengono sostituiti).

Tutte le scelte vengono comunque riportate nei report. Con `--yes` non serve un terminale
interattivo.

---

## Report

Accanto al CSV (o nella cartella indicata con `--report-dir`) vengono creati:

- `Playlist <Nome> - abbinamenti.csv` — una riga per ogni riga del CSV, con lo stato e i dati del
  brano trovato in Navidrome;
- `Playlist <Nome> - non trovati.csv` — solo i brani non abbinati o saltati.

I file usano separatore `;`, BOM UTF-8 e fine riga CRLF, così Excel li apre correttamente.

Con `--dry-run` i nomi diventano `Playlist <Nome> - anteprima - ...` per non sovrascrivere i
report di un'importazione reale.

### Stati

| Stato | Significato |
|---|---|
| `ESATTO` | titolo e artista coincidono |
| `AUTOMATICO` | abbinamento automatico con alta confidenza |
| `AMBIGUO (prima scelta)` | scelta automatica del primo candidato (solo `--yes`) |
| `SCELTO DA TE` | scelta manuale (candidato, ricerca o ID) |
| `NON TROVATO` | nessun candidato |
| `SALTATO (tua scelta)` | saltato durante la revisione |
| `DUPLICATO` | stesso brano già presente nella playlist |
| `SCARTATO (doppione)` | versione scartata di un doppione |

---

## Cache della libreria

Scaricare l'intera libreria può richiedere qualche minuto la prima volta. I dati vengono salvati
in cache per **12 ore**:

| Sistema | Percorso |
|---|---|
| macOS | `~/Library/Caches/navidrome-import/` |
| Linux | `~/.cache/navidrome-import/` |
| Windows | `%LocalAppData%\navidrome-import\` |

La cache è legata a URL + utente. Usa `--refresh` per forzare il download.

---

## Risoluzione problemi

### `no route to host` su macOS

Da macOS 15 in poi il sistema chiede il permesso di accedere alla **rete locale**. Se il binario
non è autorizzato, le connessioni verso il server falliscono con `no route to host` mentre
`curl` continua a funzionare.

Vai in **Impostazioni di Sistema → Privacy e Sicurezza → Rete locale** e abilita il terminale
da cui lanci il programma (Terminal, iTerm, ecc.). Se il binario non compare, avvialo dal
terminale e accetta il messaggio di autorizzazione quando appare.

### `401` / `Wrong username or password`

Controlla `user` e `password` nel file di configurazione. La password è quella dell'account
Navidrome (non il token API).

### La libreria sembra vecchia

Esegui con `--refresh` oppure elimina il file di cache.

### Un brano c'è ma non viene trovato

Nel prompt digita `c <parte del titolo>` e scegli il risultato, oppure `i <ID>` usando l'ID
Subsonic del brano (visibile nell'interfaccia web di Navidrome nella pagina del brano).

---

## Note tecniche

- Autenticazione Subsonic con token: `t = md5(password + salt)`. La password non viene mai
  trasmessa in chiaro, ma se il server è raggiunto in HTTP il traffico non è cifrato: in quel caso
  usa solo reti fidate o configura HTTPS.
- La ricerca dei candidati usa un indice invertito sui token di titolo e artista, la similarità
  `SequenceMatcher` di Python (reimplementata) e una serie di normalizzazioni (accenti, parentesi,
  `feat.`, `pt.` → `part`, prefissi album/artista, tag live/reprise/dub/remix/...).
- Le richieste molto lunghe (playlist con molti brani) vengono inviate in POST.
- Il file di configurazione viene creato con permessi `600`.

---

## Licenza

MIT — vedi [LICENSE](LICENSE).
