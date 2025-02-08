package httpClient

import "fmt"

// HTTPError Custom error type to be returned for status code < 200 and >300
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s - %s", e.StatusCode, e.Status, e.Body)
}
