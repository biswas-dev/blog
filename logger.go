package main

import (
	"os"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

var logger zerolog.Logger

// traceHook injects dd.trace_id, dd.span_id, and dd.service into every log
// event that carries an active OTel span, enabling log-trace correlation in
// Datadog Log Management.
//
// To attach a span to a log event, use logger.Ctx(ctx).Info().Msg("...").
// Without a context the hook is a no-op.
type traceHook struct{}

func (traceHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	span := trace.SpanFromContext(e.GetCtx())
	if !span.SpanContext().IsValid() {
		return
	}
	sc := span.SpanContext()

	// Datadog expects decimal 64-bit IDs under the dd.* keys.
	tid := sc.TraceID()
	var traceIDLow uint64
	for i := 8; i < 16; i++ {
		traceIDLow = traceIDLow<<8 | uint64(tid[i])
	}
	var spanIDInt uint64
	for _, b := range sc.SpanID() {
		spanIDInt = spanIDInt<<8 | uint64(b)
	}

	e.Uint64("dd.trace_id", traceIDLow).
		Uint64("dd.span_id", spanIDInt).
		Str("dd.service", "blog")
}

func initLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	logger = zerolog.New(os.Stderr).
		With().Timestamp().Logger().
		Hook(traceHook{})
	return logger
}
