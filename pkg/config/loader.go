package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cyd01/gorp/pkg/helper"

	"gopkg.in/yaml.v3"
)

func Load(filename string) (*Config, error) {
	data, err := helper.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return loadConfigBytes(data)
}

func LoadDir(dir string) (*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read config dir %q: %w", dir, err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".yaml") && !strings.HasSuffix(strings.ToLower(name), ".yml") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var docs []string
	for _, name := range names {
		doc, err := helper.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read config file %q: %w", name, err)
		}
		docs = append(docs, string(doc))
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no yaml files found in config directory %q", dir)
	}

	combined := strings.Join(docs, "\n---\n")
	return loadConfigBytes([]byte(combined))
}

func loadConfigBytes(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err == nil {
		sortRoutesByPrefixLength(cfg)
		return cfg, nil
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc Config
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(doc.Listeners) == 0 && len(doc.Services) == 0 && len(doc.Routes) == 0 && doc.Admin.Address == "" && doc.Admin.Enabled == false && doc.Directory == "" {
			continue
		}
		if doc.Admin.Address != "" || doc.Admin.Enabled || doc.Admin.TLS.CertFile != "" || doc.Admin.TLS.KeyFile != "" || doc.Admin.TLS.CAFile != "" || doc.Admin.TLS.CRLFile != "" || doc.Admin.TLS.OCSPURL != "" || len(doc.Admin.Middlewares) > 0 {
			cfg.Admin = doc.Admin
		}
		cfg.Listeners = append(cfg.Listeners, doc.Listeners...)
		cfg.Services = append(cfg.Services, doc.Services...)
		cfg.Routes = append(cfg.Routes, doc.Routes...)
		if doc.Directory != "" {
			cfg.Directory = doc.Directory
		}
	}

	if len(cfg.Listeners) == 0 && len(cfg.Routes) == 0 && cfg.Admin.Address == "" && cfg.Admin.Enabled == false && cfg.Directory == "" {
		return nil, fmt.Errorf("empty or invalid config")
	}

	// Sort routes by prefix length (descending)
	// This ensures longer/more specific prefixes are checked first
	sortRoutesByPrefixLength(cfg)

	return cfg, nil
}

// sortRoutesByPrefixLength sorts routes by prefix length in descending order
func sortRoutesByPrefixLength(cfg *Config) {
	sort.Slice(cfg.Routes, func(i, j int) bool {
		// Longer prefixes first (descending)
		return len(cfg.Routes[i].Prefix) > len(cfg.Routes[j].Prefix)
	})
}
