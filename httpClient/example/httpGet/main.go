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

	shutdown := tracer.InitTracer("microserviceA")
	defer shutdown()

	client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))
	url := "https://jsonplaceholder.typicode.com/posts"

	params := url2.Values{}
	params.Add("userId", "1")

	st := &gobreaker.Settings{}
	st.Name = "HTTP GET"

	st.ReadyToTrip = func(counts gobreaker.Counts) bool {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		fmt.Println(failureRatio, counts.Requests)
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

}
