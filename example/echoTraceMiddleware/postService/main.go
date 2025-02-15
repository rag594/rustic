package main

import (
	"github.com/labstack/echo/v4"
	"github.com/rag594/rustic/rusticTracer"
	"net/http"
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
	e.Use(rusticTracer.Echov4TracerMiddleware("postService"))
	shutdown := rusticTracer.InitTracer("postService", "dev", rusticTracer.OTLPExporter("localhost", "4318"))
	defer shutdown()
	e.POST("/create-post", func(c echo.Context) error {

		return c.JSON(http.StatusOK, &UserPostResp{
			Id:     5,
			UserId: 5,
			Title:  "clkdaj",
			Body:   "vcvrwrw",
		})
	})
	e.Logger.Fatal(e.Start(":1345"))
}
