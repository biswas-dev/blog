package main

import (
	"testing"

	"go.uber.org/zap"
)

func TestSugarLog(t *testing.T) {
	t.Run("sugarLog returns non-nil logger", func(t *testing.T) {
		// Note: This test will create a production logger
		// In a real scenario, you might want to use a test logger
		logger := sugarLog()

		if logger == nil {
			t.Error("Expected non-nil logger")
		}
	})

	t.Run("logger type is SugaredLogger", func(t *testing.T) {
		logger := sugarLog()

		// Verify it's the right type
		var _ *zap.SugaredLogger = logger
	})
}

// Test logger creation patterns
func TestLoggerCreation(t *testing.T) {
	t.Run("can create production logger", func(t *testing.T) {
		logger, err := zap.NewProduction()
		if err != nil {
			t.Errorf("Failed to create production logger: %v", err)
		}
		if logger == nil {
			t.Error("Expected non-nil logger")
		}

		defer logger.Sync()
	})

	t.Run("can create sugared logger", func(t *testing.T) {
		logger, err := zap.NewProduction()
		if err != nil {
			t.Fatalf("Failed to create production logger: %v", err)
		}

		sugar := logger.Sugar()
		if sugar == nil {
			t.Error("Expected non-nil sugared logger")
		}

		defer logger.Sync()
	})

	t.Run("can create development logger", func(t *testing.T) {
		logger, err := zap.NewDevelopment()
		if err != nil {
			t.Errorf("Failed to create development logger: %v", err)
		}
		if logger == nil {
			t.Error("Expected non-nil logger")
		}

		defer logger.Sync()
	})
}

// Test logger methods exist
func TestLoggerMethods(t *testing.T) {
	t.Run("sugared logger has Info method", func(t *testing.T) {
		logger, _ := zap.NewProduction()
		sugar := logger.Sugar()
		defer logger.Sync()

		// This would normally log, but in tests we just verify the method exists
		sugar.Infof("test message: %s", "info")
	})

	t.Run("sugared logger has Error method", func(t *testing.T) {
		logger, _ := zap.NewProduction()
		sugar := logger.Sugar()
		defer logger.Sync()

		sugar.Errorf("test error: %s", "error")
	})

	t.Run("sugared logger has Debug method", func(t *testing.T) {
		logger, _ := zap.NewProduction()
		sugar := logger.Sugar()
		defer logger.Sync()

		sugar.Debugf("test debug: %s", "debug")
	})

	t.Run("sugared logger has Warn method", func(t *testing.T) {
		logger, _ := zap.NewProduction()
		sugar := logger.Sugar()
		defer logger.Sync()

		sugar.Warnf("test warning: %s", "warn")
	})
}

// Test logger configuration
func TestLoggerConfiguration(t *testing.T) {
	t.Run("production logger config", func(t *testing.T) {
		config := zap.NewProductionConfig()

		if config.Level.Level() != zap.InfoLevel {
			t.Error("Expected production logger to use Info level")
		}

		if config.Encoding != "json" {
			t.Error("Expected production logger to use JSON encoding")
		}
	})

	t.Run("development logger config", func(t *testing.T) {
		config := zap.NewDevelopmentConfig()

		if config.Level.Level() != zap.DebugLevel {
			t.Error("Expected development logger to use Debug level")
		}

		if config.Encoding != "console" {
			t.Error("Expected development logger to use console encoding")
		}
	})
}

// Test logger sync
func TestLoggerSync(t *testing.T) {
	t.Run("logger sync doesn't panic", func(t *testing.T) {
		logger, err := zap.NewProduction()
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Sync shouldn't panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("logger.Sync() panicked: %v", r)
			}
		}()

		logger.Sync()
	})
}

// Test logger with various message types
func TestLoggerMessageTypes(t *testing.T) {
	logger, _ := zap.NewProduction()
	sugar := logger.Sugar()
	defer logger.Sync()

	t.Run("log string message", func(t *testing.T) {
		sugar.Info("simple string message")
	})

	t.Run("log formatted message", func(t *testing.T) {
		sugar.Infof("formatted message: %d, %s", 123, "test")
	})

	t.Run("log with key-value pairs", func(t *testing.T) {
		sugar.Infow("message with fields",
			"key1", "value1",
			"key2", 42,
		)
	})
}

// Test logger error handling
func TestLoggerErrorHandling(t *testing.T) {
	t.Run("logger handles nil values", func(t *testing.T) {
		logger, _ := zap.NewProduction()
		sugar := logger.Sugar()
		defer logger.Sync()

		// Should not panic with nil values
		var nilValue *string
		sugar.Infow("message with nil",
			"nilField", nilValue,
		)
	})

	t.Run("logger handles empty strings", func(t *testing.T) {
		logger, _ := zap.NewProduction()
		sugar := logger.Sugar()
		defer logger.Sync()

		sugar.Info("")
	})
}
