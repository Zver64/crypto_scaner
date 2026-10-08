package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// config is the CLI configuration file: server profiles and the current one.
// It holds API tokens, so it is readable only by its owner.
type config struct {
	Current  string             `json:"current,omitempty"`
	Profiles map[string]profile `json:"profiles"`
}

type profile struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

// configPath is $XDG_CONFIG_HOME/scanner/config.json, or
// ~/.config/scanner/config.json without it.
func configPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find the home directory: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "scanner", "config.json"), nil
}

// readConfig returns the configuration file's path and contents.
func readConfig() (string, config, error) {
	path, err := configPath()
	if err != nil {
		return "", config{}, failure("config", "%v", err)
	}
	loaded, err := loadConfig(path)
	if err != nil {
		return "", config{}, failure("config", "%v", err)
	}
	return path, loaded, nil
}

// loadConfig returns an empty configuration when the file does not exist.
func loadConfig(path string) (config, error) {
	loaded := config{Profiles: map[string]profile{}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return loaded, nil
	}
	if err != nil {
		return config{}, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if loaded.Profiles == nil {
		loaded.Profiles = map[string]profile{}
	}
	return loaded, nil
}

// saveConfig replaces the file atomically with one only its owner can read.
func saveConfig(path string, saved config) error {
	if err := writeConfig(path, saved); err != nil {
		return failure("config", "%v", err)
	}
	return nil
}

// writeConfig writes the file of saveConfig. A missing directory is created
// for its owner only; an existing one keeps its permissions.
func writeConfig(path string, saved config) error {
	raw, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	// CreateTemp creates the file with mode 0600.
	file, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	defer os.Remove(file.Name())
	_, err = file.Write(append(raw, '\n'))
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}
