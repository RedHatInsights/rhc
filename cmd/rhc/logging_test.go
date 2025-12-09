package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewPIDHandler verifies that NewPIDHandler creates a handler with proper defaults
func TestNewPIDHandler(t *testing.T) {
	tests := []struct {
		name          string
		opts          *PIDHandlerOptions
		expectedLevel slog.Level
	}{
		{
			name:          "nil_options_defaults_to_info",
			opts:          nil,
			expectedLevel: slog.LevelInfo,
		},
		{
			name: "custom_level_debug",
			opts: &PIDHandlerOptions{
				Level: slog.LevelDebug,
			},
			expectedLevel: slog.LevelDebug,
		},
		{
			name: "custom_level_error",
			opts: &PIDHandlerOptions{
				Level: slog.LevelError,
			},
			expectedLevel: slog.LevelError,
		},
		{
			name:          "nil_level_in_options_defaults_to_info",
			opts:          &PIDHandlerOptions{},
			expectedLevel: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewPIDHandler(buf, tt.opts)

			if handler == nil {
				t.Fatal("expected non-nil handler")
			}

			if handler.out != buf {
				t.Error("handler output writer not set correctly")
			}

			if handler.mu == nil {
				t.Error("expected non-nil mutex")
			}

			if handler.opts.Level.Level() != tt.expectedLevel {
				t.Errorf("expected level %v, got %v", tt.expectedLevel, handler.opts.Level.Level())
			}
		})
	}
}

// TestPIDHandlerEnabled verifies level filtering
func TestPIDHandlerEnabled(t *testing.T) {
	tests := []struct {
		name          string
		handlerLevel  slog.Level
		checkLevel    slog.Level
		expectEnabled bool
	}{
		{
			name:          "info_handler_debug_log_disabled",
			handlerLevel:  slog.LevelInfo,
			checkLevel:    slog.LevelDebug,
			expectEnabled: false,
		},
		{
			name:          "info_handler_info_log_enabled",
			handlerLevel:  slog.LevelInfo,
			checkLevel:    slog.LevelInfo,
			expectEnabled: true,
		},
		{
			name:          "info_handler_warn_log_enabled",
			handlerLevel:  slog.LevelInfo,
			checkLevel:    slog.LevelWarn,
			expectEnabled: true,
		},
		{
			name:          "error_handler_warn_log_disabled",
			handlerLevel:  slog.LevelError,
			checkLevel:    slog.LevelWarn,
			expectEnabled: false,
		},
		{
			name:          "debug_handler_debug_log_enabled",
			handlerLevel:  slog.LevelDebug,
			checkLevel:    slog.LevelDebug,
			expectEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewPIDHandler(buf, &PIDHandlerOptions{
				Level: tt.handlerLevel,
			})

			enabled := handler.Enabled(context.Background(), tt.checkLevel)

			if enabled != tt.expectEnabled {
				t.Errorf("expected enabled=%v, got %v", tt.expectEnabled, enabled)
			}
		})
	}
}

// TestPIDHandlerBasicLogging verifies basic log output format
func TestPIDHandlerBasicLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	testTime := time.Date(2024, 12, 10, 15, 30, 45, 0, time.UTC)
	record := slog.NewRecord(testTime, slog.LevelInfo, "test message", 0)

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify PID is included
	if !strings.Contains(output, "PID=") {
		t.Error("output missing PID prefix")
	}

	// Verify level
	if !strings.Contains(output, "INFO") {
		t.Error("output missing log level")
	}

	// Verify message
	if !strings.Contains(output, "test message") {
		t.Error("output missing log message")
	}

	// Verify timestamp format (RFC3339)
	if !strings.Contains(output, "2024-12-10T15:30:45Z") {
		t.Errorf("output missing or incorrect timestamp, got: %s", output)
	}

	// Verify newline termination
	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}

	// Test that the logger can be used normally
	buf.Reset()
	logger.Info("another test")
	if buf.Len() == 0 {
		t.Error("expected logger output")
	}
}

// TestPIDHandlerWithAttributes verifies attribute logging
func TestPIDHandlerWithAttributes(t *testing.T) {
	tests := []struct {
		name             string
		logFunc          func(*slog.Logger)
		expectedIncludes []string
	}{
		{
			name: "single_string_attribute",
			logFunc: func(l *slog.Logger) {
				l.Info("test", "key", "value")
			},
			expectedIncludes: []string{"key=\"value\""},
		},
		{
			name: "multiple_attributes",
			logFunc: func(l *slog.Logger) {
				l.Info("test", "key1", "value1", "key2", "value2")
			},
			expectedIncludes: []string{"key1=\"value1\"", "key2=\"value2\""},
		},
		{
			name: "integer_attribute",
			logFunc: func(l *slog.Logger) {
				l.Info("test", "count", 42)
			},
			expectedIncludes: []string{"count=42"},
		},
		{
			name: "boolean_attribute",
			logFunc: func(l *slog.Logger) {
				l.Info("test", "enabled", true)
			},
			expectedIncludes: []string{"enabled=true"},
		},
		{
			name: "mixed_types",
			logFunc: func(l *slog.Logger) {
				l.Info("test", "str", "hello", "num", 123, "bool", false)
			},
			expectedIncludes: []string{"str=\"hello\"", "num=123", "bool=false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewPIDHandler(buf, &PIDHandlerOptions{
				Level: slog.LevelInfo,
			})
			logger := slog.New(handler)

			tt.logFunc(logger)

			output := buf.String()

			for _, expected := range tt.expectedIncludes {
				if !strings.Contains(output, expected) {
					t.Errorf("output missing expected string %q, got: %s", expected, output)
				}
			}
		})
	}
}

// TestPIDHandlerWithGroup verifies grouped attribute logging
func TestPIDHandlerWithGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	// Test WithGroup
	groupLogger := logger.WithGroup("request")
	groupLogger.Info("test", "method", "GET", "path", "/api")

	output := buf.String()

	// Verify grouped attributes have the group prefix
	if !strings.Contains(output, "request.method=\"GET\"") {
		t.Errorf("output missing grouped attribute request.method, got: %s", output)
	}
	if !strings.Contains(output, "request.path=\"/api\"") {
		t.Errorf("output missing grouped attribute request.path, got: %s", output)
	}

	// Test nested groups
	buf.Reset()
	nestedLogger := logger.WithGroup("outer").WithGroup("inner")
	nestedLogger.Info("test", "key", "value")

	output = buf.String()
	if !strings.Contains(output, "outer.inner.key=\"value\"") {
		t.Errorf("output missing nested group prefix, got: %s", output)
	}
}

// TestPIDHandlerWithAttrs verifies WithAttrs functionality
func TestPIDHandlerWithAttrs(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	// Create logger with pre-attached attributes
	contextLogger := logger.With("request_id", "12345", "user", "alice")
	contextLogger.Info("operation completed", "status", "success")

	output := buf.String()

	// Verify both pre-attached and log-time attributes are present
	if !strings.Contains(output, "request_id=\"12345\"") {
		t.Errorf("output missing WithAttrs attribute, got: %s", output)
	}
	if !strings.Contains(output, "user=\"alice\"") {
		t.Errorf("output missing WithAttrs attribute, got: %s", output)
	}
	if !strings.Contains(output, "status=\"success\"") {
		t.Errorf("output missing log-time attribute, got: %s", output)
	}
}

// TestPIDHandlerGroupWithAttrs verifies combining WithGroup and WithAttrs
func TestPIDHandlerGroupWithAttrs(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	// Combine WithAttrs and WithGroup
	contextLogger := logger.With("global", "value").WithGroup("scoped")
	contextLogger.Info("test", "local", "data")

	output := buf.String()

	// Global attribute should not be grouped
	if !strings.Contains(output, "global=\"value\"") {
		t.Errorf("output missing global attribute, got: %s", output)
	}

	// Local attribute should be in the group
	if !strings.Contains(output, "scoped.local=\"data\"") {
		t.Errorf("output missing grouped attribute, got: %s", output)
	}
}

// TestPIDHandlerGroupedAttributes verifies slog.Group attribute handling
func TestPIDHandlerGroupedAttributes(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	// Use slog.Group to create inline grouped attributes
	logger.Info("test",
		slog.Group("user",
			slog.String("name", "alice"),
			slog.Int("age", 30),
		),
	)

	output := buf.String()

	if !strings.Contains(output, "user.name=\"alice\"") {
		t.Errorf("output missing grouped attribute user.name, got: %s", output)
	}
	if !strings.Contains(output, "user.age=30") {
		t.Errorf("output missing grouped attribute user.age, got: %s", output)
	}
}

// TestPIDHandlerTimeAttribute verifies time attribute formatting
func TestPIDHandlerTimeAttribute(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	testTime := time.Date(2024, 12, 10, 15, 30, 45, 123456789, time.UTC)
	logger.Info("test", "timestamp", testTime)

	output := buf.String()

	// Time attributes should be formatted as RFC3339Nano
	if !strings.Contains(output, "timestamp=2024-12-10T15:30:45.123456789Z") {
		t.Errorf("output missing or incorrect time attribute, got: %s", output)
	}
}

// TestPIDHandlerEmptyGroups verifies that empty groups are omitted
func TestPIDHandlerEmptyGroups(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	// Create logger with group but don't add any attributes
	groupLogger := logger.WithGroup("empty")
	groupLogger.Info("test message")

	output := buf.String()

	// Empty group prefix should not appear
	if strings.Contains(output, "empty.") {
		t.Errorf("output contains empty group prefix, got: %s", output)
	}

	// Message should still be logged
	if !strings.Contains(output, "test message") {
		t.Errorf("output missing message, got: %s", output)
	}
}

// TestPIDHandlerEmptyAttributes verifies that empty attributes are ignored
func TestPIDHandlerEmptyAttributes(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	// Create a record and manually add an empty attribute
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.Attr{}) // Empty attribute

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should only contain the basic log structure
	if !strings.Contains(output, "test") {
		t.Error("output missing message")
	}

	// Should not have any key=value pairs (empty attr should be ignored)
	lines := strings.Split(strings.TrimSpace(output), " ")
	for _, line := range lines {
		if strings.Contains(line, "=") && !strings.Contains(line, "PID=") {
			t.Errorf("output contains unexpected attribute: %s", line)
		}
	}
}

// TestPIDHandlerConcurrency verifies thread safety
func TestPIDHandlerConcurrency(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	const numGoroutines = 100
	const logsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines that log concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				logger.Info("concurrent log", "goroutine", id, "iteration", j)
			}
		}(i)
	}

	wg.Wait()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have exactly the expected number of log lines
	expectedLines := numGoroutines * logsPerGoroutine
	if len(lines) != expectedLines {
		t.Errorf("expected %d log lines, got %d", expectedLines, len(lines))
	}

	// Each line should be well-formed (contain PID and message)
	for i, line := range lines {
		if !strings.Contains(line, "PID=") {
			t.Errorf("line %d missing PID: %s", i, line)
		}
		if !strings.Contains(line, "concurrent log") {
			t.Errorf("line %d missing message: %s", i, line)
		}
	}
}

// TestPIDHandlerDifferentLevels verifies logging at different levels
func TestPIDHandlerDifferentLevels(t *testing.T) {
	tests := []struct {
		name         string
		handlerLevel slog.Level
		logLevel     slog.Level
		logMessage   string
		shouldAppear bool
	}{
		{
			name:         "debug_logged_with_debug_handler",
			handlerLevel: slog.LevelDebug,
			logLevel:     slog.LevelDebug,
			logMessage:   "debug message",
			shouldAppear: true,
		},
		{
			name:         "debug_filtered_with_info_handler",
			handlerLevel: slog.LevelInfo,
			logLevel:     slog.LevelDebug,
			logMessage:   "debug message",
			shouldAppear: false,
		},
		{
			name:         "error_logged_with_info_handler",
			handlerLevel: slog.LevelInfo,
			logLevel:     slog.LevelError,
			logMessage:   "error message",
			shouldAppear: true,
		},
		{
			name:         "warn_filtered_with_error_handler",
			handlerLevel: slog.LevelError,
			logLevel:     slog.LevelWarn,
			logMessage:   "warn message",
			shouldAppear: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewPIDHandler(buf, &PIDHandlerOptions{
				Level: tt.handlerLevel,
			})

			logger := slog.New(handler)

			// Log at the specified level
			logger.Log(context.Background(), tt.logLevel, tt.logMessage)

			output := buf.String()

			if tt.shouldAppear {
				if !strings.Contains(output, tt.logMessage) {
					t.Errorf("expected message to appear in output, got: %s", output)
				}
			} else {
				if strings.Contains(output, tt.logMessage) {
					t.Errorf("expected message to be filtered, but it appeared: %s", output)
				}
			}
		})
	}
}

// TestPIDHandlerWithGroupEmptyName verifies that empty group names are ignored
func TestPIDHandlerWithGroupEmptyName(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	// WithGroup with empty string should return the same handler
	emptyGroupLogger := logger.WithGroup("")
	emptyGroupLogger.Info("test", "key", "value")

	output := buf.String()

	// Attribute should not have a group prefix
	if strings.Contains(output, ".key=") {
		t.Errorf("empty group should not add prefix, got: %s", output)
	}
	if !strings.Contains(output, "key=\"value\"") {
		t.Errorf("output missing attribute, got: %s", output)
	}
}

// TestPIDHandlerWithAttrsEmpty verifies that empty attrs slice is handled
func TestPIDHandlerWithAttrsEmpty(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	// WithAttrs with empty slice should return the same handler
	emptyAttrsLogger := logger.With()
	emptyAttrsLogger.Info("test message")

	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Errorf("output missing message, got: %s", output)
	}
}

// TestPIDHandlerRecordWithZeroTime verifies handling of zero time
func TestPIDHandlerRecordWithZeroTime(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewPIDHandler(buf, &PIDHandlerOptions{
		Level: slog.LevelInfo,
	})

	// Create record with zero time
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test message", 0)

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should still contain PID and message
	if !strings.Contains(output, "PID=") {
		t.Error("output missing PID")
	}
	if !strings.Contains(output, "test message") {
		t.Error("output missing message")
	}

	// Time should not be in the output if it's zero
	// The output should start with PID= if time is zero
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "PID=") {
		t.Errorf("expected output to start with PID= when time is zero, got: %s", output)
	}
}
