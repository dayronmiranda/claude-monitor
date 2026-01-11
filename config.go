package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"claude-monitor/pkg/logger"
)

// Environment variable names
const (
	EnvPassword = "CLAUDE_MONITOR_PASSWORD"
	EnvAPIToken = "CLAUDE_MONITOR_API_TOKEN"
	EnvUsername = "CLAUDE_MONITOR_USERNAME"
)

// Config configuración del servidor
type Config struct {
	// Server
	Port     int    `json:"port"`
	Host     string `json:"host"`
	HostName string `json:"host_name"`

	// Auth (sensibles - no serializar a JSON, solo desde env vars)
	Username string `json:"-"`
	Password string `json:"-"`
	APIToken string `json:"-"`

	// CORS
	AllowedOrigins []string `json:"allowed_origins"`

	// Security
	AllowedPathPrefixes []string `json:"allowed_path_prefixes"`

	// Paths
	ClaudeDir  string `json:"claude_dir"`
	WorkingDir string `json:"working_dir"`

	// Cache
	CacheDurationMinutes int `json:"cache_duration_minutes"`
}

// DefaultConfig configuración por defecto
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	hostname, _ := os.Hostname()

	return &Config{
		Port:                 9090,
		Host:                 "0.0.0.0",
		HostName:             hostname,
		Username:             "admin",
		Password:             "",
		APIToken:             "",
		AllowedOrigins:       []string{}, // Vacío = permitir todos (desarrollo)
		AllowedPathPrefixes:  []string{}, // Vacío = permitir todos (desarrollo)
		ClaudeDir:            filepath.Join(homeDir, ".claude", "projects"),
		WorkingDir:           homeDir,
		CacheDurationMinutes: 5,
	}
}

// LoadConfig carga configuración desde archivo y env vars
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Cargar desde archivo (solo campos no sensibles)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Crear archivo con valores por defecto
			if err := SaveConfig(path, cfg); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Cargar credenciales desde env vars (prioridad sobre defaults)
	loadCredentialsFromEnv(cfg)

	// Validar que hay autenticación configurada
	validateAuth(cfg)

	return cfg, nil
}

// loadCredentialsFromEnv carga credenciales desde variables de entorno
func loadCredentialsFromEnv(cfg *Config) {
	if username := os.Getenv(EnvUsername); username != "" {
		cfg.Username = username
	}

	if password := os.Getenv(EnvPassword); password != "" {
		cfg.Password = password
	}

	if token := os.Getenv(EnvAPIToken); token != "" {
		cfg.APIToken = token
	}
}

// validateAuth valida que hay al menos un método de autenticación
func validateAuth(cfg *Config) {
	log := logger.Get()

	if cfg.Password == "" && cfg.APIToken == "" {
		log.Warn("Sin autenticación configurada",
			"hint", "Usa "+EnvPassword+" o "+EnvAPIToken+" para configurar credenciales",
		)
		// Usar password por defecto para desarrollo (inseguro)
		cfg.Password = "admin"
		log.Warn("Usando password por defecto 'admin' - SOLO PARA DESARROLLO")
	}

	if cfg.Password != "" {
		log.Info("Autenticación Basic Auth configurada", "user", cfg.Username)
	}

	if cfg.APIToken != "" {
		log.Info("Autenticación API Token configurada")
	}
}

// SaveConfig guarda configuración a archivo (sin credenciales)
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Global config instance
var config *Config
