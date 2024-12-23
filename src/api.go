package ircglineapi

import (
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type RetGlineData struct {
	Active           bool   `json:"active"`
	Mask             string `json:"mask"`
	ExpireTS         int64  `json:"expirets"`
	LastModTS        int64  `json:"lastmodts"`
	HoursUntilExpire int64  `json:"hoursuntilexpire"`
	Reason           string `json:"reason"`
}
type RetGlineDatas struct {
	RetGlineData []RetGlineData `json:"glines"`
}

func newRetGlineData(mask, reason string, expireTS, lastModTS, hoursUntilExpire int64, active bool) *RetGlineData {
	return &RetGlineData{
		Active:           active,
		Mask:             mask,
		ExpireTS:         expireTS,
		LastModTS:        lastModTS,
		HoursUntilExpire: hoursUntilExpire,
		Reason:           reason,
	}
}

type api_struct struct {
	Network string `param:"network"`
	Ip      string `param:"ip"`
}

type api_struct2 struct {
	Network string `param:"network"`
}

type api_irccmd_struct struct {
	Network                 string  `param:"network"`
	Command                 string  `param:"command"`
	RegexExpectedForSuccess *string `param:"regexexpectedforsuccess,omitempty"`
}

type api_remgline_struct struct {
	Network                 string  `param:"network"`
	GlineMask               string  `param:"glinemask"`
	RegexExpectedForSuccess *string `param:"regexexpectedforsuccess,omitempty"`
}

func Api_init(config Configuration) *echo.Echo {
	e := echo.New()

	e.Use(middleware.BodyLimit("1K"))
	e.Use(middleware.Logger())
	e.GET("/api2/glinelookup/:network/:ip", glineLookupApi)
	e.GET("/api2/ismyipgline/:network", glineLookupOwnIPApi)
	e.POST("/api2/sendcommand/:network", sendCommandApi)
	e.POST("/api2/remgline/:network", removeGlineApi)
	e.Use(middleware.Recover())
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper: isAPIOpen,
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == config.ApiKey, nil
		},
	}))
	e.Logger.Fatal(e.Start("127.0.0.1:2000"))
	return e
}

func isAPIOpen(c echo.Context) bool {
	switch c.Path() {
	case "/api2/glinelookup/:network/:ip":
		return true
	case "/api2/ismyipgline/:network":
		return true
	default:
		return false
	}
}

func removeGlineApi(c echo.Context) error {
	var in api_remgline_struct
	err := c.Bind(&in)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	s := servers.GetServerInfosByNetwork(in.Network)
	if s == nil {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	if !s.Conn.Connected() {
		return c.JSON(http.StatusServiceUnavailable, "Server not connected")
	}
	s.sendCommandToOperServ(strings.Replace(s.Config.OperServRemglineCmd, "$glinemask", in.GlineMask, -1))
	return c.JSON(http.StatusOK, "Command sent")
}

func sendCommandApi(c echo.Context) error {
	var in api_irccmd_struct
	err := c.Bind(&in)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	s := servers.GetServerInfosByNetwork(in.Network)
	if s == nil {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	if !s.Conn.Connected() {
		return c.JSON(http.StatusServiceUnavailable, "Server not connected")
	}
	s.Conn.Raw(in.Command)
	return c.JSON(http.StatusOK, "Command sent")
}

func glineLookupApi(c echo.Context) error {
	var in api_struct
	err := c.Bind(&in)
	return glineApi(c, in, err)
}

func glineLookupOwnIPApi(c echo.Context) error {
	var in api_struct
	var in2 api_struct2
	err := c.Bind(&in2)
	in.Network = in2.Network
	in.Ip = c.RealIP()
	return glineApi(c, in, err)
}

func glineApi(c echo.Context, in api_struct, err error) error {
	if err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	log.Println("ip =", in.Ip, ", net = ", in.Network)
	s := servers.GetServerInfosByNetwork(in.Network)
	if s == nil {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	list := make([]*RetGlineData, 0, 10)
	if glines, exp_glines, err := s.CheckGline(in.Ip); err == nil {
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
