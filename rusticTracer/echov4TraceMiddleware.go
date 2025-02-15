package rusticTracer

import (
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	otelTracer "go.opentelemetry.io/otel/trace"
)

// Echov4TracerMiddleware extracts and injects the trace for incoming HTTP requests to be propagated forward
func Echov4TracerMiddleware(service string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
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
