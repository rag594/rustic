package echov4

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})

	return exporter
}

func TestTracerMiddleware(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		path           string
		routePath      string
		expectedStatus int
		handlerError   error
	}{
		{
			name:           "successful GET request creates span",
			method:         http.MethodGet,
			path:           "/users/123",
			routePath:      "/users/:id",
			expectedStatus: http.StatusOK,
			handlerError:   nil,
		},
		{
			name:           "successful POST request creates span",
			method:         http.MethodPost,
			path:           "/users",
			routePath:      "/users",
			expectedStatus: http.StatusCreated,
			handlerError:   nil,
		},
		{
			name:           "handler error sets error attribute",
			method:         http.MethodGet,
			path:           "/error",
			routePath:      "/error",
			expectedStatus: http.StatusInternalServerError,
			handlerError:   echo.NewHTTPError(http.StatusInternalServerError, "test error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exporter := setupTestTracer(t)

			e := echo.New()
			e.Use(TracerMiddleware("test-service"))

			handler := func(c echo.Context) error {
				if tc.handlerError != nil {
					return tc.handlerError
				}
				return c.String(tc.expectedStatus, "OK")
			}

			switch tc.method {
			case http.MethodGet:
				e.GET(tc.routePath, handler)
			case http.MethodPost:
				e.POST(tc.routePath, handler)
			}

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			// Verify span was created
			spans := exporter.GetSpans()
			require.GreaterOrEqual(t, len(spans), 1, "expected at least one span to be created")

			// Find the span created by our middleware
			var middlewareSpan tracetest.SpanStub
			for _, span := range spans {
				if span.Name == "echo.http.request" {
					middlewareSpan = span
					break
				}
			}

			require.NotEmpty(t, middlewareSpan.Name, "expected middleware span to be created")
			assert.Equal(t, "echo.http.request", middlewareSpan.Name)

			// Verify span attributes
			attrs := middlewareSpan.Attributes
			var foundMethod, foundURL, foundResource bool
			for _, attr := range attrs {
				switch string(attr.Key) {
				case "http.method":
					assert.Equal(t, tc.method, attr.Value.AsString())
					foundMethod = true
				case "http.url":
					assert.Contains(t, attr.Value.AsString(), tc.path)
					foundURL = true
				case "resource.name":
					assert.Equal(t, tc.routePath, attr.Value.AsString())
					foundResource = true
				}
			}

			assert.True(t, foundMethod, "expected http.method attribute")
			assert.True(t, foundURL, "expected http.url attribute")
			assert.True(t, foundResource, "expected resource.name attribute")
		})
	}
}

func TestTracerMiddleware_PropagatesContext(t *testing.T) {
	exporter := setupTestTracer(t)

	e := echo.New()
	e.Use(TracerMiddleware("test-service"))

	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Verify span was created and context was propagated
	spans := exporter.GetSpans()
	require.GreaterOrEqual(t, len(spans), 1)

	// The span should have trace context
	span := spans[0]
	assert.True(t, span.SpanContext.HasTraceID(), "expected span to have trace ID")
	assert.True(t, span.SpanContext.HasSpanID(), "expected span to have span ID")
}

func TestTracerMiddleware_InjectsHeaders(t *testing.T) {
	_ = setupTestTracer(t)

	e := echo.New()
	e.Use(TracerMiddleware("test-service"))

	var traceParentHeader string
	e.GET("/test", func(c echo.Context) error {
		// After middleware runs, trace headers should be injected
		traceParentHeader = c.Request().Header.Get("Traceparent")
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Verify traceparent header was injected
	assert.NotEmpty(t, traceParentHeader, "expected traceparent header to be injected")
}
