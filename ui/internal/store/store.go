// Package store persists pgslot-ui's saved database connection profiles to
// a local JSON file -- single-machine, no server-side storage, matching the
// CLI's PG*/-dsn convention but remembered across runs.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Connection struct {
	Name string `json:"name"`
	DSN  string `json:"dsn"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pgslot-ui", "connections.json"), nil
}

// Load returns saved connections, or an empty slice if the config file
// doesn't exist yet.
func Load() ([]Connection, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var conns []Connection
	if err := json.Unmarshal(data, &conns); err != nil {
		return nil, err
	}
	return conns, nil
}

func Save(conns []Connection) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(conns, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Add appends a connection, replacing any existing one with the same name.
func Add(conns []Connection, c Connection) []Connection {
	for i, existing := range conns {
		if existing.Name == c.Name {
			conns[i] = c
			return conns
		}
	}
	return append(conns, c)
}

// Remove drops the connection with the given name, if present.
func Remove(conns []Connection, name string) []Connection {
	out := conns[:0]
	for _, c := range conns {
		if c.Name != name {
			out = append(out, c)
		}
	}
	return out
}
