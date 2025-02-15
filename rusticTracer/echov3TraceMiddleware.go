package rusticTracer

import (
	echov3 "github.com/labstack/echo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	otelTracer "go.opentelemetry.io/otel/trace"
)

// Echov3TracerMiddleware extracts and injects the trace for incoming HTTP requests to be propagated forward
func Echov3TracerMiddleware(service string) echov3.MiddlewareFunc {
	return func(next echov3.HandlerFunc) echov3.HandlerFunc {
		return func(c echov3.Context) error {
			// Get global tracer and propagator
			tr := otel.Tracer(service)
			propagator := otel.GetTextMapPropagator()

			// Extract the context from incoming request headers
			ctx := propagator.Extract(c.Request().Context(), propagation.HeaderCarrier(c.Request().Header))

			// Start a new span with span name "echo.http.request"
			ctx, span := tr.Start(ctx, "echo.http.request", otelTracer.WithSpanKind(otelTracer.SpanKindServer))
			defer span.End()

			// Set span attributes
			span.SetAttributes(
				attribute.String("http.method", c.Request().Method),
				attribute.String("http.url", c.Request().URL.String()),
				attribute.String("resource.name", c.Path()), // Echo route path
			)

			// Inject updated trace context into request headers for downstream services
			propagator.Inject(ctx, propagation.HeaderCarrier(c.Request().Header))

			// Attach the updated context to Echo's request
			c.SetRequest(c.Request().WithContext(ctx))

			if err := next(c); err != nil {
				span.SetAttributes(
					attribute.String("error.error", c.Request().Method))
				c.Error(err)
			}

			return nil
		}
	}
}
