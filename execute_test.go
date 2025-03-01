package rustic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rag594/rustic/httpClient"
	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestRequest struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type TestResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *httpClient.HTTPClient) {
	server := httptest.NewServer(handler)
	t.Cleanup(func() { server.Close() })

	client := &httpClient.HTTPClient{
		Client:       server.Client(),
		TraceEnabled: false,
		ServiceName:  "test-service",
	}

	return server, client
}

func TestGET(t *testing.T) {
	testCases := []struct {
		name           string
		handler        http.HandlerFunc
		setupConfig    []HTTPConfigOptions
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "successful GET request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				response := TestResponse{ID: 1, Name: "John", Age: 30}
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			},
			setupConfig: []HTTPConfigOptions{
				WithTimeout(time.Second * 5),
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "GET request with query params",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "John", r.URL.Query().Get("name"))
				assert.Equal(t, "30", r.URL.Query().Get("age"))

				response := TestResponse{ID: 1, Name: "John", Age: 30}
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			},
			setupConfig: []HTTPConfigOptions{
				WithQueryParams(url.Values{
					"name": []string{"John"},
					"age":  []string{"30"},
				}),
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "GET request with error response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, err := w.Write([]byte(`{"error": "not found"}`))
				require.NoError(t, err)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := setupTestServer(t, tc.handler)

			config := append(tc.setupConfig, WithHttpClient(client))
			resp, err := GET[TestResponse](context.Background(), server.URL, config...)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			if resp != nil {
				assert.Equal(t, "John", resp.Name)
				assert.Equal(t, 30, resp.Age)
			}
		})
	}
}

func TestPOST(t *testing.T) {
	testCases := []struct {
		name           string
		request        *TestRequest
		handler        http.HandlerFunc
		setupConfig    []HTTPConfigOptions
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "successful POST request",
			request: &TestRequest{
				Name: "John",
				Age:  30,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var req TestRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, "John", req.Name)
				assert.Equal(t, 30, req.Age)

				response := TestResponse{ID: 1, Name: req.Name, Age: req.Age}
				w.WriteHeader(http.StatusOK)
				err = json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "POST request with custom headers",
			request: &TestRequest{
				Name: "John",
				Age:  30,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
				response := TestResponse{ID: 1, Name: "John", Age: 30}
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			},
			setupConfig: []HTTPConfigOptions{
				WithHeaders(http.Header{
					"Authorization": []string{"Bearer token123"},
				}),
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := setupTestServer(t, tc.handler)

			config := append(tc.setupConfig, WithHttpClient(client))
			resp, err := POST[TestRequest, TestResponse](context.Background(), server.URL, tc.request, config...)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			if resp != nil {
				assert.Equal(t, tc.request.Name, resp.Name)
				assert.Equal(t, tc.request.Age, resp.Age)
			}
		})
	}
}

func TestPUT(t *testing.T) {
	testCases := []struct {
		name           string
		request        *TestRequest
		handler        http.HandlerFunc
		setupConfig    []HTTPConfigOptions
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "successful PUT request",
			request: &TestRequest{
				Name: "John Updated",
				Age:  31,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var req TestRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, "John Updated", req.Name)
				assert.Equal(t, 31, req.Age)

				response := TestResponse{ID: 1, Name: req.Name, Age: req.Age}
				w.WriteHeader(http.StatusOK)
				err = json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := setupTestServer(t, tc.handler)

			config := append(tc.setupConfig, WithHttpClient(client))
			resp, err := PUT[TestRequest, TestResponse](context.Background(), server.URL, tc.request, config...)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			if resp != nil {
				assert.Equal(t, tc.request.Name, resp.Name)
				assert.Equal(t, tc.request.Age, resp.Age)
			}
		})
	}
}

func TestPOSTFormData(t *testing.T) {
	testCases := []struct {
		name           string
		formData       url.Values
		handler        http.HandlerFunc
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "successful form data POST",
			formData: url.Values{
				"name": []string{"John"},
				"age":  []string{"30"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

				err := r.ParseForm()
				require.NoError(t, err)
				assert.Equal(t, "John", r.Form.Get("name"))
				assert.Equal(t, "30", r.Form.Get("age"))

				response := TestResponse{ID: 1, Name: r.Form.Get("name"), Age: 30}
				w.WriteHeader(http.StatusOK)
				err = json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := setupTestServer(t, tc.handler)

			resp, err := POSTFormData[TestResponse](
				context.Background(),
				server.URL,
				WithHttpClient(client),
				WithFormParams(tc.formData),
			)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			if resp != nil {
				assert.Equal(t, "John", resp.Name)
				assert.Equal(t, 30, resp.Age)
			}
		})
	}
}

func TestPOSTMultiPartFormData(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(tempFile, []byte("test content"), 0644)
	require.NoError(t, err)

	testCases := []struct {
		name           string
		files          map[string]string
		formData       map[string]string
		handler        http.HandlerFunc
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "successful multipart form data POST",
			files: map[string]string{
				"file": tempFile,
			},
			formData: map[string]string{
				"name": "John",
				"age":  "30",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

				err := r.ParseMultipartForm(10 << 20)
				require.NoError(t, err)

				// Check form fields
				assert.Equal(t, "John", r.FormValue("name"))
				assert.Equal(t, "30", r.FormValue("age"))

				// Check file
				file, header, err := r.FormFile("file")
				require.NoError(t, err)
				defer file.Close()

				content, err := io.ReadAll(file)
				require.NoError(t, err)
				assert.Equal(t, "test content", string(content))
				assert.Equal(t, "test.txt", header.Filename)

				response := TestResponse{ID: 1, Name: r.FormValue("name"), Age: 30}
				w.WriteHeader(http.StatusOK)
				err = json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := setupTestServer(t, tc.handler)

			resp, err := POSTMultiPartFormData[TestResponse](
				context.Background(),
				server.URL,
				tc.files,
				WithHttpClient(client),
				WithMultiPartFormParams(tc.formData),
			)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			if resp != nil {
				assert.Equal(t, "John", resp.Name)
				assert.Equal(t, 30, resp.Age)
			}
		})
	}
}

func TestCircuitBreaker(t *testing.T) {
	settings := gobreaker.Settings{
		Name:        "test-breaker",
		MaxRequests: 0, // No requests allowed when open
		Interval:    time.Second * 10,
		Timeout:     time.Second * 60,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 2 // Open after 3 consecutive failures
		},
	}
	breaker := gobreaker.NewCircuitBreaker[any](settings)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	server, client := setupTestServer(t, handler)

	// Make multiple requests to trigger the circuit breaker
	for i := 0; i < 5; i++ {
		_, err := GET[TestResponse](
			context.Background(),
			server.URL,
			WithHttpClient(client),
			WithCircuitBreaker(breaker),
		)
		assert.Error(t, err)
		time.Sleep(time.Millisecond * 100) // Give the circuit breaker time to update state
	}

	// Verify circuit breaker is open
	assert.Equal(t, gobreaker.StateOpen, breaker.State())
}

func TestTimeout(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second * 2)
		w.WriteHeader(http.StatusOK)
	}

	server, client := setupTestServer(t, handler)

	_, err := GET[TestResponse](
		context.Background(),
		server.URL,
		WithHttpClient(client),
		WithTimeout(time.Second),
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
