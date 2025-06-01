package httpClient

import (
	"context"
	"fmt"
	"io"
	"learn-go-dependency-injection/httpClient"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metrictest" // Hope this path is correct for SDK v1.34.0
)

// Helper function to setup an in-memory meter provider for tests
func setupTestMetrics(t *testing.T) (*metrictest.MeterProvider, *httpClient.HTTPClient) {
	mp := metrictest.NewMeterProvider()
	otel.SetMeterProvider(mp) // Set as global for the test

	// It's important that the httpClient uses the meter from this provider.
	// Since the client gets the global provider, this setup should work.
	client := httpClient.NewHTTPClient(
		httpClient.WithMetricsEnabled(true),
	)
	return mp, client
}

// Helper function to setup an in-memory meter provider for tests where client is configured later
func setupTestMeterProvider(t *testing.T) *metrictest.MeterProvider {
	mp := metrictest.NewMeterProvider()
	otel.SetMeterProvider(mp) // Set as global for the test
	return mp
}

func TestHTTPClient_Metrics_Enabled_SuccessfulRequest(t *testing.T) {
	mp, client := setupTestMetrics(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Hello, client")
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	// Force a collection
	rm, err := mp.Collect(context.Background())
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatalf("Expected scope metrics, got none")
	}
	// Assuming one scope "httpClient"
	if len(rm.ScopeMetrics[0].Metrics) < 2 {
		t.Fatalf("Expected at least 2 metrics (count, duration), got %d", len(rm.ScopeMetrics[0].Metrics))
	}

	var foundCount, foundDuration bool
	for _, m := range rm.ScopeMetrics[0].Metrics {
		switch m.Name {
		case "http.client.request.count":
			foundCount = true
			data, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric http.client.request.count is not a Sum[int64]: %T", m.Data)
			}
			if len(data.DataPoints) != 1 {
				t.Fatalf("Expected 1 data point for count, got %d", len(data.DataPoints))
			}
			dp := data.DataPoints[0]
			if dp.Value != 1 {
				t.Errorf("Expected count to be 1, got %d", dp.Value)
			}
			// Check attributes
			expectedAttrs := attribute.NewSet(
				attribute.String("http.method", "GET"),
				attribute.String("http.url", server.URL),
				attribute.Int("http.status_code", http.StatusOK),
			)
			if !expectedAttrs.Equals(&dp.Attributes) {
				t.Errorf("Expected attributes %v, got %v", expectedAttrs, dp.Attributes)
			}
		case "http.client.request.duration":
			foundDuration = true
			data, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("metric http.client.request.duration is not a Histogram[float64]: %T", m.Data)
			}
			if len(data.DataPoints) != 1 {
				t.Fatalf("Expected 1 data point for duration, got %d", len(data.DataPoints))
			}
			dp := data.DataPoints[0]
			if dp.Count != 1 { // Check if a recording was made
				t.Errorf("Expected duration to have 1 event, got %d", dp.Count)
			}
			// Check attributes
			expectedAttrs := attribute.NewSet(
				attribute.String("http.method", "GET"),
				attribute.String("http.url", server.URL),
				attribute.Int("http.status_code", http.StatusOK),
			)
			if !expectedAttrs.Equals(&dp.Attributes) {
				t.Errorf("Expected attributes %v, got %v", expectedAttrs, dp.Attributes)
			}
		}
	}
	if !foundCount {
		t.Error("Metric http.client.request.count not found")
	}
	if !foundDuration {
		t.Error("Metric http.client.request.duration not found")
	}
}

func TestHTTPClient_Metrics_Enabled_ErrorRequest(t *testing.T) {
	mp, client := setupTestMetrics(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, _ := http.NewRequest("POST", server.URL+"/path", nil) // Different method and path
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Expected no error from Do itself (server error), got %v", err)
	}
	defer resp.Body.Close()


	// Second request - this one will be a client-side error (network error)
	reqFail, _ := http.NewRequest("GET", "http://localhost:0", nil) // Non-routable, should cause conn refused
	_, err = client.Do(reqFail)
	if err == nil {
		t.Fatalf("Expected error for request to non-existent server, got nil")
	}


	rm, err := mp.Collect(context.Background())
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatalf("Expected scope metrics, got none")
	}

	var foundErrorCount, foundRequestCount, foundDurationCount int
	var totalRequests int64 = 0
	var totalErrors int64 = 0

	for _, m := range rm.ScopeMetrics[0].Metrics {
		switch m.Name {
		case "http.client.request.count":
			data, _ := m.Data.(metricdata.Sum[int64])
			for _, dp := range data.DataPoints {
				totalRequests += dp.Value
				foundRequestCount++
			}
		case "http.client.request.duration":
			data, _ := m.Data.(metricdata.Histogram[float64])
			for _, dp := range data.DataPoints {
				foundDurationCount += int(dp.Count)
			}
		case "http.client.request.errors":
			data, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric http.client.request.errors is not a Sum[int64]: %T", m.Data)
			}
			if len(data.DataPoints) == 0 {
				// It's possible no errors were recorded if the filter for attributes is too specific
				// Or if the error happened before attribute creation
				t.Logf("No data points found for http.client.request.errors. Data: %+v", data)
			}
			for _, dp := range data.DataPoints {
				totalErrors += dp.Value
				foundErrorCount++
				// Check attributes for the client-side error
				// The server error (500) is not an "error" for this counter, it's a successful request that got a 500
				// This counter is for client-side errors or if Do returns an error.
				expectedAttrs := attribute.NewSet(
					attribute.String("http.method", "GET"),
					attribute.String("http.url", "http://localhost:0"),
					// No status code for client-side errors
				)
				// Note: The order of requests matters here. This check is brittle if other errors are also recorded.
				// We expect one error data point for the client-side error.
				if !expectedAttrs.Equals(&dp.Attributes) {
					t.Errorf("Expected error attributes %v, got %v", expectedAttrs, dp.Attributes)
				}
			}
		}
	}

	if totalRequests != 2 {
		t.Errorf("Expected total request count to be 2, got %d (found %d DPs)", totalRequests, foundRequestCount)
	}
	if foundDurationCount != 2 { // One for 500, one for client error (duration still recorded)
		t.Errorf("Expected total duration events to be 2, got %d", foundDurationCount)
	}
	if totalErrors != 1 { // Only the client-side error
		t.Errorf("Expected total error count to be 1, got %d (found %d DPs)", totalErrors, foundErrorCount)
	}
	if foundErrorCount == 0 && totalErrors == 0 { // Be more explicit if no error DPs found
		t.Error("No data points found for http.client.request.errors metric, but expected 1")
	}
}


func TestHTTPClient_Metrics_Disabled(t *testing.T) {
	mp := setupTestMeterProvider(t) // just need the meter provider to check it's empty

	client := httpClient.NewHTTPClient(
		httpClient.WithMetricsEnabled(false), // Metrics disabled
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	// Try a failing request too
	reqFail, _ := http.NewRequest("GET", "http://localhost:0", nil)
	_, err = client.Do(reqFail)
	if err == nil {
		t.Fatalf("Expected error for request to non-existent server, got nil")
	}

	rm, err := mp.Collect(context.Background())
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) != 0 {
		// Log the unexpected metrics for debugging
		for _, sm := range rm.ScopeMetrics {
			t.Logf("Unexpected scope metric: %s", sm.Scope.Name)
			for _, m := range sm.Metrics {
				t.Logf("  Unexpected metric: %s", m.Name)
			}
		}
		t.Errorf("Expected no scope metrics when MetricsEnabled is false, got %d", len(rm.ScopeMetrics))
	}
}

// Test for correct attributes, especially for status codes and errors
func TestHTTPClient_Metrics_Attributes(t *testing.T) {
	mp, client := setupTestMetrics(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/ok") {
			w.WriteHeader(http.StatusOK)
		} else if strings.Contains(r.URL.Path, "/notfound") {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Request 1: OK
	reqOK, _ := http.NewRequest("GET", server.URL+"/ok", nil)
	respOK, _ := client.Do(reqOK)
	defer respOK.Body.Close()

	// Request 2: NotFound
	reqNotFound, _ := http.NewRequest("PUT", server.URL+"/notfound", nil)
	respNotFound, _ := client.Do(reqNotFound)
	defer respNotFound.Body.Close()

	// Request 3: Client-side error
	reqClientError, _ := http.NewRequest("POST", "http://badhost:12345", nil)
	_, errClient := client.Do(reqClientError)
	if errClient == nil {
		t.Fatal("Expected client error, got none")
	}

	rm, err := mp.Collect(context.Background())
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 || len(rm.ScopeMetrics[0].Metrics) == 0 {
		t.Fatal("No metrics collected")
	}

	metrics := rm.ScopeMetrics[0].Metrics
	expectedDataPoints := 3 // total requests

	var countDps, durationDps, errorDps int
	for _, m := range metrics {
		switch m.Name {
		case "http.client.request.count":
			sum, _ := m.Data.(metricdata.Sum[int64])
			countDps = len(sum.DataPoints)
			for _, dp := range sum.DataPoints {
				url, _ := dp.Attributes.Value("http.url")
				method, _ := dp.Attributes.Value("http.method")
				statusCode, statusOk := dp.Attributes.Value("http.status_code")

				if url.AsString() == server.URL+"/ok" {
					if method.AsString() != "GET" || !statusOk || statusCode.AsInt64() != http.StatusOK {
						t.Errorf("Incorrect attributes for /ok count: %+v", dp.Attributes)
					}
				} else if url.AsString() == server.URL+"/notfound" {
					if method.AsString() != "PUT" || !statusOk || statusCode.AsInt64() != http.StatusNotFound {
						t.Errorf("Incorrect attributes for /notfound count: %+v", dp.Attributes)
					}
				} else if url.AsString() == "http://badhost:12345" {
					// For client error, status code attribute should not be present on count if Do returns error before response.
					// However, our current implementation adds it if resp is nil, which might be an issue.
					// Let's assume current behavior: status code is not present for client error if resp is nil.
					// Actually, client.Do adds status code if resp is not nil.
					// For the client error (badhost), resp will be nil. So status_code should NOT be present.
					if method.AsString() != "POST" || statusOk { // statusOk should be false
						t.Errorf("Incorrect attributes for client error count: %+v. StatusOK: %v", dp.Attributes, statusOk)
					}
				}
			}
		case "http.client.request.duration":
			hist, _ := m.Data.(metricdata.Histogram[float64])
			durationDps = len(hist.DataPoints)
			// Similar checks for duration attributes
			for _, dp := range hist.DataPoints {
				url, _ := dp.Attributes.Value("http.url")
				method, _ := dp.Attributes.Value("http.method")
				statusCode, statusOk := dp.Attributes.Value("http.status_code")

				if url.AsString() == server.URL+"/ok" {
					if method.AsString() != "GET" || !statusOk || statusCode.AsInt64() != http.StatusOK {
						t.Errorf("Incorrect attributes for /ok duration: %+v", dp.Attributes)
					}
				} else if url.AsString() == server.URL+"/notfound" {
					if method.AsString() != "PUT" || !statusOk || statusCode.AsInt64() != http.StatusNotFound {
						t.Errorf("Incorrect attributes for /notfound duration: %+v", dp.Attributes)
					}
				} else if url.AsString() == "http://badhost:12345" {
					if method.AsString() != "POST" || statusOk {
						t.Errorf("Incorrect attributes for client error duration: %+v. StatusOK: %v", dp.Attributes, statusOk)
					}
				}
			}
		case "http.client.request.errors":
			sum, _ := m.Data.(metricdata.Sum[int64])
			errorDps = len(sum.DataPoints)
			if errorDps != 1 {
				t.Errorf("Expected 1 error data point, got %d", errorDps)
			}
			for _, dp := range sum.DataPoints {
				url, _ := dp.Attributes.Value("http.url")
				method, _ := dp.Attributes.Value("http.method")
				_, statusOk := dp.Attributes.Value("http.status_code") // Should not be present

				if !(url.AsString() == "http://badhost:12345" && method.AsString() == "POST" && !statusOk) {
					t.Errorf("Incorrect attributes for errors: %+v", dp.Attributes)
				}
			}
		}
	}

	if countDps != expectedDataPoints {
		t.Errorf("Expected %d data points for request_count, got %d", expectedDataPoints, countDps)
	}
	if durationDps != expectedDataPoints {
		t.Errorf("Expected %d data points for request_duration, got %d", expectedDataPoints, durationDps)
	}
	// errorDps already checked
}

// Ensure httpClient uses the globally set MeterProvider if not overridden.
// This test relies on the global meter provider being set.
func TestHTTPClient_UsesGlobalMeterProvider(t *testing.T) {
	mp := metrictest.NewMeterProvider()
	otel.SetMeterProvider(mp) // Set as global

	// Create client AFTER setting global provider
	client := httpClient.NewHTTPClient(httpClient.WithMetricsEnabled(true))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	rm, err := mp.Collect(context.Background())
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("No metrics collected, client might not be using the global meter provider")
	}
	if len(rm.ScopeMetrics[0].Metrics) == 0 {
		t.Fatal("No metrics from the httpClient scope, client might not be using the global meter provider")
	}
	// Basic check for count metric
	found := false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "http.client.request.count" {
			found = true
			break
		}
	}
	if !found {
		t.Error("http.client.request.count not found, client may not be using the set meter provider")
	}
}
