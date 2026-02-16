// Package capture provides error and panic capture functionality.
package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RuntimeInfo holds runtime environment information.
type RuntimeInfo struct {
	Runtime        string `json:"runtime"`
	RuntimeVersion string `json:"runtime_version"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	NumCPU         int    `json:"num_cpu"`
	NumGoroutine   int    `json:"num_goroutine"`
}

// ExceptionCapture holds captured exception data.
type ExceptionCapture struct {
	ID             string                 `json:"id"`
	ExceptionType  string                 `json:"exception_type"`
	Message        string                 `json:"message"`
	Fingerprint    string                 `json:"fingerprint"`
	StackTrace     []StackFrame           `json:"stack_trace"`
	LocalVariables map[string]Variable    `json:"local_variables"`
	Context        map[string]interface{} `json:"context"`
	CapturedAt     string                 `json:"captured_at"`
	AgentID        string                 `json:"agent_id"`
	Environment    string                 `json:"environment"`
	Runtime        string                 `json:"runtime"`
	RuntimeInfo    RuntimeInfo            `json:"runtime_info"`
}

// StackFrame represents a single frame in the stack trace.
type StackFrame struct {
	MethodName      string `json:"method_name"`
	FileName        string `json:"file_name,omitempty"`
	FilePath        string `json:"file_path,omitempty"`
	LineNumber      int    `json:"line_number,omitempty"`
	PackageName     string `json:"package_name,omitempty"`
	IsNative        bool   `json:"is_native"`
	SourceAvailable bool   `json:"source_available"`
}

// Variable represents a captured variable.
type Variable struct {
	Name          string              `json:"name"`
	Type          string              `json:"type"`
	Value         string              `json:"value"`
	IsNull        bool                `json:"is_null"`
	IsTruncated   bool                `json:"is_truncated"`
	Children      map[string]Variable `json:"children,omitempty"`
	ArrayElements []Variable          `json:"array_elements,omitempty"`
	ArrayLength   *int                `json:"array_length,omitempty"`
}

// CaptureError captures an error with stack trace and context.
func CaptureError(err error, maxDepth int, ctx map[string]interface{}) *ExceptionCapture {
	stackTrace := captureStackTrace(3) // Skip CaptureError, CaptureError, agent.CaptureError
	fingerprint := calculateFingerprint(err, stackTrace)

	context := make(map[string]interface{})
	if ctx != nil {
		for k, v := range ctx {
			context[k] = v
		}
	}

	// Capture local variables from context and error
	localVariables := make(map[string]Variable)

	// Capture context values as local variables
	for key, value := range ctx {
		localVariables[key] = captureValue(key, value, 0, maxDepth)
	}

	// Extract fields from the error if it's a struct
	extractErrorFields(err, localVariables, maxDepth)

	// Try to extract wrapped error chain
	extractWrappedErrors(err, localVariables, maxDepth)

	return &ExceptionCapture{
		ID:             uuid.New().String(),
		ExceptionType:  getErrorType(err),
		Message:        err.Error(),
		Fingerprint:    fingerprint,
		StackTrace:     stackTrace,
		LocalVariables: localVariables,
		Context:        context,
		CapturedAt:     time.Now().UTC().Format(time.RFC3339),
	}
}

// extractErrorFields extracts public fields from a custom error type.
func extractErrorFields(err error, vars map[string]Variable, maxDepth int) {
	v := reflect.ValueOf(err)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < t.NumField() && i < 50; i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldValue := v.Field(i)
		if !fieldValue.CanInterface() {
			continue
		}

		fieldName := "err." + field.Name
		vars[fieldName] = captureValue(fieldName, fieldValue.Interface(), 0, maxDepth)
	}
}

// extractWrappedErrors extracts information from wrapped errors.
func extractWrappedErrors(err error, vars map[string]Variable, maxDepth int) {
	// Check for Unwrap() error (Go 1.13+ wrapped errors)
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		if inner := unwrapper.Unwrap(); inner != nil {
			vars["wrapped_error"] = Variable{
				Name:  "wrapped_error",
				Type:  getErrorType(inner),
				Value: inner.Error(),
			}

			// Recursively extract from wrapped error
			extractErrorFields(inner, vars, maxDepth)
		}
	}

	// Check for Unwrap() []error (multi-error)
	if multiUnwrapper, ok := err.(interface{ Unwrap() []error }); ok {
		errors := multiUnwrapper.Unwrap()
		if len(errors) > 0 {
			elements := make([]Variable, 0, len(errors))
			for i, e := range errors {
				if i >= 10 { // Limit to 10 wrapped errors
					break
				}
				elements = append(elements, Variable{
					Name:  fmt.Sprintf("[%d]", i),
					Type:  getErrorType(e),
					Value: e.Error(),
				})
			}
			length := len(errors)
			vars["wrapped_errors"] = Variable{
				Name:          "wrapped_errors",
				Type:          "[]error",
				Value:         fmt.Sprintf("[%d errors]", length),
				ArrayElements: elements,
				ArrayLength:   &length,
			}
		}
	}

	// Check for Cause() error (pkg/errors style)
	if causer, ok := err.(interface{ Cause() error }); ok {
		if cause := causer.Cause(); cause != nil {
			vars["cause"] = Variable{
				Name:  "cause",
				Type:  getErrorType(cause),
				Value: cause.Error(),
			}
		}
	}
}

// CaptureValue captures an arbitrary value.
func CaptureValue(name string, value interface{}, maxDepth int) Variable {
	return captureValue(name, value, 0, maxDepth)
}

func captureStackTrace(skip int) []StackFrame {
	var frames []StackFrame
	pcs := make([]uintptr, 50)
	n := runtime.Callers(skip, pcs)
	pcs = pcs[:n]

	frameIter := runtime.CallersFrames(pcs)
	for {
		frame, more := frameIter.Next()
		if !more {
			break
		}

		// Skip runtime internals
		if strings.HasPrefix(frame.Function, "runtime.") {
			continue
		}

		f := StackFrame{
			MethodName:      extractFunctionName(frame.Function),
			FilePath:        frame.File,
			FileName:        extractFileName(frame.File),
			LineNumber:      frame.Line,
			PackageName:     extractPackageName(frame.Function),
			IsNative:        strings.HasPrefix(frame.File, "runtime/"),
			SourceAvailable: !strings.Contains(frame.File, "/pkg/mod/"),
		}

		frames = append(frames, f)

		if len(frames) >= 50 {
			break
		}
	}

	return frames
}

func captureValue(name string, value interface{}, depth, maxDepth int) Variable {
	if value == nil {
		return Variable{
			Name:   name,
			Type:   "nil",
			Value:  "nil",
			IsNull: true,
		}
	}

	if depth > maxDepth {
		return Variable{
			Name:        name,
			Type:        reflect.TypeOf(value).String(),
			Value:       "<max depth exceeded>",
			IsTruncated: true,
		}
	}

	v := reflect.ValueOf(value)
	t := v.Type()

	switch v.Kind() {
	case reflect.Invalid:
		return Variable{
			Name:   name,
			Type:   "invalid",
			Value:  "invalid",
			IsNull: true,
		}

	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return Variable{
			Name:  name,
			Type:  t.String(),
			Value: fmt.Sprintf("%v", value),
		}

	case reflect.String:
		s := v.String()
		truncated := len(s) > 1000
		if truncated {
			s = s[:1000]
		}
		return Variable{
			Name:        name,
			Type:        "string",
			Value:       s,
			IsTruncated: truncated,
		}

	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return Variable{
				Name:   name,
				Type:   t.String(),
				Value:  "nil",
				IsNull: true,
			}
		}
		return captureValue(name, v.Elem().Interface(), depth, maxDepth)

	case reflect.Slice, reflect.Array:
		length := v.Len()
		lenPtr := &length
		elements := []Variable{}

		maxElements := 100
		if length < maxElements {
			maxElements = length
		}

		for i := 0; i < maxElements; i++ {
			elem := captureValue(fmt.Sprintf("[%d]", i), v.Index(i).Interface(), depth+1, maxDepth)
			elements = append(elements, elem)
		}

		return Variable{
			Name:          name,
			Type:          t.String(),
			Value:         fmt.Sprintf("[%d items]", length),
			ArrayElements: elements,
			ArrayLength:   lenPtr,
			IsTruncated:   length > 100,
		}

	case reflect.Map:
		children := make(map[string]Variable)
		keys := v.MapKeys()

		maxKeys := 100
		if len(keys) < maxKeys {
			maxKeys = len(keys)
		}

		for i := 0; i < maxKeys; i++ {
			key := keys[i]
			keyStr := fmt.Sprintf("%v", key.Interface())
			val := v.MapIndex(key)
			children[keyStr] = captureValue(keyStr, val.Interface(), depth+1, maxDepth)
		}

		return Variable{
			Name:        name,
			Type:        t.String(),
			Value:       fmt.Sprintf("map[%d]", len(keys)),
			Children:    children,
			IsTruncated: len(keys) > 100,
		}

	case reflect.Struct:
		children := make(map[string]Variable)

		for i := 0; i < t.NumField() && i < 100; i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}

			fieldValue := v.Field(i)
			children[field.Name] = captureValue(field.Name, fieldValue.Interface(), depth+1, maxDepth)
		}

		return Variable{
			Name:     name,
			Type:     t.String(),
			Value:    fmt.Sprintf("<%s>", t.Name()),
			Children: children,
		}

	default:
		return Variable{
			Name:  name,
			Type:  t.String(),
			Value: fmt.Sprintf("<%s>", t.Kind()),
		}
	}
}

func calculateFingerprint(err error, stackTrace []StackFrame) string {
	parts := []string{getErrorType(err)}

	added := 0
	for _, frame := range stackTrace {
		if added >= 5 {
			break
		}
		if frame.IsNative {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d", frame.MethodName, frame.LineNumber))
		added++
	}

	hash := sha256.Sum256([]byte(strings.Join(parts, ":")))
	return hex.EncodeToString(hash[:8])
}

func getErrorType(err error) string {
	t := reflect.TypeOf(err)
	if t == nil {
		return "error"
	}
	if t.Kind() == reflect.Ptr {
		return t.Elem().String()
	}
	return t.String()
}

func extractFunctionName(fullName string) string {
	// Split by /
	parts := strings.Split(fullName, "/")
	if len(parts) > 0 {
		// Get last part and split by .
		last := parts[len(parts)-1]
		dotParts := strings.Split(last, ".")
		if len(dotParts) > 1 {
			return dotParts[len(dotParts)-1]
		}
		return last
	}
	return fullName
}

func extractPackageName(fullName string) string {
	lastSlash := strings.LastIndex(fullName, "/")
	if lastSlash >= 0 {
		fullName = fullName[lastSlash+1:]
	}
	firstDot := strings.Index(fullName, ".")
	if firstDot >= 0 {
		return fullName[:firstDot]
	}
	return fullName
}

func extractFileName(path string) string {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash >= 0 {
		return path[lastSlash+1:]
	}
	return path
}
