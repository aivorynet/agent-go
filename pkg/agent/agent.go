package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/aivorynet/agent-go/pkg/capture"
	"github.com/aivorynet/agent-go/pkg/transport"
)

// Agent is the main AIVory Monitor agent.
type Agent struct {
	config     *Config
	connection *transport.Connection
	started    bool
	mu         sync.RWMutex

	// Custom context
	customContext map[string]interface{}
	user          map[string]string
}

var (
	globalAgent *Agent
	globalOnce  sync.Once
)

// Init initializes the global agent with the given options.
func Init(options ...ConfigOption) *Agent {
	globalOnce.Do(func() {
		config := NewConfig(options...)

		if config.APIKey == "" {
			log.Println("[AIVory Monitor] API key is required. Set AIVORY_API_KEY or use WithAPIKey option.")
			return
		}

		globalAgent = &Agent{
			config:        config,
			customContext: make(map[string]interface{}),
			user:          make(map[string]string),
		}

		globalAgent.Start()

		log.Printf("[AIVory Monitor] Agent v1.0.0 initialized")
		log.Printf("[AIVory Monitor] Environment: %s", config.Environment)
	})

	return globalAgent
}

// GetAgent returns the global agent instance.
func GetAgent() *Agent {
	return globalAgent
}

// Start starts the agent.
func (a *Agent) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return
	}

	// Initialize connection
	a.connection = transport.NewConnection(a.config.BackendURL, a.config.APIKey, a.config.Debug)

	// Connect to backend
	go a.connection.Connect(context.Background())

	// Handle shutdown signals
	go a.handleSignals()

	a.started = true

	if a.config.Debug {
		log.Println("[AIVory Monitor] Agent started")
	}
}

// Stop stops the agent.
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started {
		return
	}

	if a.connection != nil {
		a.connection.Disconnect()
	}

	a.started = false

	if a.config.Debug {
		log.Println("[AIVory Monitor] Agent stopped")
	}
}

// CaptureError captures an error with optional context.
func (a *Agent) CaptureError(err error, ctx ...map[string]interface{}) {
	if !a.started || !a.config.ShouldSample() {
		return
	}

	var context map[string]interface{}
	if len(ctx) > 0 {
		context = ctx[0]
	}

	captured := capture.CaptureError(err, a.config.MaxCaptureDepth, context)
	captured.AgentID = a.config.AgentID
	captured.Environment = a.config.Environment
	captured.Runtime = "go"
	ri := a.config.GetRuntimeInfo()
	captured.RuntimeInfo = capture.RuntimeInfo{
		Runtime:        ri.Runtime,
		RuntimeVersion: ri.RuntimeVersion,
		Platform:       ri.Platform,
		Arch:           ri.Arch,
		NumCPU:         ri.NumCPU,
		NumGoroutine:   ri.NumGoroutine,
	}

	// Add custom context
	a.mu.RLock()
	for k, v := range a.customContext {
		captured.Context[k] = v
	}
	if len(a.user) > 0 {
		captured.Context["user"] = a.user
	}
	a.mu.RUnlock()

	if a.connection != nil {
		a.connection.SendException(captured)
	}
}

// handlePanic handles a recovered panic value (internal use).
func (a *Agent) handlePanic(r interface{}) {
	var err error
	switch v := r.(type) {
	case error:
		err = v
	case string:
		err = fmt.Errorf("%s", v)
	default:
		err = fmt.Errorf("%v", v)
	}

	a.CaptureError(err, map[string]interface{}{"panic": true})
}

// CapturePanic captures a panic value with recovery.
// IMPORTANT: Must be called directly as a deferred function because
// recover() only works when called directly by a deferred function.
// Use: defer agent.CapturePanic()
func (a *Agent) CapturePanic() {
	if r := recover(); r != nil {
		a.handlePanic(r)
		// Re-panic to maintain normal behavior
		panic(r)
	}
}

// SetContext sets custom context that will be sent with all captures.
func (a *Agent) SetContext(ctx map[string]interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.customContext = make(map[string]interface{})
	for k, v := range ctx {
		a.customContext[k] = v
	}
}

// SetUser sets the current user information.
func (a *Agent) SetUser(id, email, username string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.user = make(map[string]string)
	if id != "" {
		a.user["id"] = id
	}
	if email != "" {
		a.user["email"] = email
	}
	if username != "" {
		a.user["username"] = username
	}
}

// Config returns the agent configuration.
func (a *Agent) Config() *Config {
	return a.config
}

func (a *Agent) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	a.Stop()
}

// Package-level convenience functions

// CaptureError captures an error using the global agent.
func CaptureError(err error, ctx ...map[string]interface{}) {
	if globalAgent != nil {
		globalAgent.CaptureError(err, ctx...)
	}
}

// CapturePanic captures a panic using the global agent.
// IMPORTANT: recover() must be called directly in the deferred function,
// so we call recover() here and pass the value to handlePanic.
// Use: defer agent.CapturePanic()
func CapturePanic() {
	if r := recover(); r != nil {
		if globalAgent != nil {
			globalAgent.handlePanic(r)
		}
		// Re-panic to maintain normal behavior
		panic(r)
	}
}

// SetContext sets custom context using the global agent.
func SetContext(ctx map[string]interface{}) {
	if globalAgent != nil {
		globalAgent.SetContext(ctx)
	}
}

// SetUser sets user information using the global agent.
func SetUser(id, email, username string) {
	if globalAgent != nil {
		globalAgent.SetUser(id, email, username)
	}
}

// Shutdown stops the global agent.
func Shutdown() {
	if globalAgent != nil {
		globalAgent.Stop()
	}
}
