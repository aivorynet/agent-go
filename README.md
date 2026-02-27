# AIVory Monitor Go Agent

Go agent for capturing exceptions and panics with full context and local variables.

## Requirements

- Go 1.21 or higher

## Installation

```bash
go get github.com/aivory/aivory-monitor-go
```

Currently available as:
```bash
go get github.com/ilscipio/aivory-monitor/agent-go
```

## Usage

### Basic Initialization

```go
package main

import (
    "github.com/ilscipio/aivory-monitor/agent-go/pkg/agent"
)

func main() {
    // Initialize agent with API key
    agent.Init(
        agent.WithAPIKey("your-api-key"),
        agent.WithEnvironment("production"),
    )
    defer agent.Shutdown()

    // Your application code
}
```

### Panic Recovery

The agent captures panics using Go's `defer` and `recover` mechanism. IMPORTANT: `agent.CapturePanic()` must be deferred to work correctly.

```go
func handleRequest() {
    defer agent.CapturePanic()

    // Your code that might panic
    processData()
}
```

For nested recovery (capture and continue):

```go
func handleRequest() {
    defer func() {
        if r := recover(); r != nil {
            // Handle recovery (log, cleanup, etc.)
            fmt.Printf("Recovered: %v\n", r)
        }
    }()
    defer agent.CapturePanic() // Captures then re-panics

    // Your code
}
```

### Manual Error Capture

```go
func processOrder(orderID string) error {
    order, err := fetchOrder(orderID)
    if err != nil {
        // Capture error with context
        agent.CaptureError(err, map[string]interface{}{
            "order_id": orderID,
            "operation": "fetch_order",
        })
        return err
    }

    return nil
}
```

### HTTP Middleware Example

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/ilscipio/aivory-monitor/agent-go/pkg/agent"
)

func aivoryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer agent.CapturePanic()

        // Set request context
        agent.SetContext(map[string]interface{}{
            "path":   r.URL.Path,
            "method": r.Method,
            "ip":     r.RemoteAddr,
        })

        next.ServeHTTP(w, r)
    })
}

func main() {
    agent.Init(agent.WithAPIKey("your-api-key"))
    defer agent.Shutdown()

    mux := http.NewServeMux()
    mux.HandleFunc("/", handleHome)

    http.ListenAndServe(":8080", aivoryMiddleware(mux))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
}
```

### Setting User Context

```go
// Set user information for all subsequent captures
agent.SetUser(
    "user-123",              // User ID
    "user@example.com",      // Email
    "john_doe",              // Username
)

// Set custom context
agent.SetContext(map[string]interface{}{
    "tenant_id": "org-456",
    "feature_flags": []string{"new-ui", "beta-api"},
})
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AIVORY_API_KEY` | Agent authentication key | Required |
| `AIVORY_BACKEND_URL` | Backend WebSocket URL | `wss://api.aivory.net/monitor/agent` |
| `AIVORY_ENVIRONMENT` | Environment name | `production` |
| `AIVORY_SAMPLING_RATE` | Exception sampling (0-1) | `1.0` |
| `AIVORY_MAX_DEPTH` | Variable capture depth | `10` |
| `AIVORY_MAX_STRING_LENGTH` | Max string length in captures | `1000` |
| `AIVORY_MAX_COLLECTION_SIZE` | Max array/map size in captures | `100` |
| `AIVORY_DEBUG` | Enable debug logging | `false` |

### Configuration Options

```go
agent.Init(
    // Required
    agent.WithAPIKey("your-api-key"),

    // Optional
    agent.WithBackendURL("wss://api.aivory.net/monitor/agent"),
    agent.WithEnvironment("staging"),
    agent.WithSamplingRate(0.5),        // Sample 50% of errors
    agent.WithDebug(true),              // Enable debug logs
)
```

### Available Config Functions

- `WithAPIKey(key string)` - Set API key
- `WithBackendURL(url string)` - Set backend WebSocket URL
- `WithEnvironment(env string)` - Set environment name
- `WithSamplingRate(rate float64)` - Set sampling rate (0.0-1.0)
- `WithDebug(debug bool)` - Enable/disable debug logging

## Building from Source

```bash
cd monitor-agents/agent-go
go mod tidy
go build ./pkg/agent
```

## How It Works

### Panic Capture

The agent uses Go's built-in `defer` and `recover` mechanism to capture panics. When `agent.CapturePanic()` is deferred, it:

1. Calls `recover()` to capture the panic value
2. Extracts stack trace and local variables
3. Sends exception data to the backend via WebSocket
4. Re-panics to maintain normal panic behavior

### Goroutine Safety

The agent is goroutine-safe and uses `sync.RWMutex` to protect shared state. You can safely call agent methods from multiple goroutines.

### WebSocket Transport

The agent maintains a persistent WebSocket connection to the backend:

- Automatic reconnection on disconnect
- Heartbeat for connection monitoring
- Buffered message queue during connection loss

### Signal Handling

The agent automatically handles `SIGINT` and `SIGTERM` signals for graceful shutdown:

```go
agent.Init(agent.WithAPIKey("..."))
// Agent will automatically shutdown on SIGINT/SIGTERM

// Or manual shutdown
defer agent.Shutdown()
```

## Local Development Testing

### Quick Test with Test App

```bash
cd monitor-agents/agent-go

# Set environment variables
export AIVORY_API_KEY=ilscipio-dev-2024
export AIVORY_BACKEND_URL=ws://localhost:19999/ws/monitor/agent
export AIVORY_DEBUG=true

# Run test application
go run ./cmd/testapp/
```

The test app triggers various panic types:
- Nil pointer dereference (index out of range)
- Explicit panic with string message
- Nil map assignment

### Prerequisites for Local Testing

1. Backend running on `localhost:19999`
2. Dev token bypass enabled (uses `ilscipio-dev-2024`)
3. Org schema `org_test_20` exists in database

## Troubleshooting

**Agent not connecting:**
- Check backend is running: `curl http://localhost:19999/health`
- Verify WebSocket endpoint: `ws://localhost:19999/ws/monitor/agent`
- Check API key is set correctly
- Enable debug mode with `agent.WithDebug(true)`

**Panics not being captured:**
- Ensure `defer agent.CapturePanic()` is called before the panic
- Remember that `recover()` only works in deferred functions
- Check that the agent is initialized before panic occurs

**High memory usage:**
- Reduce `AIVORY_MAX_DEPTH` to capture fewer nested variables
- Reduce `AIVORY_MAX_COLLECTION_SIZE` for large arrays/maps
- Lower `AIVORY_SAMPLING_RATE` to capture fewer exceptions

**WebSocket connection issues:**
- Check firewall rules for WebSocket connections
- Verify backend URL includes protocol (`ws://` or `wss://`)
- Check backend logs for connection errors
- Enable debug logging to see connection attempts
