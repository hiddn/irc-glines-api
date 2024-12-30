package ircglineapi

import (
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type ApiData struct {
	Config       Configuration
	EchoInstance *echo.Echo
}

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
	Message                 string  `param:"message"`
	RegexExpectedForSuccess *string `param:"regexexpectedforsuccess,omitempty"`
}

func Api_init(config Configuration) *echo.Echo {
	e := echo.New()
	a := &ApiData{
		Config:       config,
		EchoInstance: e,
	}
	e.Use(middleware.BodyLimit("1K"))
	e.Use(middleware.Logger())
	e.GET("/api2/glinelookup/:network/:ip", a.glineLookupApi)
	e.GET("/api2/ismyipgline/:network", a.glineLookupOwnIPApi)
	e.POST("/api2/sendcommand/:network", a.sendCommandApi)
	e.POST("/api2/remgline/:network", a.removeGlineApi)
	e.Use(middleware.Recover())
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper: a.IsAPIOpen,
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == config.ApiKey, nil
		},
	}))
	e.Logger.Fatal(e.Start("127.0.0.1:2000"))
	return e
}

func (a *ApiData) IsAPIOpen(c echo.Context) bool {
	switch c.Path() {
	case "/api2/glinelookup/:network/:ip":
		return true
	case "/api2/ismyipgline/:network":
		return true
	default:
		return false
	}
}

func (a *ApiData) removeGlineApi(c echo.Context) error {
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
	if len(in.Message) > 400 {
		in.Message = in.Message[:400] + " [...]"
	}
	in.Message = strings.ReplaceAll(in.Message, "\n", "|")
	s.MsgMainChan(in.Message)
	return c.JSON(http.StatusOK, "Command sent")
}

func (a *ApiData) sendCommandApi(c echo.Context) error {
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

func (a *ApiData) glineLookupApi(c echo.Context) error {
	var in api_struct
	err := c.Bind(&in)
	return a.glineApi(c, in, err)
}

func (a *ApiData) glineLookupOwnIPApi(c echo.Context) error {
	var in api_struct
	var in2 api_struct2
	err := c.Bind(&in2)
	in.Network = in2.Network
	in.Ip = c.RealIP()
	return a.glineApi(c, in, err)
}

func (a *ApiData) glineApi(c echo.Context, in api_struct, err error) error {
	var list []*RetGlineData
	if err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	if a.Config.ForbidCIDRLookupsViaAPI {
		in.Ip = strings.Split(in.Ip, "/")[0]
	}
	log.Println("ip =", in.Ip, ", net = ", in.Network)
	s := servers.GetServerInfosByNetwork(in.Network)
	if s == nil {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	if glines, exp_glines, err := s.CheckGline(in.Ip); err == nil {
		list = make([]*RetGlineData, 0, len(glines)+len(exp_glines))
		for _, entry := range glines {
			e := newRetGlineData(entry.mask, entry.reason, entry.expireTS, entry.lastModTS, entry.HoursUntilExpiration(), entry.active)
			list = append(list, e)
		}
		for _, entry := range exp_glines {
			e := newRetGlineData(entry.mask, entry.reason, entry.expireTS, entry.lastModTS, entry.HoursUntilExpiration(), entry.active)
			list = append(list, e)
		}
	} else {
		return c.JSON(http.StatusBadRequest, "Invalid IP")
	}
	return c.JSON(http.StatusOK, &list)
}
