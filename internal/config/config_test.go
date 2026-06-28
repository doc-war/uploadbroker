package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "broker.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoadMinimal(t *testing.T) {
	yaml := `
base_url: https://upload.example.com
url_blake2b_salts:
  - my-salt
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Listen != "127.0.0.1:0" {
		t.Fatalf("Listen = %s, want 127.0.0.1:0", cfg.Listen)
	}
	if cfg.BaseURL != "https://upload.example.com" {
		t.Fatalf("BaseURL = %s, want https://upload.example.com", cfg.BaseURL)
	}
	if len(cfg.URLBlake2bSalts) != 1 || cfg.URLBlake2bSalts[0] != "my-salt" {
		t.Fatalf("URLBlake2bSalts = %v", cfg.URLBlake2bSalts)
	}
	if cfg.URLPrefix != "tmp" {
		t.Fatalf("URLPrefix = %s, want tmp", cfg.URLPrefix)
	}
	if cfg.MetadataDB != "./data/broker.db" {
		t.Fatalf("MetadataDB = %s", cfg.MetadataDB)
	}
	if cfg.CleanupInterval != 10*time.Minute {
		t.Fatalf("CleanupInterval = %v", cfg.CleanupInterval)
	}
	if cfg.DefaultTTL != 24*time.Hour {
		t.Fatalf("DefaultTTL = %v", cfg.DefaultTTL)
	}
	if int64(cfg.Limits.Image) != int64(2<<20) {
		t.Fatalf("Limits.Image = %d, want %d", cfg.Limits.Image, 2<<20)
	}
	if cfg.Storage.UploadDriver != "local" {
		t.Fatalf("Storage.UploadDriver = %s", cfg.Storage.UploadDriver)
	}
	if cfg.Storage.Drivers["local"].Provider != "local" {
		t.Fatalf("local provider = %s", cfg.Storage.Drivers["local"].Provider)
	}
	if cfg.Storage.Drivers["local"].Root != "./data/objects" {
		t.Fatalf("local root = %s", cfg.Storage.Drivers["local"].Root)
	}
}

func TestLoadFull(t *testing.T) {
	yaml := `
listen: 127.0.0.1:9001
base_url: https://cdn.example.com
url_blake2b_salts:
  - current-salt
  - previous-salt
url_prefix: files
metadata_db: /var/data/broker.db
cleanup_interval: 30m
default_ttl: 12h
hmac_secret: shared-secret

limits:
  image: 5MB
  audio: 10MB
  video: 50MB

storage:
  upload_driver: local
  drivers:
    local:
      root: /var/objects
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Listen != "127.0.0.1:9001" {
		t.Fatalf("Listen = %s", cfg.Listen)
	}
	if cfg.BaseURL != "https://cdn.example.com" {
		t.Fatalf("BaseURL = %s", cfg.BaseURL)
	}
	if len(cfg.URLBlake2bSalts) != 2 {
		t.Fatalf("expected 2 salts, got %d", len(cfg.URLBlake2bSalts))
	}
	if cfg.URLPrefix != "files" {
		t.Fatalf("URLPrefix = %s", cfg.URLPrefix)
	}
	if cfg.MetadataDB != "/var/data/broker.db" {
		t.Fatalf("MetadataDB = %s", cfg.MetadataDB)
	}
	if cfg.CleanupInterval != 30*time.Minute {
		t.Fatalf("CleanupInterval = %v", cfg.CleanupInterval)
	}
	if cfg.DefaultTTL != 12*time.Hour {
		t.Fatalf("DefaultTTL = %v", cfg.DefaultTTL)
	}
	if cfg.HMACSecret != "shared-secret" {
		t.Fatalf("HMACSecret = %s", cfg.HMACSecret)
	}
	if int64(cfg.Limits.Image) != int64(5*1024*1024) {
		t.Fatalf("Limits.Image = %d, want %d", cfg.Limits.Image, 5*1024*1024)
	}
	if int64(cfg.Limits.Audio) != int64(10*1024*1024) {
		t.Fatalf("Limits.Audio = %d", cfg.Limits.Audio)
	}
	if int64(cfg.Limits.Video) != int64(50*1024*1024) {
		t.Fatalf("Limits.Video = %d", cfg.Limits.Video)
	}
	if cfg.Storage.UploadDriver != "local" {
		t.Fatalf("UploadDriver = %s", cfg.Storage.UploadDriver)
	}
	if cfg.Storage.Drivers["local"].Root != "/var/objects" {
		t.Fatalf("local root = %s", cfg.Storage.Drivers["local"].Root)
	}
}

func TestLoadMissingBaseURL(t *testing.T) {
	yaml := `url_blake2b_salts: ["salt"]`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
}

func TestLoadMissingSalts(t *testing.T) {
	yaml := `base_url: https://example.com`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.URLBlake2bSalts) != 1 || cfg.URLBlake2bSalts[0] != "" {
		t.Fatalf("expected [\"\"], got %v", cfg.URLBlake2bSalts)
	}
}

func TestLoadTooManySalts(t *testing.T) {
	yaml := `
base_url: https://example.com
url_blake2b_salts:
  - salt1
  - salt2
  - salt3
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for >2 salts")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/broker.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeConfig(t, "invalid: yaml: : :")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"2MB", 2 * 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"500KB", 500 * 1024},
		{"0MB", 0},
	}

	for _, tt := range tests {
		got, err := parseSize(tt.input)
		if err != nil {
			t.Errorf("parseSize(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseSizeInvalid(t *testing.T) {
	_, err := parseSize("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}

	_, err = parseSize("abc")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}

	_, err = parseSize("10XB")
	if err == nil {
		t.Fatal("expected error for unknown unit")
	}
}

func TestSizeBytesUnmarshal(t *testing.T) {
	path := writeConfig(t, "base_url: https://x.com\nurl_blake2b_salts: [s]\nlimits:\n  image: 5MB\n  audio: 3MB\n  video: 10MB")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if int64(cfg.Limits.Image) != int64(5*1024*1024) {
		t.Fatalf("Image limit = %d", cfg.Limits.Image)
	}
}
