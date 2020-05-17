package logger

// Logger represents an interface for a logger
type Logger interface {
	SetToDebug()
	SetToInfo()
	SetToError()
	Module(ns string) Logger
	Debug(msg string, keyValues ...interface{})
	Info(msg string, keyValues ...interface{})
	Error(msg string, keyValues ...interface{})
	Fatal(msg string, keyValues ...interface{})
	// Fatalf(msg string, keyValues ...interface{})
	Warn(msg string, keyValues ...interface{})
}
