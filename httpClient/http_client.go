package httpClient

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"
	"runtime"
)

// HTTPClient wrapper over net/http client with tracing
type HTTPClient struct {
	Client       *http.Client
	TraceEnabled bool
	ServiceName  string
}

// HTTPClientOption different options to configure the HTTPClient
type HTTPClientOption func(client *HTTPClient)

// WithTraceEnabled allows to toggle tracing
func WithTraceEnabled(e bool) HTTPClientOption {
	return func(client *HTTPClient) {
		client.TraceEnabled = e
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

	return &httpClient
}

// Do makes an HTTP request with the native `http.Do` interface
func (c *HTTPClient) Do(request *http.Request) (*http.Response, error) {
	resp, err := c.Client.Do(request)
	if err != nil {
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
