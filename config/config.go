package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"plexcovermanager/models"
)

const appDirName = "PlexCoverManager"
const configFileName = "config.json"

type Manager struct {
	mu   sync.RWMutex
	path string
	cfg  models.AppConfig
}

func NewManager() (*Manager, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	manager := &Manager{path: path, cfg: models.DefaultConfig()}
	if err := manager.Load(); err != nil {
		return nil, err
	}
	return manager, nil
}

func ConfigPath() (string, error) {
	base := os.Getenv("APPDATA")
	if base == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = dir
	}
	return filepath.Join(base, appDirName, configFileName), nil
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) Get() models.AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := m.cfg
	cfg.MediaPaths = append([]models.MediaPath(nil), m.cfg.MediaPaths...)
	return cfg
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			m.cfg = models.DefaultConfig()
			return m.saveLocked()
		}
		return err
	}
	if len(data) == 0 {
		m.cfg = models.DefaultConfig()
		return m.saveLocked()
	}
	var cfg models.AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	cfg.Normalize()
	m.cfg = cfg
	return nil
}

func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.Normalize()
	return m.saveLocked()
}

func (m *Manager) Update(fn func(*models.AppConfig)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(&m.cfg)
	m.cfg.Normalize()
	return m.saveLocked()
}

func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, append(data, '\n'), 0o644)
}
