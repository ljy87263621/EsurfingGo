package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadFileConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "esurfing.json")
	data := []byte(`{
		"user": "18900000000",
		"password": "secret",
		"sms": "123456",
		"network": 2,
		"log_file": "esurfing.log",
		"save_credentials": true,
		"start_with_windows": true
	}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadFileConfig(path)
	if err != nil {
		t.Fatalf("loadFileConfig returned error: %v", err)
	}

	if cfg.User != "18900000000" || cfg.Password != "secret" || cfg.SMSCode != "123456" || cfg.Network != 2 || cfg.LogFile != "esurfing.log" || !cfg.SaveCredentials || !cfg.StartWithWindows {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestSaveFileConfigRoundTripsGUISettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "esurfing.local.json")
	want := FileConfig{
		User:             "18900000000",
		Password:         "secret",
		Network:          3,
		SaveCredentials:  true,
		StartWithWindows: true,
	}

	if err := saveFileConfig(path, want); err != nil {
		t.Fatalf("saveFileConfig returned error: %v", err)
	}

	got, err := loadFileConfig(path)
	if err != nil {
		t.Fatalf("loadFileConfig returned error: %v", err)
	}
	if got != want {
		t.Fatalf("round trip config = %+v, want %+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestGUIConfigForSaveClearsCredentialsWhenDisabled(t *testing.T) {
	cfg := guiConfigForSave(FileConfig{
		SMSCode:  "legacy-code",
		LogFile:  "esurfing.log",
		User:     "old-user",
		Password: "old-password",
	}, "new-user", "new-password", 2, false, true)

	if cfg.User != "" || cfg.Password != "" {
		t.Fatalf("credentials should be cleared when saving is disabled: %+v", cfg)
	}
	if cfg.SMSCode != "legacy-code" || cfg.LogFile != "esurfing.log" || cfg.Network != 2 || !cfg.StartWithWindows {
		t.Fatalf("unrelated settings changed: %+v", cfg)
	}
}

func TestApplyFileConfigKeepsExplicitFlags(t *testing.T) {
	user := "cli-user"
	password := ""
	smsCode := ""
	logFile := ""
	networkIdx := 9

	applyFileConfig(FileConfig{
		User:     "config-user",
		Password: "config-password",
		SMSCode:  "654321",
		Network:  2,
		LogFile:  "esurfing.log",
	}, map[string]bool{"u": true, "network": true}, &user, &password, &smsCode, &logFile, &networkIdx)

	if user != "cli-user" {
		t.Fatalf("user was overridden: %q", user)
	}
	if password != "config-password" {
		t.Fatalf("password = %q, want config-password", password)
	}
	if smsCode != "654321" {
		t.Fatalf("smsCode = %q, want 654321", smsCode)
	}
	if networkIdx != 9 {
		t.Fatalf("networkIdx was overridden: %d", networkIdx)
	}
	if logFile != "esurfing.log" {
		t.Fatalf("logFile = %q, want esurfing.log", logFile)
	}
}

func TestSetupLogOutputCreatesParentDirectory(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "logs", "esurfing.log")
	oldOutput := log.Writer()
	file, err := setupLogOutput(logPath)
	if err != nil {
		t.Fatalf("setupLogOutput returned error: %v", err)
	}
	defer func() {
		log.SetOutput(oldOutput)
		file.Close()
	}()

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log file was not created: %v", err)
	}
}
