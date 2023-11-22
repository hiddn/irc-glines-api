package ircglineapi

import (
	"log"
	"net/http"

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

func Api_init(config Configuration) *echo.Echo {
	e := echo.New()

	e.Use(middleware.BodyLimit("1K"))
	e.Use(middleware.Logger())
	e.GET("/checkgline/:network/:ip", checkGlineApi)
	e.GET("/glinelookup/:network", checkGlineOwnIPApi)
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
	if c.Path() == "/glinelookup/:network" {
		return true
	}
	return false
}
func checkGlineApi(c echo.Context) error {
	var in api_struct
	err := c.Bind(&in)
	return glineApi(c, in, err)
}

func checkGlineOwnIPApi(c echo.Context) error {
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
