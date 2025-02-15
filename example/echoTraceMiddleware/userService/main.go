package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/rag594/rustic"
	"github.com/rag594/rustic/httpClient"
	"github.com/rag594/rustic/rusticTracer"
	"net/http"
	"time"
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
	e := echo.New()
	// you can try out with tracer.StdOutExporter() in your localhost
	shutdown := rusticTracer.InitTracer("userService", "dev", rusticTracer.OTLPExporter("localhost", "4318"))

	defer shutdown()
	e.Use(rusticTracer.Echov4TracerMiddleware("userService"))
	client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))
	e.POST("/user/:user_id/post", func(c echo.Context) error {

		url := "http://localhost:1345/create-post"

		userPostReq := &UserPostReq{Title: "foo", Body: "bar", UserId: 1}

		ctx := c.Request().Context()

		post, err := rustic.POST[UserPostReq, UserPostResp](ctx,
			url,
			userPostReq,
			rustic.WithHttpClient(client),
			rustic.WithTimeout(time.Duration(4)*time.Minute),
		)

		if err != nil {
			fmt.Println(err)
		}

		return c.JSON(http.StatusOK, post)
	})
	e.Logger.Fatal(e.Start(":1323"))
}
