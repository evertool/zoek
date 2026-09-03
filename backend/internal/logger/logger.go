package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps a zap logger with a simplified interface.
type Logger struct {
	zap *zap.Logger
}

// New creates a Logger from config parameters.
func New(level, encoding, output string) (*Logger, error) {
	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	if encoding == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Output writer
	var writer zapcore.WriteSyncer
	switch output {
	case "stdout", "":
		writer = zapcore.Lock(os.Stdout)
	case "stderr":
		writer = zapcore.Lock(os.Stderr)
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file %s: %w", output, err)
		}
		writer = zapcore.AddSync(f)
	}

	core := zapcore.NewCore(encoder, writer, zapLevel)
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0))

	return &Logger{zap: zapLogger}, nil
}

// NewNop returns a no-op logger for testing.
func NewNop() *Logger {
	return &Logger{zap: zap.NewNop()}
}

// Info logs an info message with optional structured fields.
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zap.Info(msg, fields...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.zap.Warn(msg, fields...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.zap.Error(msg, fields...)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zap.Debug(msg, fields...)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.zap.Fatal(msg, fields...)
}

// With returns a child logger with the given fields attached.
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{zap: l.zap.With(fields...)}
}

// String, Int64, Error helpers — thin wrappers over zap for convenience.
func String(key, val string) zap.Field          { return zap.String(key, val) }
func Int64(key string, val int64) zap.Field     { return zap.Int64(key, val) }
func Int(key string, val int) zap.Field         { return zap.Int(key, val) }
func ErrorField(err error) zap.Field            { return zap.Error(err) }
func Any(key string, val interface{}) zap.Field { return zap.Any(key, val) }

// Sync flushes buffered log entries. Should be called on shutdown.
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// GORMLogLevel converts a string log level to gorm's logger.LogLevel.
// Used by the store layer to configure GORM logging.
func GORMLogLevel(level string) int {
	switch strings.ToLower(level) {
	case "silent":
		return 0 // Silent
	case "error":
		return 1 // Error
	case "warn":
		return 2 // Warn
	case "info":
		return 3 // Info
	default:
		return 2 // Warn
	}
}

// gormWriter implements gorm/logger.Writer by forwarding log messages to the
// zap logger. GORM's logger.Writer interface requires a Printf method.
type gormWriter struct {
	log *Logger
}

// Printf implements gorm/logger.Writer. It forwards the formatted message to
// the underlying zap logger at Warn level.
func (g *gormWriter) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	g.log.Warn("gorm", String("sql", msg))
}

// NewGORMWriter returns a gorm/logger.Writer-compatible adapter.
func (l *Logger) NewGORMWriter() interface {
	Printf(format string, args ...interface{})
} {
	return &gormWriter{log: l}
}
