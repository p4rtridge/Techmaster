package logger

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"
)

// TestNewLogger tests the creation of new logger instances
func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		options []LoggerOption
		wantErr bool
	}{
		{
			name: "Console output only (default)",
			options: []LoggerOption{
				WithConsoleOutput(true),
			},
			wantErr: false,
		},
		{
			name: "File output only",
			options: []LoggerOption{
				WithConsoleOutput(false),
				WithFileOutput(true),
			},
			wantErr: false,
		},
		{
			name: "Both outputs",
			options: []LoggerOption{
				WithConsoleOutput(true),
				WithFileOutput(true),
			},
			wantErr: false,
		},
		{
			name: "Custom configuration",
			options: []LoggerOption{
				WithConsoleOutput(true),
				WithFileOutput(true),
				WithStackTrace(WARNING),
				WithStackTraceDepth(15),
				WithLogDirectory("test_logs"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.options...)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewLogger() error = %v", err)
				return
			}

			if logger == nil {
				t.Error("Expected logger instance, got nil")
				return
			}

			// Cleanup
			if logger.logFile != nil {
				logger.Close()
				os.RemoveAll(logger.logDir)
			}
		})
	}
}

// TestLogLevels tests different logging levels
func TestLogLevels(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(
		WithConsoleOutput(false),
		WithFileOutput(true),
		WithLogDirectory(tempDir),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		logger.Close()
		os.RemoveAll(tempDir)
	}()

	tests := []struct {
		level      LogLevel
		levelStr   string
		logMessage string
	}{
		{INFO, "INFO", "Test info message"},
		{WARNING, "WARNING", "Test warning message"},
		{ERROR, "ERROR", "Test error message"},
	}

	for _, tt := range tests {
		t.Run(tt.levelStr, func(t *testing.T) {
			switch tt.level {
			case INFO:
				logger.Info(tt.logMessage)
			case WARNING:
				logger.Warning(tt.logMessage)
			case ERROR:
				logger.Error(tt.logMessage)
			}

			// Read the last line from the log file
			content, err := os.ReadFile(logger.GetCurrentLogFile())
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			lastLine := getLastLine(string(content))

			// Check if the log contains the correct level and message
			if !strings.Contains(lastLine, "["+tt.levelStr+"]") {
				t.Errorf("Log level not found, got: %s, want: %s", lastLine, tt.levelStr)
			}
			if !strings.Contains(lastLine, tt.logMessage) {
				t.Errorf("Log message not found, got: %s, want: %s", lastLine, tt.logMessage)
			}
		})
	}
}

// TestConsoleOutput tests the console output functionality
func TestConsoleOutput(t *testing.T) {
	// Redirect stdout to capture console output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger, err := NewLogger(
		WithConsoleOutput(true),
		WithFileOutput(false),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	testMessage := "Test console output"
	logger.Info(testMessage)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var output strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n == 0 || err != nil {
			break
		}
		output.Write(buf[:n])
	}

	if !strings.Contains(output.String(), testMessage) {
		t.Errorf("Console output doesn't contain test message.\nGot: %s\nWant: %s",
			output.String(), testMessage)
	}
}

// TestStackTrace tests the stack trace functionality
func TestStackTrace(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(
		WithFileOutput(true),
		WithLogDirectory(tempDir),
		WithStackTrace(WARNING),
		WithStackTraceDepth(5),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		logger.Close()
		os.RemoveAll(tempDir)
	}()

	logger.Warning("Test stack trace")

	content, err := os.ReadFile(logger.GetCurrentLogFile())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Stack Trace:") {
		t.Error("Stack trace not found in log output")
	}
}

func TestStackTraceOptional(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name             string
		options          []LoggerOption
		expectStackTrace bool
	}{
		{
			name: "Without stack trace",
			options: []LoggerOption{
				WithFileOutput(true),
				WithLogDirectory(tempDir),
			},
			expectStackTrace: false,
		},
		{
			name: "With stack trace",
			options: []LoggerOption{
				WithFileOutput(true),
				WithLogDirectory(tempDir),
				WithStackTrace(WARNING),
			},
			expectStackTrace: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.options...)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			defer func() {
				logger.Close()
				os.RemoveAll(tempDir)
			}()

			logger.Warning("Test message")

			content, err := os.ReadFile(logger.GetCurrentLogFile())
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			hasStackTrace := strings.Contains(string(content), "Stack Trace:")
			if hasStackTrace != tt.expectStackTrace {
				t.Errorf("Stack trace presence = %v, want %v", hasStackTrace, tt.expectStackTrace)
			}
		})
	}
}

// TestRotateLogFile tests log file rotation
func TestRotateLogFile(t *testing.T) {
	tests := []struct {
		name      string
		options   []LoggerOption
		wantError bool
	}{
		{
			name: "Successful rotation",
			options: []LoggerOption{
				WithFileOutput(true),
				WithLogDirectory(t.TempDir()),
			},
			wantError: false,
		},
		{
			name: "Rotation with file output disabled",
			options: []LoggerOption{
				WithFileOutput(false),
				WithLogDirectory(t.TempDir()),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.options...)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			defer func() {
				logger.Close()
				if logger.logDir != "" {
					os.RemoveAll(logger.logDir)
				}
			}()

			if !tt.wantError {
				// Test successful rotation
				firstFile := logger.GetCurrentLogFile()
				if firstFile == "" {
					t.Fatal("Expected a log file to be created")
				}
				logger.Info("First log")

				// Wait for 1 second to ensure different timestamp in the filename
				time.Sleep(time.Second)

				err = logger.RotateLogFile()
				if err != nil {
					t.Fatalf("Failed to rotate log file: %v", err)
				}

				secondFile := logger.GetCurrentLogFile()
				if secondFile == "" {
					t.Fatal("Expected a new log file to be created after rotation")
				}
				logger.Info("Second log")

				if firstFile == secondFile {
					t.Errorf("Log file rotation did not create a new file:\nFirst: %s\nSecond: %s",
						firstFile, secondFile)
				}

				// Check both files exist and have content
				firstContent, err := os.ReadFile(firstFile)
				if err != nil {
					t.Errorf("Cannot read first log file: %v", err)
				}
				if !strings.Contains(string(firstContent), "First log") {
					t.Error("First log file doesn't contain expected content")
				}

				secondContent, err := os.ReadFile(secondFile)
				if err != nil {
					t.Errorf("Cannot read second log file: %v", err)
				}
				if !strings.Contains(string(secondContent), "Second log") {
					t.Error("Second log file doesn't contain expected content")
				}
			} else {
				// Test rotation failure
				err = logger.RotateLogFile()
				if err == nil {
					t.Error("Expected error when rotating log file with file output disabled")
				}
			}
		})
	}
}

// Helper function to get the last line of a string
func getLastLine(s string) string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	return lastLine
}
