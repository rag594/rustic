package tracer

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	stdoutTrace "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	otelTracer "go.opentelemetry.io/otel/trace"
	"log"
)

func InitTracer(serviceName string) func() {
	// Create an OTLP exporter (send data to OpenTelemetry collector)
	_, err := otlptracehttp.New(context.Background(), otlptracehttp.WithInsecure(), otlptracehttp.WithEndpoint("localhost:4317"))
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	// Current using stdOut exporter
	stdOutExporter, err := stdoutTrace.New(stdoutTrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(stdOutExporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)

	// Return function to shut down the tracer
	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("failed to shutdown tracer: %v", err)
		}
	}
}

func GetTracer(serviceName string) otelTracer.Tracer {
	return otel.Tracer(serviceName)
}
