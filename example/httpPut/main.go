package main

import (
	"context"
	"fmt"
	"github.com/rag594/rustic"
	"github.com/rag594/rustic/httpClient"
	"github.com/rag594/rustic/rusticTracer"
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
	shutdown := rusticTracer.InitTracer("microserviceA", "dev", rusticTracer.StdOutExporter())
	defer shutdown()

	client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))
	url := "https://jsonplaceholder.typicode.com/posts/1"

	userPutReq := &UserPutReq{Title: "foo", Body: "bar", UserId: 1, Id: 1}

	post, err := rustic.PUT[UserPutReq, UserPutResp](context.Background(),
		url,
		userPutReq,
		rustic.WithHttpClient(client),
		rustic.WithTimeout(time.Duration(1)*time.Minute),
	)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(post)
}
