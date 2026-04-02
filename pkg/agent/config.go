// Package agent provides the AIVory Monitor Go agent.
package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

// Config holds the agent configuration.
type Config struct {
	APIKey            string
	BackendURL        string
	Environment       string
	SamplingRate      float64
	MaxCaptureDepth   int
	MaxStringLength   int
	MaxCollectionSize int
	Debug             bool
	EnableBreakpoints bool
	Hostname          string
	AgentID           string
}

// NewConfig creates a new configuration with defaults from environment variables.
func NewConfig(options ...ConfigOption) *Config {
	cfg := &Config{
		APIKey:            getEnvOrDefault("AIVORY_API_KEY", ""),
		BackendURL:        getEnvOrDefault("AIVORY_BACKEND_URL", "wss://api.aivory.net/ws/agent"),
		Environment:       getEnvOrDefault("AIVORY_ENVIRONMENT", "production"),
		SamplingRate:      getEnvFloatOrDefault("AIVORY_SAMPLING_RATE", 1.0),
		MaxCaptureDepth:   getEnvIntOrDefault("AIVORY_MAX_DEPTH", 10),
		MaxStringLength:   getEnvIntOrDefault("AIVORY_MAX_STRING_LENGTH", 1000),
		MaxCollectionSize: getEnvIntOrDefault("AIVORY_MAX_COLLECTION_SIZE", 100),
		Debug:             getEnvOrDefault("AIVORY_DEBUG", "false") == "true",
		EnableBreakpoints: getEnvOrDefault("AIVORY_ENABLE_BREAKPOINTS", "true") == "true",
	}

	// Generate hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	cfg.Hostname = hostname

	// Generate agent ID
	cfg.AgentID = generateAgentID()

	// Apply options
	for _, opt := range options {
		opt(cfg)
	}

	return cfg
}

// ConfigOption is a function that modifies Config.
type ConfigOption func(*Config)

// WithAPIKey sets the API key.
func WithAPIKey(key string) ConfigOption {
	return func(c *Config) {
		c.APIKey = key
	}
}

// WithBackendURL sets the backend URL.
func WithBackendURL(url string) ConfigOption {
	return func(c *Config) {
		c.BackendURL = url
	}
}

// WithEnvironment sets the environment name.
func WithEnvironment(env string) ConfigOption {
	return func(c *Config) {
		c.Environment = env
	}
}

// WithSamplingRate sets the sampling rate.
func WithSamplingRate(rate float64) ConfigOption {
	return func(c *Config) {
		c.SamplingRate = rate
	}
}

// WithDebug enables debug logging.
func WithDebug(debug bool) ConfigOption {
	return func(c *Config) {
		c.Debug = debug
	}
}

// WithEnableBreakpoints enables or disables breakpoint support.
func WithEnableBreakpoints(enable bool) ConfigOption {
	return func(c *Config) {
		c.EnableBreakpoints = enable
	}
}

// ShouldSample returns true if the current event should be sampled.
func (c *Config) ShouldSample() bool {
	if c.SamplingRate >= 1.0 {
		return true
	}
	if c.SamplingRate <= 0.0 {
		return false
	}

	// Simple random sampling
	var b [8]byte
	rand.Read(b[:])
	r := float64(b[0]) / 256.0
	return r < c.SamplingRate
}

// RuntimeInfo contains Go runtime information.
type RuntimeInfo struct {
	Runtime        string `json:"runtime"`
	RuntimeVersion string `json:"runtime_version"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	NumCPU         int    `json:"num_cpu"`
	NumGoroutine   int    `json:"num_goroutine"`
}

// GetRuntimeInfo returns current runtime information.
func (c *Config) GetRuntimeInfo() RuntimeInfo {
	return RuntimeInfo{
		Runtime:        "go",
		RuntimeVersion: runtime.Version(),
		Platform:       runtime.GOOS,
		Arch:           runtime.GOARCH,
		NumCPU:         runtime.NumCPU(),
		NumGoroutine:   runtime.NumGoroutine(),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func generateAgentID() string {
	timestamp := fmt.Sprintf("%x", time.Now().Unix())
	random := make([]byte, 4)
	rand.Read(random)
	return fmt.Sprintf("agent-%s-%s", timestamp, hex.EncodeToString(random))
}
