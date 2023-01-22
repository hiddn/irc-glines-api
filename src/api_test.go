package ircglineapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Run it like this: go test -v ./... -bench Benchmark

func BenchmarkCheckGlineApi(b *testing.B) {
	e := echo.New()
	e.Use(middleware.BodyLimit("1K"))
	e.GET("/checkgline/:network/:ip", checkGlineApi)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/checkgline/undernet/1.2.3.4", nil)

	for i := 0; i < b.N; i++ {
		e.ServeHTTP(w, r)
	}
}
