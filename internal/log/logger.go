package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log levels
type Level string

const (
	DEBUG Level = "DEBUG"
	INFO  Level = "INFO"
	WARN  Level = "WARN"
	ERROR Level = "ERROR"
)

// Logger provides structured logging
type Logger struct {
	mu     sync.Mutex
	output io.Writer
	level  Level
}

// Entry represents a single log entry
type Entry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Init initializes the default logger
func Init(level Level) {
	once.Do(func() {
		defaultLogger = &Logger{
			output: os.Stderr,
			level:  level,
		}
	})
}

// SetLevel sets the log level
func SetLevel(level Level) {
	if defaultLogger != nil {
		defaultLogger.level = level
	}
}

// shouldLog checks if a message at the given level should be logged
func (l *Logger) shouldLog(level Level) bool {
	levels := map[Level]int{
		DEBUG: 0,
		INFO:  1,
		WARN:  2,
		ERROR: 3,
	}
	return levels[level] >= levels[l.level]
}

// log writes a log entry
func (l *Logger) log(level Level, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     string(level),
		Message:   message,
		Fields:    fields,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	fmt.Fprintf(l.output, "%s\n", data)
}

// Debug logs a debug message
func Debug(message string, fields ...map[string]interface{}) {
	if defaultLogger == nil {
		Init(DEBUG)
	}
	f := make(map[string]interface{})
	if len(fields) > 0 {
		f = fields[0]
	}
	defaultLogger.log(DEBUG, message, f)
}

// Info logs an info message
func Info(message string, fields ...map[string]interface{}) {
	if defaultLogger == nil {
		Init(INFO)
	}
	f := make(map[string]interface{})
	if len(fields) > 0 {
		f = fields[0]
	}
	defaultLogger.log(INFO, message, f)
}

// Warn logs a warning message
func Warn(message string, fields ...map[string]interface{}) {
	if defaultLogger == nil {
		Init(WARN)
	}
	f := make(map[string]interface{})
	if len(fields) > 0 {
		f = fields[0]
	}
	defaultLogger.log(WARN, message, f)
}

// Error logs an error message
func Error(message string, fields ...map[string]interface{}) {
	if defaultLogger == nil {
		Init(ERROR)
	}
	f := make(map[string]interface{})
	if len(fields) > 0 {
		f = fields[0]
	}
	defaultLogger.log(ERROR, message, f)
}
