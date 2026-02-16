// AIVory Go Agent Test Application
//
// Generates various panic types to test exception capture and local variable extraction.
//
// Usage:
//
//	cd monitor-agents/agent-go
//	go mod tidy
//	AIVORY_API_KEY=test-key-123 AIVORY_BACKEND_URL=ws://localhost:19999/api/monitor/agent/v1 AIVORY_DEBUG=true go run ./cmd/testapp/
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aivorynet/agent-go/pkg/agent"
)

// UserContext is a helper struct to test object capture.
type UserContext struct {
	UserID string
	Email  string
	Active bool
}

func main() {
	fmt.Println("===========================================")
	fmt.Println("AIVory Go Agent Test Application")
	fmt.Println("===========================================")

	// Initialize the agent
	agent.Init(
		agent.WithDebug(true),
	)
	defer agent.Shutdown()

	// Set user context
	agent.SetUser("test-user-001", "tester@example.com", "tester")

	// Wait for agent to connect
	fmt.Println("Waiting for agent to connect...")
	time.Sleep(3 * time.Second)
	fmt.Println("Starting panic tests...")
	fmt.Println()

	// Generate test panics
	for i := 0; i < 3; i++ {
		fmt.Printf("--- Test %d ---\n", i+1)

		// Wrap in a function to recover from panic
		func() {
			// CapturePanic must be deferred AFTER the recover handler
			// because defers run in LIFO order. CapturePanic will capture
			// and re-panic, then the outer recover handles it.
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Recovered from panic: %v\n", r)
				}
			}()
			defer agent.CapturePanic() // This runs first, captures, then re-panics
			triggerPanic(i)
		}()

		fmt.Println()
		time.Sleep(3 * time.Second)
	}

	// Also test manual error capture
	fmt.Println("--- Manual Error Capture Test ---")
	err := fmt.Errorf("manually triggered test error")
	agent.CaptureError(err, map[string]interface{}{
		"test_type": "manual",
		"iteration": 99,
	})
	fmt.Printf("Captured error: %v\n", err)

	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("Test complete. Check database for exceptions.")
	fmt.Println("===========================================")

	// Keep running briefly to allow final messages to send
	time.Sleep(2 * time.Second)
}

func triggerPanic(iteration int) {
	// Create some local variables to capture
	testVar := fmt.Sprintf("test-value-%d", iteration)
	count := iteration * 10
	items := []string{"apple", "banana", "cherry"}
	metadata := map[string]interface{}{
		"iteration": iteration,
		"timestamp": time.Now().Unix(),
		"nested":    map[string]interface{}{"key": "value", "count": count},
	}
	user := UserContext{
		UserID: fmt.Sprintf("user-%d", iteration),
		Email:  "test@example.com",
		Active: true,
	}

	// Use variables to prevent "unused variable" errors
	log.Printf("Variables: testVar=%s, count=%d, items=%v, user=%v", testVar, count, items, user)
	_ = metadata

	switch iteration {
	case 0:
		// Nil pointer dereference (like NullPointerException)
		fmt.Println("Triggering nil pointer panic...")
		var nilSlice []int
		_ = nilSlice[0] // panic: index out of range

	case 1:
		// Explicit panic
		fmt.Println("Triggering explicit panic...")
		panic("Test panic error")

	case 2:
		// Nil map assignment
		fmt.Println("Triggering nil map panic...")
		var m map[string]int
		m["key"] = 1 // panic: assignment to entry in nil map

	default:
		panic(fmt.Sprintf("Unknown iteration: %d", iteration))
	}
}
