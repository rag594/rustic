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

// MetricType defines the type of an OpenTelemetry metric.
type MetricType string

const (
	// Int64Counter is a metric type for a counter that records int64 values.
	Int64Counter MetricType = "Int64Counter"
	// Float64Histogram is a metric type for a histogram that records float64 values.
	Float64Histogram MetricType = "Float64Histogram"
	// Add other types as needed, e.g., Int64UpDownCounter, Float64Gauge, etc.
)

// MetricConfig defines the configuration for an OpenTelemetry metric.
type MetricConfig struct {
	Name        string
	Description string
	Type        MetricType
	// Future additions could include Unit, specific advice for histogram boundaries, etc.
}

var defaultMetricConfigs = []MetricConfig{
	{
		Name:        "http.client.request.count",
		Description: "The total number of HTTP requests made by the client.",
		Type:        Int64Counter,
	},
	{
		Name:        "http.client.request.duration",
		Description: "The duration of HTTP requests made by the client, in seconds.",
		Type:        Float64Histogram,
	},
	{
		Name:        "http.client.request.errors",
		Description: "The number of HTTP requests by the client that resulted in an error (e.g., network errors).",
		Type:        Int64Counter,
	},
}

// HTTPClient wrapper over net/http client with tracing
type HTTPClient struct {
	Client           *http.Client
	TraceEnabled     bool
	MetricsEnabled   bool // Added MetricsEnabled field
	ServiceName      string
	meter            metric.Meter
	instruments      map[string]any // Stores initialized metric instruments
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
		httpClient.instruments = make(map[string]any)

		// Initialize instruments based on defaultMetricConfigs
		for _, config := range defaultMetricConfigs {
			switch config.Type {
			case Int64Counter:
				instrument := metric.Must(httpClient.meter).Int64Counter(
					config.Name,
					metric.WithDescription(config.Description),
				)
				httpClient.instruments[config.Name] = instrument
			case Float64Histogram:
				instrument := metric.Must(httpClient.meter).Float64Histogram(
					config.Name,
					metric.WithDescription(config.Description),
				)
				httpClient.instruments[config.Name] = instrument
				// Add cases for other metric types if they are defined in MetricType
			default:
				// Optionally log or handle unknown metric types
				// For now, we'll ignore unknown types to avoid panicking if defaultMetricConfigs is extended with unsupported types.
			}
		}
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

	// Record metrics if enabled and instruments are available
	if c.MetricsEnabled && c.instruments != nil && c.meter != nil {
		// Record duration
		if inst, ok := c.instruments["http.client.request.duration"]; ok {
			if histogram, typeOk := inst.(metric.Float64Histogram); typeOk {
				histogram.Record(request.Context(), duration, metric.WithAttributes(attributes...))
			}
		}

		// Record count
		if inst, ok := c.instruments["http.client.request.count"]; ok {
			if counter, typeOk := inst.(metric.Int64Counter); typeOk {
				counter.Add(request.Context(), 1, metric.WithAttributes(attributes...))
			}
		}

		if err != nil {
			// Record error count
			if inst, ok := c.instruments["http.client.request.errors"]; ok {
				if counter, typeOk := inst.(metric.Int64Counter); typeOk {
					// For errors, attributes should not include http.status_code if resp is nil
					errorAttributes := []attribute.KeyValue{
						attribute.String("http.method", request.Method),
						attribute.String("http.url", request.URL.String()),
					}
					// If resp is not nil (e.g. a 500 error from server still means Do itself didn't error yet),
					// we might want to include status code. However, this block is specifically for `err != nil`,
					// which implies a client-side error or an error before getting a full response.
					counter.Add(request.Context(), 1, metric.WithAttributes(errorAttributes...))
				}
			}
			return nil, err // Return error after attempting to record it
		}
	} else if err != nil { // If metrics are not enabled (or instruments map is nil), but there's an error, still return the error
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
