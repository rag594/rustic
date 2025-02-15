package rustic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rag594/rustic/httpClient"
	"github.com/rag594/rustic/rusticTracer"
	"github.com/sony/gobreaker/v2"
	otelTracer "go.opentelemetry.io/otel/trace"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	netUrl "net/url"
	"os"
	"strings"
	"time"
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

// GET http method with Res as response type
func GET[Res any](ctx context.Context, url string, opts ...HTTPConfigOptions) (*Res, error) {
	getOpts := &HTTPConfig{
		// add default options
	}

	for _, opt := range opts {
		opt(getOpts)
	}

	if getOpts.Headers != nil {
		getOpts.Headers = map[string][]string{
			"Content-Type": {"application/json"},
		}
	}

	var (
		parentCtx  context.Context
		newCtx     context.Context
		cancelFunc context.CancelFunc
		span       otelTracer.Span
	)

	// if ctx is nil then set parentContext to background else use the passed one as parent
	if ctx == nil {
		parentCtx = context.Background()
	} else {
		parentCtx = ctx
	}

	newCtx = parentCtx

	// if timeout is set then create new context
	if getOpts.Timeout != 0 {
		newCtx, cancelFunc = context.WithTimeout(newCtx, getOpts.Timeout)
		defer cancelFunc()
	}

	// if trace is enabled start trace using new context
	if getOpts.HttpClient.TraceEnabled {
		tr := rusticTracer.GetTracer(getOpts.HttpClient.ServiceName)
		newCtx, span = tr.Start(newCtx, httpClient.GetCallerFunctionName())
		defer span.End()
	}

	parsedURL, err := netUrl.Parse(url)
	if err != nil {
		log.Fatal(err)
	}

	if len(getOpts.QueryParams) != 0 {
		parsedURL.RawQuery = getOpts.QueryParams.Encode()
	}

	request, err := http.NewRequestWithContext(newCtx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, errors.New("unable to create get request")
	}

	for key, value := range getOpts.Headers {
		if len(value[0]) > 0 {
			request.Header.Set(key, value[0])
		}
	}

	if getOpts.CircuitBreaker != nil {
		response, err := getOpts.CircuitBreaker.Execute(func() (any, error) {
			resp, err := getOpts.HttpClient.Do(request)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				t := new(Res)
				err = json.NewDecoder(resp.Body).Decode(t)
				return t, nil
			}
			// Read response body
			body, _ := io.ReadAll(resp.Body)
			defer resp.Body.Close()

			return nil, &httpClient.HTTPError{StatusCode: resp.StatusCode, Status: http.StatusText(resp.StatusCode), Body: string(body)}
		})
		if response != nil {
			return response.(*Res), err
		}
		return nil, err
	}

	response, err := getOpts.HttpClient.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t := new(Res)
		err = json.NewDecoder(response.Body).Decode(t)
		defer response.Body.Close()
		return t, nil
	}
	// Read response body
	body, _ := io.ReadAll(response.Body)
	defer response.Body.Close()

	return nil, &httpClient.HTTPError{StatusCode: response.StatusCode, Status: http.StatusText(response.StatusCode), Body: string(body)}

}

// POST http method with Req as request type and Res as response type
func POST[Req, Res any](ctx context.Context, url string, req *Req, opts ...HTTPConfigOptions) (*Res, error) {
	postOpts := &HTTPConfig{
		// add default options
	}

	for _, opt := range opts {
		opt(postOpts)
	}

	if postOpts.Headers != nil {
		postOpts.Headers = map[string][]string{
			"Content-Type": {"application/json"},
		}
	}

	var (
		parentCtx  context.Context
		newCtx     context.Context
		cancelFunc context.CancelFunc
		span       otelTracer.Span
	)

	// if ctx is nil then set parentContext to background else use the passed one as parent
	if ctx == nil {
		parentCtx = context.Background()
	} else {
		parentCtx = ctx
	}

	newCtx = parentCtx

	// if timeout is set then create new context
	if postOpts.Timeout != 0 {
		newCtx, cancelFunc = context.WithTimeout(newCtx, postOpts.Timeout)
		defer cancelFunc()
	}

	// if trace is enabled start trace using new context
	if postOpts.HttpClient.TraceEnabled {
		tr := rusticTracer.GetTracer(postOpts.HttpClient.ServiceName)
		newCtx, span = tr.Start(newCtx, httpClient.GetCallerFunctionName())
		defer span.End()
	}

	b, _ := json.Marshal(req)

	request, err := http.NewRequestWithContext(newCtx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("unable to create post request")
	}

	for key, value := range postOpts.Headers {
		if len(value[0]) > 0 {
			request.Header.Set(key, value[0])
		}
	}

	if postOpts.CircuitBreaker != nil {
		response, err := postOpts.CircuitBreaker.Execute(func() (any, error) {
			resp, err := postOpts.HttpClient.Do(request)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				t := new(Res)
				err = json.NewDecoder(resp.Body).Decode(t)
				return t, nil
			}
			// Read response body
			body, _ := io.ReadAll(resp.Body)
			defer resp.Body.Close()

			return nil, &httpClient.HTTPError{StatusCode: resp.StatusCode, Status: http.StatusText(resp.StatusCode), Body: string(body)}
		})
		if response != nil {
			return response.(*Res), err
		}
		return nil, err
	}

	response, err := postOpts.HttpClient.Do(request)
	defer response.Body.Close()

	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t := new(Res)
		err = json.NewDecoder(response.Body).Decode(t)
		return t, nil
	}

	// Read response body
	body, _ := io.ReadAll(response.Body)

	return nil, &httpClient.HTTPError{StatusCode: response.StatusCode, Status: http.StatusText(response.StatusCode), Body: string(body)}
}

// PUT http method with Req as request type and Res as response type
func PUT[Req, Res any](ctx context.Context, url string, req *Req, opts ...HTTPConfigOptions) (*Res, error) {
	putOpts := &HTTPConfig{
		// add default options
	}

	if putOpts.Headers != nil {
		putOpts.Headers = map[string][]string{
			"Content-Type": {"application/json"},
		}
	}

	for _, opt := range opts {
		opt(putOpts)
	}

	var (
		parentCtx  context.Context
		newCtx     context.Context
		cancelFunc context.CancelFunc
		span       otelTracer.Span
	)

	// if ctx is nil then set parentContext to background else use the passed one as parent
	if ctx == nil {
		parentCtx = context.Background()
	} else {
		parentCtx = ctx
	}

	newCtx = parentCtx

	// if timeout is set then create new context
	if putOpts.Timeout != 0 {
		newCtx, cancelFunc = context.WithTimeout(newCtx, putOpts.Timeout)
		defer cancelFunc()
	}

	// if trace is enabled start trace using new context
	if putOpts.HttpClient.TraceEnabled {
		tr := rusticTracer.GetTracer(putOpts.HttpClient.ServiceName)
		newCtx, span = tr.Start(newCtx, httpClient.GetCallerFunctionName())
		defer span.End()
	}

	b, _ := json.Marshal(req)

	request, err := http.NewRequestWithContext(newCtx, http.MethodPut, url, bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("unable to create Put request")
	}

	for key, value := range putOpts.Headers {
		if len(value[0]) > 0 {
			request.Header.Set(key, value[0])
		}
	}

	response, err := putOpts.HttpClient.Do(request)
	defer response.Body.Close()

	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t := new(Res)
		err = json.NewDecoder(response.Body).Decode(t)
		return t, nil
	}

	// Read response body
	body, _ := io.ReadAll(response.Body)

	return nil, &httpClient.HTTPError{StatusCode: response.StatusCode, Status: http.StatusText(response.StatusCode), Body: string(body)}
}

// POSTFormData with Res as response type and allows application/x-www-form-urlencoded -> formData
func POSTFormData[Res any](ctx context.Context, url string, opts ...HTTPConfigOptions) (*Res, error) {
	postOpts := &HTTPConfig{
		// add default options
	}

	if postOpts.Headers != nil {
		postOpts.Headers = map[string][]string{
			"Content-Type": {"application/x-www-form-urlencoded"},
		}
	}

	for _, opt := range opts {
		opt(postOpts)
	}

	var (
		parentCtx  context.Context
		newCtx     context.Context
		cancelFunc context.CancelFunc
		span       otelTracer.Span
	)

	// if ctx is nil then set parentContext to background else use the passed one as parent
	if ctx == nil {
		parentCtx = context.Background()
	} else {
		parentCtx = ctx
	}

	newCtx = parentCtx

	// if timeout is set then create new context
	if postOpts.Timeout != 0 {
		newCtx, cancelFunc = context.WithTimeout(newCtx, postOpts.Timeout)
		defer cancelFunc()
	}

	// if trace is enabled start trace using new context
	if postOpts.HttpClient.TraceEnabled {
		tr := rusticTracer.GetTracer(postOpts.HttpClient.ServiceName)
		newCtx, span = tr.Start(newCtx, httpClient.GetCallerFunctionName())
		defer span.End()
	}

	request, err := http.NewRequestWithContext(newCtx, http.MethodPost, url, strings.NewReader(postOpts.FormParams.Encode()))
	if err != nil {
		return nil, errors.New("unable to create post request")
	}

	for key, value := range postOpts.Headers {
		if len(value[0]) > 0 {
			request.Header.Set(key, value[0])
		}
	}

	response, err := postOpts.HttpClient.Do(request)
	defer response.Body.Close()

	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t := new(Res)
		err = json.NewDecoder(response.Body).Decode(t)
		return t, nil
	}

	// Read response body
	body, _ := io.ReadAll(response.Body)

	return nil, &httpClient.HTTPError{StatusCode: response.StatusCode, Status: http.StatusText(response.StatusCode), Body: string(body)}
}

// POSTMultiPartFormData with Res as response type, map of files with key as fieldName and value as filePath
func POSTMultiPartFormData[Res any](ctx context.Context, url string, files map[string]string, opts ...HTTPConfigOptions) (*Res, error) {
	postOpts := &HTTPConfig{
		// add default options
	}

	for _, opt := range opts {
		opt(postOpts)
	}

	if postOpts.Headers != nil {
		postOpts.Headers = make(http.Header)
	}

	var (
		parentCtx  context.Context
		newCtx     context.Context
		cancelFunc context.CancelFunc
		span       otelTracer.Span
	)

	// if ctx is nil then set parentContext to background else use the passed one as parent
	if ctx == nil {
		parentCtx = context.Background()
	} else {
		parentCtx = ctx
	}

	newCtx = parentCtx

	// if timeout is set then create new context
	if postOpts.Timeout != 0 {
		newCtx, cancelFunc = context.WithTimeout(newCtx, postOpts.Timeout)
		defer cancelFunc()
	}

	// if trace is enabled start trace using new context
	if postOpts.HttpClient.TraceEnabled {
		tr := rusticTracer.GetTracer(postOpts.HttpClient.ServiceName)
		newCtx, span = tr.Start(newCtx, httpClient.GetCallerFunctionName())
		defer span.End()
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	defer writer.Close()

	// Add multiple files
	for fieldName, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}

		part, err := writer.CreateFormFile(fieldName, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}
		_, err = io.Copy(part, file)
		if err != nil {
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}

		file.Close()
	}

	// Add extra fields
	for key, value := range postOpts.MultipartFormParams {
		_ = writer.WriteField(key, value)
	}

	request, err := http.NewRequestWithContext(newCtx, http.MethodPost, url, body)
	if err != nil {
		return nil, errors.New("unable to create post request")
	}

	postOpts.Headers.Set("Content-Type", writer.FormDataContentType())

	for key, value := range postOpts.Headers {
		if len(value[0]) > 0 {
			request.Header.Set(key, value[0])
		}
	}

	response, err := postOpts.HttpClient.Do(request)
	defer response.Body.Close()

	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t := new(Res)
		err = json.NewDecoder(response.Body).Decode(t)
		return t, nil
	}

	// Read response body
	resBody, _ := io.ReadAll(response.Body)

	return nil, &httpClient.HTTPError{StatusCode: response.StatusCode, Status: http.StatusText(response.StatusCode), Body: string(resBody)}
}
