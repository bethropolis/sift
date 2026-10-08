package logger

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

type LogLevel int

const (
	// Log levels from least to most restrictive
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelNone
)

type Logger struct {
	mu          sync.Mutex
	out         io.Writer
	useColors   bool
	level       LogLevel
	VerboseMode bool // Legacy flag, maps to Debug level
}

// New creates a new Logger with the given settings. Normal CLI operation is
// warning-oriented; verbose mode enables debug and info diagnostics. Callers
// can explicitly request info with SetLevel("info").
func New(out io.Writer, verbose bool, useColors bool) *Logger {
	level := LevelWarn
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

func (l *Logger) WithLevel(level LogLevel) *Logger {
	l.level = level
	// Keep VerboseMode in sync for backward compatibility
	l.VerboseMode = (level <= LevelDebug)
	return l
}

func (l *Logger) SetLevel(levelStr string) {
	level := parseLogLevel(levelStr)
	l.WithLevel(level)
}

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

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.write("DEBUG", color.CyanString, format, args...)
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.write("INFO", color.BlueString, format, args...)
	}
}

func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.write("WARN", color.YellowString, format, args...)
	}
}

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

func timeString() string {
	return time.Now().Format("15:04:05.000")
}
