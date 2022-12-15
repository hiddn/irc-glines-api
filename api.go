package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type RetGlineData struct {
	Active           string `json:"active"`
	Mask             string `json:"mask"`
	ExpireTS         string `json:"expirets"`
	LastModTS        string `json:"lastmodts"`
	HoursUntilExpire string `json:"hoursuntilexpire"`
	Reason           string `json:"reason"`
}
type RetGlineDatas struct {
	RetGlineData []RetGlineData `json:"glines"`
}

func newRetGlineData(mask, reason string, expireTS, lastModTS, hoursUntilExpire int64, active bool) *RetGlineData {
	return &RetGlineData{
		Active:           strconv.FormatBool(active),
		Mask:             mask,
		ExpireTS:         strconv.Itoa(int(expireTS)),
		LastModTS:        strconv.Itoa(int(lastModTS)),
		HoursUntilExpire: strconv.Itoa(int(hoursUntilExpire)),
		Reason:           reason,
	}
}

func start_api() {
	e := echo.New()

	e.Use(middleware.BodyLimit("1K"))
	e.GET("/checkgline/:network/:ip", checkGlineApi)
	e.Logger.Fatal(e.Start("127.0.0.1:2000"))
}

func checkGlineApi(c echo.Context) error {
	ip := c.Param("ip")
	network := c.Param("network")
	log.Println("ip =", ip, ", net = ", network)
	s := servers.GetServerInfosByNetwork(network)
	if s == nil {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	list := make([]*RetGlineData, 0, 10)
	if glines, exp_glines, err := s.CheckGline(ip); err == nil {
		for _, entry := range glines {
			e := newRetGlineData(entry.mask, entry.reason, entry.expireTS, entry.lastModTS, entry.HoursUntilExpiration(), entry.IsGlineActive())
			list = append(list, e)
		}
		for _, entry := range exp_glines {
			e := newRetGlineData(entry.mask, entry.reason, entry.expireTS, entry.lastModTS, entry.HoursUntilExpiration(), entry.IsGlineActive())
			list = append(list, e)
		}
	}
	return c.JSON(http.StatusOK, &list)
}
