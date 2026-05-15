// Package logger menyediakan structured logging menggunakan zerolog
// Support console output (dengan warna) dan file output (plain text)
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

var (
	// Log adalah global logger instance untuk dipakai di seluruh aplikasi
	Log zerolog.Logger
)

// Config untuk konfigurasi logger
type Config struct {
	Level         string // Minimum log level: debug, info, warn, error
	LogDir        string // Directory untuk log files
	LogFileName   string // Nama file log
	ConsoleOutput bool   // Enable output ke console (stdout)
	FileOutput    bool   // Enable output ke file
}

// DefaultConfig return konfigurasi default logger
func DefaultConfig() Config {
	return Config{
		Level:         "info",
		LogDir:        "logs",
		LogFileName:   "app.log",
		ConsoleOutput: true,
		FileOutput:    true,
	}
}

// Init initialize global logger dengan configuration yang diberikan
// Support multi-writer: console (dengan warna) dan file (plain text)
func Init(cfg Config) error {
	var writers []io.Writer

	// Setup console output dengan warna untuk development
	if cfg.ConsoleOutput {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
			// Custom format untuk level dengan warna
			FormatLevel: func(i any) string {
				return colorLevel(fmt.Sprintf("%s", i))
			},
			FormatMessage: func(i any) string {
				return fmt.Sprintf("│ %s", i)
			},
			FormatFieldName: func(i any) string {
				return fmt.Sprintf("%s=", i)
			},
			FormatFieldValue: func(i any) string {
				return fmt.Sprintf("%s", i)
			},
			// Format caller info agar clickable di VS Code terminal
			// Output: filename.go:123 >
			FormatCaller: func(i any) string {
				if i == nil || i == "" {
					return ""
				}
				return fmt.Sprintf("%s >", i)
			},
		}
		writers = append(writers, consoleWriter)
	}

	// Setup file output untuk production/archiving
	if cfg.FileOutput {
		// Create log directory jika belum ada
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// Create atau open log file (append mode)
		logFilePath := filepath.Join(cfg.LogDir, cfg.LogFileName)
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		// File writer tanpa warna (plain text untuk parsing/searching)
		fileWriter := zerolog.ConsoleWriter{
			Out:        logFile,
			NoColor:    true,
			TimeFormat: time.RFC3339,
		}
		writers = append(writers, fileWriter)
	}

	// Create multi-writer untuk output simultan ke console & file
	multi := io.MultiWriter(writers...)

	// Set global log level dari config
	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	// Custom caller format: Short relative path untuk VS Code clickability
	// Format: internal/handler/file.go:123 (tanpa project name prefix)
	// Lebih ringkas tapi tetap clickable!
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		// Find project root dan skip project name
		projectName := "aira-auth-service/"
		if idx := findProjectRoot(file, projectName); idx >= 0 {
			// Skip project name, ambil sisanya
			relativePath := file[idx+len(projectName):]
			return fmt.Sprintf("%s:%d", relativePath, line)
		}
		// Fallback: basename jika project name tidak ditemukan
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	// Create logger global dengan timestamp
	Log = zerolog.New(multi).With().Timestamp().Logger()

	return nil
}

// parseLevel konversi string level ke zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// colorLevel return colored level string untuk console output
func colorLevel(level string) string {
	switch level {
	case "debug":
		return "\033[36mDBG\033[0m" // Cyan
	case "info":
		return "\033[32mINF\033[0m" // Green
	case "warn":
		return "\033[33mWRN\033[0m" // Yellow
	case "error":
		return "\033[31mERR\033[0m" // Red
	case "fatal":
		return "\033[35mFTL\033[0m" // Magenta
	default:
		return level
	}
}

// findProjectRoot mencari index dari project name di file path
// Returns index of project name, or -1 if not found
func findProjectRoot(file, projectName string) int {
	// Replace backslashes with forward slashes for consistency
	file = filepath.ToSlash(file)
	
	// Find last occurrence of project name
	idx := -1
	for i := len(file) - len(projectName); i >= 0; i-- {
		if i+len(projectName) <= len(file) && file[i:i+len(projectName)] == projectName {
			idx = i
			break
		}
	}
	return idx
}

// ===== Convenience Methods =====
// Wrapper functions untuk quick logging dengan caller info
// Usage: logger.Info().Str("key", "value").Msg("message")

// Debug log dengan level debug (untuk development/troubleshooting)
func Debug() *zerolog.Event {
	return Log.Debug().Caller(1)
}

// Info log dengan level info (events normal/penting)
func Info() *zerolog.Event {
	return Log.Info().Caller(1)
}

// Warn log dengan level warning (potential issues)
func Warn() *zerolog.Event {
	return Log.Warn().Caller(1)
}

// Error log dengan level error (errors yang di-handle)
func Error() *zerolog.Event {
	return Log.Error().Caller(1)
}

// Fatal log dengan level fatal (exit aplikasi setelah log)
func Fatal() *zerolog.Event {
	return Log.Fatal().Caller(1)
}

// ===== Context Utilities =====

// GetLoggerFromGinContext retrieves the logger instance from Gin context
// Logger ini sudah include request ID built-in dari RequestIDMiddleware
//
// Usage di handler:
//   log := logger.GetLoggerFromGinContext(c)
//   log.Error().Str("user_id", userID).Msg("Failed to login")
//
// Jika logger tidak ditemukan di context (tidak seharusnya terjadi),
// akan return logger global fallback
func GetLoggerFromGinContext(c interface{ Get(any) (any, bool) }) *zerolog.Logger {
	if logger, exists := c.Get("logger"); exists {
		if log, ok := logger.(*zerolog.Logger); ok {
			return log
		}
	}
	// Fallback ke logger global jika tidak ada di context
	// (shouldn't happen jika RequestIDMiddleware sudah terpasang)
	return &Log
}
