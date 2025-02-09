package main

import (
	"context"
	"fmt"
	"github.com/rag594/rustic"
	"github.com/rag594/rustic/httpClient"
	"github.com/rag594/rustic/tracer"
	"github.com/sony/gobreaker/v2"
	url2 "net/url"
	"time"
)

type UserPost struct {
	UserId int    `json:"userId"`
	Id     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func main() {

	shutdown := tracer.InitTracer("microserviceA", "dev", tracer.StdOutExporter())
	defer shutdown()

	// With Circuit breaker

	client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))
	// invalid host to test out the circuit breaker
	url := "https://abd.vty"

	params := url2.Values{}
	params.Add("userId", "1")

	st := &gobreaker.Settings{}
	st.Name = "HTTP GET"

	st.ReadyToTrip = func(counts gobreaker.Counts) bool {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		return counts.Requests >= 3 && failureRatio >= 0.6
	}

	cb := gobreaker.NewCircuitBreaker[any](*st)

	for i := 0; i < 10; i++ {
		post, err := rustic.GET[[]UserPost](context.Background(),
			url,
			rustic.WithQueryParams(params),
			rustic.WithHttpClient(client),
			rustic.WithTimeout(time.Duration(1)*time.Second),
			rustic.WithCircuitBreaker(cb),
		)
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println(post)
	}

	// Without CircuitBreaker
	newUrl := "https://jsonplaceholder.typicode.com/posts"
	post, err := rustic.GET[[]UserPost](context.Background(),
		newUrl,
		rustic.WithQueryParams(params),
		rustic.WithHttpClient(client),
		rustic.WithTimeout(time.Duration(1)*time.Second),
	)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(post)

}
