package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Token string `json:"token"`
}

func LoadOrCreateConfig(token string) (*Config, error) {
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	if token != "" {
		return &Config{Token: token}, nil
	}

	data, err := os.ReadFile(cfgPath)
	if err == nil {
		var cfg Config
		if json.Unmarshal(data, &cfg) == nil && cfg.Token != "" {
			return &cfg, nil
		}
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	cfg := &Config{Token: hex.EncodeToString(buf)}

	_ = os.MkdirAll(filepath.Dir(cfgPath), 0700)
	data, _ = json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(cfgPath, data, 0600)

	return cfg, nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ollama-farm", "config.json"), nil
}
