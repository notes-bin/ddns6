package log

import (
	"log"
	"os"
)

const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

type Logger struct {
	*log.Logger
	level int
}

func NewLogger(level int) *Logger {
	return &Logger{
		Logger: log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile),
		level:  level,
	}
}

func (l *Logger) Debug(v ...interface{}) {
	if l.level <= LevelDebug {
		l.Logger.Printf("[DEBUG] %v", v...)
	}
}

func (l *Logger) Info(v ...interface{}) {
	if l.level <= LevelInfo {
		l.Logger.Printf("[INFO] %v", v...)
	}
}

func (l *Logger) Warn(v ...interface{}) {
	if l.level <= LevelWarn {
		l.Logger.Printf("[WARN] %v", v...)
	}
}

func (l *Logger) Error(v ...interface{}) {
	if l.level <= LevelError {
		l.Logger.Printf("[ERROR] %v", v...)
	}
}

func (l *Logger) Fatal(v ...interface{}) {
	if l.level <= LevelFatal {
		l.Logger.Fatalf("[FATAL] %v", v...)
	}
}
