package rustic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	netUrl "net/url"
	"os"
	"strings"
	"time"

	"github.com/rag594/rustic/httpClient"
	"github.com/rag594/rustic/rusticTracer"
	"github.com/sony/gobreaker/v2"
)

// HTTPConfig different http configurations
type HTTPConfig struct {
	HttpClient          *httpClient.HTTPClient
	Timeout             time.Duration
	Headers             http.Header
	QueryParams         netUrl.Values
	FormParams          netUrl.Values
	MultipartFormParams map[string]string
	CircuitBreaker      *gobreaker.CircuitBreaker[any] // currently only github.com/sony/gobreaker/v2 is supported
}

type HTTPConfigOptions func(*HTTPConfig)

func WithHttpClient(c *httpClient.HTTPClient) HTTPConfigOptions {
	return func(p *HTTPConfig) {
		p.HttpClient = c
	}
}

func WithTimeout(t time.Duration) HTTPConfigOptions {
	return func(p *HTTPConfig) {
		p.Timeout = t
	}
}

func WithHeaders(c http.Header) HTTPConfigOptions {
	return func(p *HTTPConfig) {
		p.Headers = c
	}
}

func WithQueryParams(c netUrl.Values) HTTPConfigOptions {
	return func(p *HTTPConfig) {
		p.QueryParams = c
	}
}

func WithFormParams(c netUrl.Values) HTTPConfigOptions {
	return func(p *HTTPConfig) {
		p.FormParams = c
	}
}

func WithMultiPartFormParams(c map[string]string) HTTPConfigOptions {
	return func(p *HTTPConfig) {
		p.MultipartFormParams = c
	}
}

func WithCircuitBreaker(c *gobreaker.CircuitBreaker[any]) HTTPConfigOptions {
	return func(config *HTTPConfig) {
		config.CircuitBreaker = c
	}
}

// setupContext prepares the context with timeout and tracing
func setupContext(ctx context.Context, config *HTTPConfig) (context.Context, func()) {
	if ctx == nil {
		ctx = context.Background()
	}

	var cancel context.CancelFunc
	if config.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
	} else {
		cancel = func() {}
	}

	if config.HttpClient.TraceEnabled {
		tr := rusticTracer.GetTracer(config.HttpClient.ServiceName)
		ctx, span := tr.Start(ctx, httpClient.GetCallerFunctionName())
		return ctx, func() {
			span.End()
			cancel()
		}
	}

	return ctx, cancel
}

// applyHeaders applies headers to the request
func applyHeaders(req *http.Request, headers http.Header) {
	for key, values := range headers {
		if len(values) > 0 && len(values[0]) > 0 {
			req.Header.Set(key, values[0])
		}
	}
}

// handleResponse processes the HTTP response
func handleResponse[Res any](resp *http.Response, err error) (*Res, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result Res
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return nil, &httpClient.HTTPError{
		StatusCode: resp.StatusCode,
		Status:     http.StatusText(resp.StatusCode),
		Body:       string(body),
	}
}

// executeRequest executes the HTTP request with circuit breaker if configured
func executeRequest[Res any](client *httpClient.HTTPClient, req *http.Request, breaker *gobreaker.CircuitBreaker[any]) (*Res, error) {
	if breaker != nil {
		result, err := breaker.Execute(func() (any, error) {
			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			return handleResponse[Res](resp, nil)
		})
		if err != nil {
			return nil, err
		}
		return result.(*Res), nil
	}

	resp, err := client.Do(req)
	return handleResponse[Res](resp, err)
}

// createRequest creates an HTTP request with the given method and body
func createRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", method, err)
	}
	return req, nil
}

// GET http method with Res as response type
func GET[Res any](ctx context.Context, url string, opts ...HTTPConfigOptions) (*Res, error) {
	config := &HTTPConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.Headers == nil {
		config.Headers = http.Header{}
	}
	config.Headers.Set("Content-Type", "application/json")

	ctx, cancel := setupContext(ctx, config)
	defer cancel()

	parsedURL, err := netUrl.Parse(url)
	if err != nil {
		log.Fatal(err)
	}

	if len(config.QueryParams) != 0 {
		parsedURL.RawQuery = config.QueryParams.Encode()
	}

	req, err := createRequest(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}

	applyHeaders(req, config.Headers)
	return executeRequest[Res](config.HttpClient, req, config.CircuitBreaker)
}

// POST http method with Req as request type and Res as response type
func POST[Req, Res any](ctx context.Context, url string, req *Req, opts ...HTTPConfigOptions) (*Res, error) {
	config := &HTTPConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.Headers == nil {
		config.Headers = http.Header{}
	}
	config.Headers.Set("Content-Type", "application/json")

	ctx, cancel := setupContext(ctx, config)
	defer cancel()

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	request, err := createRequest(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	applyHeaders(request, config.Headers)
	return executeRequest[Res](config.HttpClient, request, config.CircuitBreaker)
}

// PUT http method with Req as request type and Res as response type
func PUT[Req, Res any](ctx context.Context, url string, req *Req, opts ...HTTPConfigOptions) (*Res, error) {
	config := &HTTPConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.Headers == nil {
		config.Headers = http.Header{}
	}
	config.Headers.Set("Content-Type", "application/json")

	ctx, cancel := setupContext(ctx, config)
	defer cancel()

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	request, err := createRequest(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	applyHeaders(request, config.Headers)
	return executeRequest[Res](config.HttpClient, request, config.CircuitBreaker)
}

// POSTFormData with Res as response type and allows application/x-www-form-urlencoded -> formData
func POSTFormData[Res any](ctx context.Context, url string, opts ...HTTPConfigOptions) (*Res, error) {
	config := &HTTPConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.Headers == nil {
		config.Headers = http.Header{}
	}
	config.Headers.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx, cancel := setupContext(ctx, config)
	defer cancel()

	request, err := createRequest(ctx, http.MethodPost, url, strings.NewReader(config.FormParams.Encode()))
	if err != nil {
		return nil, err
	}

	applyHeaders(request, config.Headers)
	return executeRequest[Res](config.HttpClient, request, config.CircuitBreaker)
}

// POSTMultiPartFormData with Res as response type, map of files with key as fieldName and value as filePath
func POSTMultiPartFormData[Res any](ctx context.Context, url string, files map[string]string, opts ...HTTPConfigOptions) (*Res, error) {
	config := &HTTPConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.Headers != nil {
		config.Headers = make(http.Header)
	}

	ctx, cancel := setupContext(ctx, config)
	defer cancel()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add multiple files
	for fieldName, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}

		part, err := writer.CreateFormFile(fieldName, filePath)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}

		if _, err = io.Copy(part, file); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}
		file.Close()
	}

	// Add extra fields
	for key, value := range config.MultipartFormParams {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write form field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	request, err := createRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())
	applyHeaders(request, config.Headers)
	return executeRequest[Res](config.HttpClient, request, config.CircuitBreaker)
}
