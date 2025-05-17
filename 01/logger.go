package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Add new constants for the log directory and file format
const (
	defaultLogDir     = "logs"
	logFileTimeFormat = "2006-01-02_15-04-05"
	logFileNameFormat = "%s.log"
)

// LogLevel defines logging levels
type LogLevel int

const (
	INFO LogLevel = iota
	WARNING
	ERROR
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

// Logger contains necessary information for logging
type Logger struct {
	consoleOutput    bool
	fileOutput       bool
	logFile          *os.File
	logDir           string
	enableStackTrace bool
	stackTraceLevel  LogLevel
	stackTraceDepth  int
}

// LoggerConfig holds all logger configuration
type LoggerConfig struct {
	consoleOutput    bool
	fileOutput       bool
	logDir           string
	enableStackTrace bool
	stackTraceLevel  LogLevel
	stackTraceDepth  int
}

// LoggerOption defines a function type for setting logger options
type LoggerOption func(*LoggerConfig)

// WithConsoleOutput enables/disables console output
func WithConsoleOutput(enabled bool) LoggerOption {
	return func(c *LoggerConfig) {
		c.consoleOutput = enabled
	}
}

// WithFileOutput enables/disables file output
func WithFileOutput(enabled bool) LoggerOption {
	return func(c *LoggerConfig) {
		c.fileOutput = enabled
	}
}

// WithLogDirectory sets a custom directory for log files
func WithLogDirectory(dir string) LoggerOption {
	return func(c *LoggerConfig) {
		c.logDir = dir
	}
}

// WithStackTrace enables stack trace for logs at or above the specified level
func WithStackTrace(level LogLevel) LoggerOption {
	return func(c *LoggerConfig) {
		c.enableStackTrace = true
		c.stackTraceLevel = level
	}
}

// WithStackTraceDepth sets the maximum depth of the stack trace
func WithStackTraceDepth(depth int) LoggerOption {
	return func(c *LoggerConfig) {
		c.stackTraceDepth = depth
	}
}

// createLogFile creates a new log file with the timestamp
func (l *Logger) createLogFile() error {
	// Create a logs directory if it doesn't exist
	if err := os.MkdirAll(l.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format(logFileTimeFormat)
	filename := fmt.Sprintf(logFileNameFormat, timestamp)
	logPath := filepath.Join(l.logDir, filename)

	// Open the log file
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot create log file: %v", err)
	}

	l.logFile = file
	return nil
}

// NewLogger creates a new instance of Logger with the provided options
func NewLogger(options ...LoggerOption) (*Logger, error) {
	// Default configuration
	config := &LoggerConfig{
		consoleOutput:    true,
		fileOutput:       false,
		logDir:           defaultLogDir,
		enableStackTrace: false,
		stackTraceLevel:  ERROR,
		stackTraceDepth:  10,
	}

	// Apply all options
	for _, option := range options {
		option(config)
	}

	// Create logger instance
	logger := &Logger{
		consoleOutput:    config.consoleOutput,
		fileOutput:       config.fileOutput,
		logDir:           config.logDir,
		enableStackTrace: config.enableStackTrace,
		stackTraceLevel:  config.stackTraceLevel,
		stackTraceDepth:  config.stackTraceDepth,
	}

	// Create a log file if file output is enabled
	if config.fileOutput {
		if err := logger.createLogFile(); err != nil {
			return nil, err
		}
	}

	return logger, nil
}

// GetCurrentLogFile returns the path of the current log file
func (l *Logger) GetCurrentLogFile() string {
	if l.logFile == nil {
		return ""
	}
	return l.logFile.Name()
}

// RotateLogFile closes the current log file and creates a new one
func (l *Logger) RotateLogFile() error {
	// Check if the file output is enabled
	if !l.fileOutput {
		return fmt.Errorf("file output is not enabled")
	}

	// Close the existing file if it exists
	if l.logFile != nil {
		if err := l.logFile.Close(); err != nil {
			return fmt.Errorf("failed to close current log file: %v", err)
		}
	}

	// Create a new log file
	return l.createLogFile()
}

// getStackTrace returns the stack trace as a string
func (l *Logger) getStackTrace() string {
	var builder strings.Builder
	builder.WriteString("\nStack Trace:\n")

	// Skip 3 frames: getStackTrace, log, and the logging function (Info/Warning/Error)
	skip := 3
	for i := 0; i < l.stackTraceDepth; i++ {
		pc, file, line, ok := runtime.Caller(skip + i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			break
		}
		builder.WriteString(fmt.Sprintf("\t%s:%d - %s\n", filepath.Base(file), line, filepath.Base(fn.Name())))
	}
	return builder.String()
}

// log performs the actual logging operation
func (l *Logger) log(level LogLevel, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	location := getLocation()
	levelStr := getLevelStr(level)

	// Add stack trace if needed
	var stackTrace string
	if l.enableStackTrace && level >= l.stackTraceLevel {
		stackTrace = l.getStackTrace()
	}

	// Create a colored version for the console
	coloredLogMessage := fmt.Sprintf("%s[%s]%s %s - %s: %s%s",
		getLevelColor(level),
		levelStr,
		colorReset,
		timestamp,
		location,
		message,
		stackTrace,
	)

	// Create a plain version for the file
	plainLogMessage := fmt.Sprintf("[%s] %s - %s: %s%s",
		levelStr,
		timestamp,
		location,
		message,
		stackTrace,
	)

	if l.consoleOutput {
		fmt.Print(coloredLogMessage)
	}

	if l.fileOutput && l.logFile != nil {
		log.New(l.logFile, "", 0).Print(plainLogMessage)
	}
}

// Close closes the log file if it's being used
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// getLocation retrieves the caller's file location and line number
func getLocation() string {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown location"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

// getLevelColor returns the color code for the log level
func getLevelColor(level LogLevel) string {
	switch level {
	case WARNING:
		return colorYellow
	case ERROR:
		return colorRed
	default:
		return colorGreen
	}
}

// getLevelStr returns the string representation of the log level
func getLevelStr(level LogLevel) string {
	switch level {
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Info logs a message with the INFO level
func (l *Logger) Info(message string) {
	l.log(INFO, message)
}

// Warning logs a message with the WARNING level
func (l *Logger) Warning(message string) {
	l.log(WARNING, message)
}

// Error logs a message with ERROR level
func (l *Logger) Error(message string) {
	l.log(ERROR, message)
}
