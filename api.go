package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
)

type RetGlineData struct {
	Active           string `json:"active"`
	Mask             string `json:"mask"`
	ExpireTS         string `json:"expirets"`
	LastModTS        string `json:"lastmodts"`
	HoursUntilExpire string `json:"hoursuntilexpire"`
}
type RetGlineDatas struct {
	RetGlineData []RetGlineData `json:"glines"`
}

func newRetGlineData(mask string, expireTS, lastModTS, hoursUntilExpire int64, active bool) *RetGlineData {
	return &RetGlineData{
		Active:           strconv.FormatBool(active),
		Mask:             mask,
		ExpireTS:         strconv.Itoa(int(expireTS)),
		LastModTS:        strconv.Itoa(int(lastModTS)),
		HoursUntilExpire: strconv.Itoa(int(hoursUntilExpire)),
	}
}

func start_api() {
	//var err error

	e := echo.New()

	e.GET("/checkgline/:network/:ip", checkGlineApi)
	e.Logger.Fatal(e.Start("127.0.0.1:2000"))
}

func checkGlineApi(c echo.Context) error {
	//TODO: limit input's length
	//return c.JSON(http.StatusCreated, "")
	ip := c.Param("ip")
	network := c.Param("network")
	log.Println("ip =", ip, ", net = ", network)
	fmt.Println("ip =", ip, ", net = ", network)
	s := servers.GetServerInfosByNetwork(network)
	list := make([]*RetGlineData, 0, 10)
	if glines, exp_glines, err := s.CheckGline(ip); err == nil {
		for _, entry := range glines {
			e := newRetGlineData(entry.mask, entry.expireTS, entry.lastModTS, entry.HoursUntilExpiration(), entry.IsGlineActive())
			list = append(list, e)
		}
		for _, entry := range exp_glines {
			e := newRetGlineData(entry.mask, entry.expireTS, entry.lastModTS, entry.HoursUntilExpiration(), entry.IsGlineActive())
			list = append(list, e)
		}
	}
	return c.String(http.StatusOK, "Test "+ip)
}
