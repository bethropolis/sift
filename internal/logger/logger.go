package logger

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// LogLevel defines log severity levels
type LogLevel int

const (
	// Log levels from least to most restrictive
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelNone
)

// Logger provides structured logging with levels
type Logger struct {
	mu          sync.Mutex
	out         io.Writer
	useColors   bool
	level       LogLevel
	VerboseMode bool // Legacy flag, maps to Debug level
}

// New creates a new Logger with the given settings
func New(out io.Writer, verbose bool, useColors bool) *Logger {
	level := LevelInfo
	if verbose {
		level = LevelDebug
	}

	return &Logger{
		out:         out,
		useColors:   useColors,
		level:       level,
		VerboseMode: verbose,
	}
}

// WithLevel sets the log level and returns the logger
func (l *Logger) WithLevel(level LogLevel) *Logger {
	l.level = level
	// Keep VerboseMode in sync for backward compatibility
	l.VerboseMode = (level <= LevelDebug)
	return l
}

// SetLevel sets the log level
func (l *Logger) SetLevel(levelStr string) {
	level := parseLogLevel(levelStr)
	l.WithLevel(level)
}

// parseLogLevel converts a string level to LogLevel
func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "none", "off":
		return LevelNone
	default:
		return LevelInfo // Default to Info level
	}
}

// Debug logs a debug message if verbose mode is enabled
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.write("DEBUG", color.CyanString, format, args...)
	}
}

// Info logs an informational message (standard level)
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.write("INFO", color.BlueString, format, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.write("WARN", color.YellowString, format, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= LevelError {
		l.write("ERROR", color.RedString, format, args...)
	}
}

// write emits a single formatted log line, serialized against concurrent writers.
func (l *Logger) write(level string, colorize func(string, ...interface{}) string, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	prefix := level
	if l.useColors {
		prefix = colorize(prefix)
	}
	fmt.Fprintf(l.out, "[%s %s] %s\n", timeString(), prefix, fmt.Sprintf(format, args...))
}

// timeString returns a formatted time string for the log prefix
func timeString() string {
	return time.Now().Format("15:04:05.000")
}
