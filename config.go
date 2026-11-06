package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// Config contiene i parametri di connessione a Navidrome.
type Config struct {
	URL      string
	User     string
	Password string
}

func defaultConfigPath() string {
	if p := os.Getenv("NAVIDROME_IMPORT_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "navidrome-import", "config.conf")
}

// loadConfig legge un file di testo con righe chiave=valore.
// Sono ignorate le righe vuote e quelle che iniziano con # o ;.
func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := &Config{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		switch k {
		case "url", "server", "indirizzo":
			c.URL = v
		case "user", "username", "utente":
			c.User = v
		case "password", "pass":
			c.Password = v
		}
	}
	return c, sc.Err()
}

func saveConfig(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Configurazione navidrome-import\n")
	b.WriteString("# Modifica questo file per cambiare server o credenziali.\n")
	b.WriteString("# Le righe che iniziano con # sono commenti.\n\n")
	fmt.Fprintf(&b, "url = %s\n", c.URL)
	fmt.Fprintf(&b, "user = %s\n", c.User)
	fmt.Fprintf(&b, "password = %s\n", c.Password)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// setupConfig chiede i dati all'utente e li salva su disco.
func setupConfig(path string) (*Config, error) {
	in := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("Configurazione Navidrome")
	fmt.Println("------------------------")
	fmt.Printf("File di configurazione: %s\n\n", path)

	url := askLine(in, "URL del server (es. http://192.168.1.10:4533)", "")
	if url == "" {
		return nil, fmt.Errorf("URL obbligatorio")
	}
	user := askLine(in, "Username", "")
	if user == "" {
		return nil, fmt.Errorf("username obbligatorio")
	}

	fmt.Print("Password: ")
	var pwd string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return nil, err
		}
		pwd = string(b)
	} else {
		line, _ := in.ReadString('\n')
		pwd = strings.TrimRight(line, "\r\n")
	}
	if pwd == "" {
		return nil, fmt.Errorf("password obbligatoria")
	}

	c := &Config{URL: url, User: user, Password: pwd}
	if err := saveConfig(path, c); err != nil {
		fmt.Fprintf(os.Stderr, "Attenzione: impossibile salvare la configurazione: %v\n", err)
	} else {
		fmt.Printf("Configurazione salvata in %s (permessi 600)\n", path)
	}
	return c, nil
}

func askLine(in *bufio.Reader, prompt, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", prompt, def)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}
