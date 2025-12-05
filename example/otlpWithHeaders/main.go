package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rag594/rustic/httpClient"
	"github.com/rag594/rustic/rusticTracer"
)

type UserPostReq struct {
	UserId int    `json:"userId"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type UserPostResp struct {
	Id     int    `json:"id"`
	UserId int    `json:"userId"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func main() {
	// Define custom headers for OTLP exporter
	// This is useful for authentication with backends like Grafana Cloud, Honeycomb, etc.
	headers := map[string]string{
		"Authorization": "Bearer <your-api-token>",
	}

	// Initialize tracer with OTLP exporter using custom headers
	shutdown := rusticTracer.InitTracer("microserviceA", "dev", rusticTracer.OTLPExporter("localhost", "4318", headers))
	defer shutdown()

	client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))

	url := "https://jsonplaceholder.typicode.com/posts"

	userPostReq := &UserPostReq{Title: "foo", Body: "bar", UserId: 1}

	post, err := httpClient.POST[UserPostReq, UserPostResp](context.Background(),
		url,
		userPostReq,
		httpClient.WithHttpClient(client),
		httpClient.WithTimeout(time.Duration(1)*time.Minute),
	)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(post)
}
