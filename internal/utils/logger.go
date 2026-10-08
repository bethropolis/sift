// Package utils provides common utilities shared across packages
package utils

type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

type NoopLogger struct{}

func (l NoopLogger) Debug(format string, args ...interface{}) {}
func (l NoopLogger) Info(format string, args ...interface{})  {}
func (l NoopLogger) Warn(format string, args ...interface{})  {}
func (l NoopLogger) Error(format string, args ...interface{}) {}
