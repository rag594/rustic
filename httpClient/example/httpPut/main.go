package main

import (
	"context"
	"fmt"
	"github.com/rag594/rustic/httpClient"
	"github.com/rag594/rustic/tracer"
	"time"
)

type UserPutReq struct {
	Id     int    `json:"id"`
	UserId int    `json:"userId"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type UserPutResp struct {
	Id     int    `json:"id"`
	UserId int    `json:"userId"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func main() {
	shutdown := tracer.InitTracer("microserviceA")
	defer shutdown()

	client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))
	url := "https://jsonplaceholder.typicode.com/posts/1"

	userPutReq := &UserPutReq{Title: "foo", Body: "bar", UserId: 1, Id: 1}

	post, err := httpClient.PUT[UserPutReq, UserPutResp](context.Background(),
		url,
		userPutReq,
		httpClient.WithHttpClient(client),
		httpClient.WithTimeout(time.Duration(1)*time.Minute),
	)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(post)
}
