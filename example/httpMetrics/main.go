package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"learn-go-dependency-injection/httpClient"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func main() {
	// Initialize stdoutmetric exporter
	exporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to create stdoutmetric exporter: %v", err)
	}

	// Create a new MeterProvider with the exporter and set it as global
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.PeriodicReaderOption{
		Interval: 1 * time.Second, // Export metrics every second
	})))
	otel.SetMeterProvider(mp)

	// Ensure provider is shutdown at the end
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}()

	// Create an HTTPClient with metrics enabled
	client := httpClient.NewHTTPClient(
		httpClient.WithMetricsEnabled(true),
		httpClient.WithTraceEnabled(false), // Explicitly disable tracing if not needed for this example
	)

	log.Println("Making HTTP GET request to https://www.google.com...")
	// Create a new HTTP request
	req, err := http.NewRequestWithContext(context.Background(), "GET", "https://www.google.com", nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	// Make the HTTP request using the custom client
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("HTTP request failed: %v", err)
		// Note: The error metric should be recorded by the client.Do method
	} else {
		defer resp.Body.Close()
		log.Printf("Response Status: %s", resp.Status)
		// Request count and duration metrics should be recorded by client.Do
	}

	// Make another request to see more metrics
	req2, err := http.NewRequestWithContext(context.Background(), "GET", "https://www.example.com", nil)
	if err != nil {
		log.Fatalf("Failed to create request for example.com: %v", err)
	}
	resp2, err := client.Do(req2)
	if err != nil {
		log.Fatalf("HTTP request to example.com failed: %v", err)
	} else {
		defer resp2.Body.Close()
		log.Printf("Response Status (example.com): %s", resp2.Status)
	}

	// Make a request that should fail (non-existent domain)
	log.Println("Making HTTP GET request to https://nonexistentdomain.invalid...")
	reqFail, err := http.NewRequestWithContext(context.Background(), "GET", "https://nonexistentdomain.invalid", nil)
	if err != nil {
		log.Fatalf("Failed to create request for nonexistentdomain.invalid: %v", err)
	}
	_, err = client.Do(reqFail)
	if err != nil {
		log.Printf("HTTP request to nonexistentdomain.invalid failed as expected: %v", err)
	} else {
		log.Println("Request to nonexistentdomain.invalid succeeded unexpectedly")
	}


	// Keep the program running for a bit to allow metrics to be exported.
	// The stdout exporter typically exports periodically.
	log.Println("Waiting for metrics to be exported (approx 5 seconds)...")
	time.Sleep(5 * time.Second) // Adjust as needed for exporter to flush

	log.Println("Example finished.")
}
