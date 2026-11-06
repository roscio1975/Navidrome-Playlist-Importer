package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// Config holds the connection parameters for Navidrome.
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

// loadConfig reads a plain-text key=value file. Empty lines and lines starting
// with # or ; are ignored.
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
	b.WriteString("# navidrome-import configuration\n")
	b.WriteString("# Edit this file to change the server or the credentials.\n")
	b.WriteString("# Lines starting with # are comments.\n\n")
	fmt.Fprintf(&b, "url = %s\n", c.URL)
	fmt.Fprintf(&b, "user = %s\n", c.User)
	fmt.Fprintf(&b, "password = %s\n", c.Password)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// setupConfig asks the user for the connection details and saves them to disk.
func setupConfig(path string) (*Config, error) {
	in := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("Navidrome setup")
	fmt.Println("---------------")
	fmt.Printf("Configuration file: %s\n\n", path)

	url := askLine(in, "Server URL (e.g. http://192.168.1.10:4533)", "")
	if url == "" {
		return nil, fmt.Errorf("URL is required")
	}
	user := askLine(in, "Username", "")
	if user == "" {
		return nil, fmt.Errorf("username is required")
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
		return nil, fmt.Errorf("password is required")
	}

	c := &Config{URL: url, User: user, Password: pwd}
	if err := saveConfig(path, c); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot save the configuration: %v\n", err)
	} else {
		fmt.Printf("Configuration saved to %s (permissions 600)\n", path)
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
