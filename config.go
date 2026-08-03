package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type FileConfig struct {
	User             string `json:"user"`
	Password         string `json:"password"`
	SMSCode          string `json:"sms"`
	Network          int    `json:"network"`
	LogFile          string `json:"log_file"`
	SaveCredentials  bool   `json:"save_credentials"`
	StartWithWindows bool   `json:"start_with_windows"`
}

func loadFileConfig(path string) (FileConfig, error) {
	var cfg FileConfig
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config file %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config file %q: %w", path, err)
	}
	return cfg, nil
}

func saveFileConfig(path string, cfg FileConfig) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config path is empty")
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create config directory %q: %w", dir, err)
		}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("set temporary config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err == nil {
		if err := os.Chmod(path, 0600); err != nil {
			return fmt.Errorf("set config permissions: %w", err)
		}
		return nil
	}
	// Windows cannot rename over an existing file. Keep the temporary file on
	// the same volume, then retry after replacing the old local config.
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("replace existing config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temporary config: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}
	return nil
}

func guiConfigForSave(existing FileConfig, user, password string, network int, saveCredentials, startWithWindows bool) FileConfig {
	existing.Network = network
	existing.SaveCredentials = saveCredentials
	existing.StartWithWindows = startWithWindows
	if saveCredentials {
		existing.User = user
		existing.Password = password
	} else {
		existing.User = ""
		existing.Password = ""
	}
	return existing
}

func applyFileConfig(cfg FileConfig, provided map[string]bool, user, password, smsCode, logFile *string, networkIdx *int) {
	if !flagProvided(provided, "u", "user") && cfg.User != "" {
		*user = cfg.User
	}
	if !flagProvided(provided, "p", "password") && cfg.Password != "" {
		*password = cfg.Password
	}
	if !flagProvided(provided, "c", "sms") && cfg.SMSCode != "" {
		*smsCode = cfg.SMSCode
	}
	if !flagProvided(provided, "n", "network") && cfg.Network != 0 {
		*networkIdx = cfg.Network
	}
	if !flagProvided(provided, "log-file") && cfg.LogFile != "" {
		*logFile = cfg.LogFile
	}
}

func flagProvided(provided map[string]bool, names ...string) bool {
	for _, name := range names {
		if provided[name] {
			return true
		}
	}
	return false
}

func setupLogOutput(logFile string) (*os.File, error) {
	if strings.TrimSpace(logFile) == "" {
		return nil, nil
	}

	dir := filepath.Dir(logFile)
	if dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create log directory %q: %w", dir, err)
		}
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", logFile, err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, file))
	return file, nil
}
