package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo"
)

func start_api() {
	//var err error

	e := echo.New()

	type Employee struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		Salary string `json: "salary"`
		Age    string `json: "age"`
	}
	type Employees struct {
		Employees []Employee `json:"employees"`
	}

	e.GET("/checkgline/:ip", checkGlineApi)
	e.Logger.Fatal(e.Start("127.0.0.1:2000"))
}

func checkGlineApi(c echo.Context) error {
	//TODO: limit input's length
	//return c.JSON(http.StatusCreated, "")
	ip := c.Param("ip")
	log.Println("ip =", ip)
	fmt.Println("ip =", ip)
	return c.String(http.StatusOK, "Test "+ip)
}
