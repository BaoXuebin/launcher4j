// Package config provides JSON-based persistent configuration storage.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ProjectConfig represents a configured Maven project.
type ProjectConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	JDKHome     string `json:"jdk_home"`
	VMArgs      string `json:"vm_args"`
	EnvVars     string `json:"env_vars"` // KEY=VALUE, one per line
	AutoCompile bool   `json:"auto_compile"`
	AddedAt     string `json:"added_at"`
}

// AppSettings represents application-wide settings.
type AppSettings struct {
	Theme                 string `json:"theme"`
	MavenPath             string `json:"maven_path"`
	AutoCompileDebounceMs int    `json:"auto_compile_debounce_ms"`
}

// DefaultSettings returns the default application settings.
func DefaultSettings() AppSettings {
	return AppSettings{
		Theme:                 "dark",
		MavenPath:             "",
		AutoCompileDebounceMs: 500,
	}
}

// ConfigStore manages persistent configuration stored as JSON.
type ConfigStore struct {
	configDir  string
	configPath string
}

// NewConfigStore creates a ConfigStore with the default OS-specific config directory.
func NewConfigStore() *ConfigStore {
	var base string
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		base = appData
	} else {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	configDir := filepath.Join(base, "launcher4j")
	return NewConfigStoreWithDir(configDir)
}

// NewConfigStoreWithDir creates a ConfigStore with a specific config directory.
func NewConfigStoreWithDir(configDir string) *ConfigStore {
	os.MkdirAll(configDir, 0755)
	return &ConfigStore{
		configDir:  configDir,
		configPath: filepath.Join(configDir, "launcher4j.json"),
	}
}

// ConfigDir returns the configuration directory path.
func (s *ConfigStore) ConfigDir() string { return s.configDir }

// ConfigPath returns the configuration file path.
func (s *ConfigStore) ConfigPath() string { return s.configPath }

func (s *ConfigStore) loadRaw() (map[string]any, error) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return make(map[string]any), nil
	}
	return raw, nil
}

func (s *ConfigStore) saveRaw(data map[string]any) error {
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, payload, 0644)
}

// LoadProjects returns all configured projects.
func (s *ConfigStore) LoadProjects() ([]ProjectConfig, error) {
	raw, err := s.loadRaw()
	if err != nil {
		return nil, err
	}
	projectsData, _ := raw["projects"].([]any)
	projects := make([]ProjectConfig, 0, len(projectsData))
	for _, p := range projectsData {
		b, err := json.Marshal(p)
		if err != nil {
			continue
		}
		var pc ProjectConfig
		if err := json.Unmarshal(b, &pc); err != nil {
			continue
		}
		projects = append(projects, pc)
	}
	return projects, nil
}

// SaveProject adds or updates a project.
func (s *ConfigStore) SaveProject(project ProjectConfig) error {
	raw, err := s.loadRaw()
	if err != nil {
		return err
	}
	projectsData, _ := raw["projects"].([]any)
	projects := make([]map[string]any, 0, len(projectsData))
	for _, p := range projectsData {
		if m, ok := p.(map[string]any); ok {
			projects = append(projects, m)
		}
	}

	found := false
	projMap := structToMap(project)
	for i, p := range projects {
		if p["id"] == project.ID {
			projects[i] = projMap
			found = true
			break
		}
	}
	if !found {
		projects = append(projects, projMap)
	}

	raw["projects"] = projects
	return s.saveRaw(raw)
}

// RemoveProject removes a project by ID.
func (s *ConfigStore) RemoveProject(projectID string) error {
	raw, err := s.loadRaw()
	if err != nil {
		return err
	}
	projectsData, _ := raw["projects"].([]any)
	filtered := make([]any, 0, len(projectsData))
	for _, p := range projectsData {
		if m, ok := p.(map[string]any); ok {
			if m["id"] != projectID {
				filtered = append(filtered, p)
			}
		}
	}
	raw["projects"] = filtered
	return s.saveRaw(raw)
}

// LoadSettings returns the application settings.
func (s *ConfigStore) LoadSettings() (AppSettings, error) {
	raw, err := s.loadRaw()
	if err != nil {
		return DefaultSettings(), err
	}
	settingsData, _ := raw["settings"].(map[string]any)
	if settingsData == nil {
		return DefaultSettings(), nil
	}
	b, err := json.Marshal(settingsData)
	if err != nil {
		return DefaultSettings(), nil
	}
	var settings AppSettings
	if err := json.Unmarshal(b, &settings); err != nil {
		return DefaultSettings(), nil
	}
	if settings.AutoCompileDebounceMs <= 0 {
		settings.AutoCompileDebounceMs = 500
	}
	if settings.Theme == "" {
		settings.Theme = "dark"
	}
	return settings, nil
}

// SaveSettings saves the application settings.
func (s *ConfigStore) SaveSettings(settings AppSettings) error {
	raw, err := s.loadRaw()
	if err != nil {
		return err
	}
	raw["settings"] = structToMap(settings)
	return s.saveRaw(raw)
}

func structToMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// DefaultProjectConfig returns a ProjectConfig with default values.
func DefaultProjectConfig(path, name string) ProjectConfig {
	if name == "" {
		name = filepath.Base(path)
	}
	return ProjectConfig{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:        name,
		Path:        path,
		JDKHome:     "java",
		AutoCompile: true,
		AddedAt:     time.Now().Format(time.RFC3339),
	}
}
