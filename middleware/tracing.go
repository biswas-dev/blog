package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TracingMiddleware instruments every HTTP request with an OpenTelemetry span.
//
// The APM backend is determined entirely by the global tracer set in apm.Init().
// To switch providers, change OTEL_EXPORTER_OTLP_ENDPOINT — no code changes here.
// When APM_ENABLED is not "true" the global tracer is a noop, making this
// middleware zero-overhead.
func TracingMiddleware() func(http.Handler) http.Handler {
	return otelhttp.NewMiddleware("blog.http")
}
