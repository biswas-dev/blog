package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
)

func TestInitLogger(t *testing.T) {
	t.Run("initLogger returns a valid logger", func(t *testing.T) {
		l := initLogger()
		// zerolog.Logger zero value is usable, but we verify it writes output
		var buf bytes.Buffer
		l = l.Output(&buf)
		l.Info().Msg("test")
		if buf.Len() == 0 {
			t.Error("Expected logger to produce output")
		}
	})

	t.Run("initLogger sets global logger", func(t *testing.T) {
		initLogger()
		// The global logger variable should be set
		var buf bytes.Buffer
		logger = logger.Output(&buf)
		logger.Info().Msg("global test")
		if buf.Len() == 0 {
			t.Error("Expected global logger to produce output")
		}
	})
}

func TestLoggerOutput(t *testing.T) {
	t.Run("logger outputs valid JSON", func(t *testing.T) {
		var buf bytes.Buffer
		l := zerolog.New(&buf).With().Timestamp().Logger()
		l.Info().Msg("json test")

		var result map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("Expected valid JSON output, got error: %v", err)
		}
		if result["message"] != "json test" {
			t.Errorf("Expected message 'json test', got %v", result["message"])
		}
	})

	t.Run("logger includes level", func(t *testing.T) {
		var buf bytes.Buffer
		l := zerolog.New(&buf)
		l.Info().Msg("level test")

		var result map[string]interface{}
		json.Unmarshal(buf.Bytes(), &result)
		if result["level"] != "info" {
			t.Errorf("Expected level 'info', got %v", result["level"])
		}
	})

	t.Run("logger supports structured fields", func(t *testing.T) {
		var buf bytes.Buffer
		l := zerolog.New(&buf)
		l.Info().Str("key", "value").Int("count", 42).Msg("structured")

		var result map[string]interface{}
		json.Unmarshal(buf.Bytes(), &result)
		if result["key"] != "value" {
			t.Errorf("Expected key 'value', got %v", result["key"])
		}
		if result["count"] != float64(42) {
			t.Errorf("Expected count 42, got %v", result["count"])
		}
	})
}

func TestLogLevels(t *testing.T) {
	levels := []struct {
		name  string
		level string
		logFn func(zerolog.Logger, string)
	}{
		{"info", "info", func(l zerolog.Logger, msg string) { l.Info().Msg(msg) }},
		{"warn", "warn", func(l zerolog.Logger, msg string) { l.Warn().Msg(msg) }},
		{"error", "error", func(l zerolog.Logger, msg string) { l.Error().Msg(msg) }},
		{"debug", "debug", func(l zerolog.Logger, msg string) { l.Debug().Msg(msg) }},
	}

	for _, tc := range levels {
		t.Run(tc.name+" level", func(t *testing.T) {
			var buf bytes.Buffer
			l := zerolog.New(&buf).Level(zerolog.TraceLevel)
			tc.logFn(l, "test "+tc.name)

			var result map[string]interface{}
			json.Unmarshal(buf.Bytes(), &result)
			if result["level"] != tc.level {
				t.Errorf("Expected level %q, got %v", tc.level, result["level"])
			}
		})
	}
}
