package rusticTracer

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	stdoutTrace "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	otelTracer "go.opentelemetry.io/otel/trace"
	"log"
)

// StdOutExporter outputs the traces to the stdout
func StdOutExporter() *stdoutTrace.Exporter {
	// Currently using stdOut exporter
	stdOutExporter, err := stdoutTrace.New(stdoutTrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	return stdOutExporter
}

// OTLPExporter Uses OpenTelemetry’s standard OTLP/gRPC or HTTP with host/port
func OTLPExporter(host, port string) *otlptrace.Exporter {
	// Create an OTLP exporter (send data to OpenTelemetry collector)
	oltpExporter, err := otlptracehttp.New(context.Background(), otlptracehttp.WithInsecure(), otlptracehttp.WithEndpoint(fmt.Sprintf("%s:%s", host, port)))
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	return oltpExporter
}

// InitTracer initialises the otel tracer for a serviceName and env with exporter of choice
func InitTracer(serviceName, env string, exporter trace.SpanExporter) func() {
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			semconv.DeploymentEnvironmentNameKey.String(env),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return function to shut down the tracer
	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("failed to shutdown tracer: %v", err)
		}
	}
}

// GetTracer returns the global tracer initialised for the serviceName
func GetTracer(serviceName string) otelTracer.Tracer {
	return otel.Tracer(serviceName)
}
