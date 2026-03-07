package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Profile holds persisted user preferences.
type Profile struct {
	PersonaName string `json:"persona_name"`
	ModelName   string `json:"model_name,omitempty"`
}

func profilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lazyreview", "profile.json"), nil
}

// Load reads the profile from disk. Returns a zero Profile on any error.
func Load() Profile {
	path, err := profilePath()
	if err != nil {
		return Profile{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}
	}
	var p Profile
	if json.Unmarshal(data, &p) != nil {
		return Profile{}
	}
	return p
}

// Save writes the profile to disk, creating the directory if needed.
func Save(p Profile) {
	path, err := profilePath()
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, data, 0644)
}
