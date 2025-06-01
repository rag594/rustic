package httpClient

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel" // Added this import
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"net/http"
	"runtime"
	"time"
)

// HTTPClient wrapper over net/http client with tracing
type HTTPClient struct {
	Client           *http.Client
	TraceEnabled     bool
	MetricsEnabled   bool // Added MetricsEnabled field
	ServiceName      string
	meter            metric.Meter
	requestCount     metric.Int64Counter
	requestDuration  metric.Float64Histogram
	requestErrors    metric.Int64Counter
}

// HTTPClientOption different options to configure the HTTPClient
type HTTPClientOption func(client *HTTPClient)

// WithTraceEnabled allows to toggle tracing
func WithTraceEnabled(e bool) HTTPClientOption {
	return func(client *HTTPClient) {
		client.TraceEnabled = e
	}
}

// WithMetricsEnabled allows to toggle metrics
func WithMetricsEnabled(e bool) HTTPClientOption {
	return func(client *HTTPClient) {
		client.MetricsEnabled = e
	}
}

// NewHTTPClient creates a new HTTPClient with DefaultTransport
// TODO: add options to configure transport
func NewHTTPClient(opt ...HTTPClientOption) *HTTPClient {
	httpClient := HTTPClient{Client: &http.Client{}}
	for _, option := range opt {
		option(&httpClient)
	}

	if httpClient.TraceEnabled {
		httpClient.Client.Transport = otelhttp.NewTransport(http.DefaultTransport)
	} else {
		httpClient.Client.Transport = http.DefaultTransport.(*http.Transport)
	}

	if httpClient.MetricsEnabled {
		// Initialize meter
		httpClient.meter = otel.Meter("httpClient")

		// Define metrics
		httpClient.requestCount = metric.Must(httpClient.meter).Int64Counter("http.client.request.count")
		httpClient.requestDuration = metric.Must(httpClient.meter).Float64Histogram("http.client.request.duration")
		httpClient.requestErrors = metric.Must(httpClient.meter).Int64Counter("http.client.request.errors")
	}

	return &httpClient
}

// Do makes an HTTP request with the native `http.Do` interface
func (c *HTTPClient) Do(request *http.Request) (*http.Response, error) {
	startTime := time.Now()

	resp, err := c.Client.Do(request)

	duration := time.Since(startTime).Seconds()

	attributes := []attribute.KeyValue{
		attribute.String("http.method", request.Method),
		attribute.String("http.url", request.URL.String()),
	}

	if resp != nil {
		attributes = append(attributes, attribute.Int("http.status_code", resp.StatusCode))
	}

	if c.MetricsEnabled && c.meter != nil { // Check if metrics are enabled and meter is initialized
		if c.requestDuration != nil {
			c.requestDuration.Record(request.Context(), duration, metric.WithAttributes(attributes...))
		}
		if c.requestCount != nil {
			c.requestCount.Add(request.Context(), 1, metric.WithAttributes(attributes...))
		}

		if err != nil {
			if c.requestErrors != nil {
				// Create a separate attribute set for errors to avoid adding status_code if resp is nil
				errorAttributes := []attribute.KeyValue{
					attribute.String("http.method", request.Method),
					attribute.String("http.url", request.URL.String()),
				}
				c.requestErrors.Add(request.Context(), 1, metric.WithAttributes(errorAttributes...))
			}
			return nil, err // Return error after attempting to record it
		}
	} else if err != nil { // If metrics are not enabled, but there's an error, still return the error
		return nil, err
	}

	return resp, err
}

// GetCallerFunctionName for extracting name of next to next caller function
func GetCallerFunctionName() string {
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return runtime.FuncForPC(pc).Name()
}
