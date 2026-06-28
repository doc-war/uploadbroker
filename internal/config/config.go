package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen          string           `yaml:"listen"`
	HMACSecret      string           `yaml:"hmac_secret"`
	BaseURL         string           `yaml:"base_url"`
	URLBlake2bSalts []string         `yaml:"url_blake2b_salts"`
	URLPrefix       string           `yaml:"url_prefix"`
	MetadataDB      string           `yaml:"metadata_db"`
	CleanupInterval time.Duration    `yaml:"cleanup_interval"`
	DefaultTTL      time.Duration    `yaml:"default_ttl"`
	Limits          Limits           `yaml:"limits"`
	Storage         StorageConfig    `yaml:"storage"`
	Version         string           `yaml:"-"`
}

type Limits struct {
	Image    SizeBytes `yaml:"image"`
	Audio    SizeBytes `yaml:"audio"`
	Video    SizeBytes `yaml:"video"`
	Document SizeBytes `yaml:"document"`
}

type SizeBytes int64

func (s SizeBytes) String() string {
	return fmt.Sprintf("%dMB", int64(s)/(1024*1024))
}

func (s *SizeBytes) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	n, err := parseSize(raw)
	if err != nil {
		return err
	}
	*s = SizeBytes(n)
	return nil
}

type StorageConfig struct {
	UploadDriver string                 `yaml:"upload_driver"`
	Drivers       map[string]DriverConfig `yaml:"drivers"`
}

type DriverConfig struct {
	Root            string `yaml:"root,omitempty"`
	Provider        string `yaml:"provider,omitempty"`
	Endpoint        string `yaml:"endpoint,omitempty"`
	Bucket          string `yaml:"bucket,omitempty"`
	Region          string `yaml:"region,omitempty"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	Secure          *bool  `yaml:"secure,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.fillDefaults(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) fillDefaults() error {
	if c.Listen == "" {
		c.Listen = "127.0.0.1:0"
	}
	if c.URLPrefix == "" {
		c.URLPrefix = "tmp"
	}
	if c.MetadataDB == "" {
		c.MetadataDB = "./data/broker.db"
	}
	if c.CleanupInterval == 0 {
		c.CleanupInterval = 10 * time.Minute
	}
	if c.DefaultTTL == 0 {
		c.DefaultTTL = 24 * time.Hour
	}
	if c.Limits.Image == 0 {
		c.Limits.Image = SizeBytes(2 << 20)
	}
	if c.Limits.Audio == 0 {
		c.Limits.Audio = SizeBytes(3 << 20)
	}
	if c.Limits.Video == 0 {
		c.Limits.Video = SizeBytes(10 << 20)
	}
	if c.Limits.Document == 0 {
		c.Limits.Document = SizeBytes(2 << 20)
	}
	if c.Storage.UploadDriver == "" {
		c.Storage.UploadDriver = "local"
	}
	if c.Storage.Drivers == nil {
		c.Storage.Drivers = map[string]DriverConfig{
			"local": {Provider: "local", Root: "./data/objects"},
		}
	}
	if len(c.URLBlake2bSalts) == 0 {
		c.URLBlake2bSalts = []string{""}
	}
	if len(c.URLBlake2bSalts) > 2 {
		return fmt.Errorf("at most 2 url_blake2b_salts allowed")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	return nil
}

func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}

	unit := strings.ToUpper(s[len(s)-2:])
	numStr := strings.TrimSpace(s[:len(s)-2])

	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size: %s", s)
	}

	switch unit {
	case "KB":
		n *= 1024
	case "MB":
		n *= 1024 * 1024
	case "GB":
		n *= 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s (use KB, MB, GB)", unit)
	}
	return n, nil
}
