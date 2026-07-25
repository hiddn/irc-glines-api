package ircglineapi

import (
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
	ID               string `json:"id"`
}
type RetGlineDatas struct {
	RetGlineData []RetGlineData `json:"glines"`
}

func newRetGlineData(mask, reason string, expireTS, lastModTS, hoursUntilExpire int64, active bool, id string) *RetGlineData {
	return &RetGlineData{
		Active:           active,
		Mask:             mask,
		ExpireTS:         expireTS,
		LastModTS:        lastModTS,
		HoursUntilExpire: hoursUntilExpire,
		Reason:           reason,
		ID:               id,
	}
}

// redactMaskHost replaces the host part of a "user@host" mask with a
// placeholder, keeping the user part intact.
func redactMaskHost(mask string) string {
	if i := strings.Index(mask, "@"); i != -1 {
		return mask[:i+1] + "[hidden]"
	}
	return mask
}

// buildRetGlineDataList builds the JSON response list for a set of gline
// entries. When redactIP is true (public ID-based lookups), the mask's
// host/IP part is replaced with a placeholder.
func buildRetGlineDataList(entries []*glineData, redactIP bool) []*RetGlineData {
	list := make([]*RetGlineData, 0, len(entries))
	for _, e := range entries {
		mask := e.Mask()
		if redactIP {
			mask = redactMaskHost(mask)
		}
		list = append(list, newRetGlineData(mask, e.reason, e.expireTS, e.lastModTS, e.HoursUntilExpiration(), e.active, e.ID()))
	}
	return list
}

type api_struct struct {
	Network string `param:"network"`
	Ip      string `param:"ip"`
}

type api_struct_id struct {
	Network string `param:"network"`
	ID      string `param:"id"`
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
	e.GET("/api2/glineidlookup/:network/:id", a.glineIDLookupApi)
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
	case "/api2/glineidlookup/:network/:id":
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

func (a *ApiData) glineIDLookupApi(c echo.Context) error {
	var in api_struct_id
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	debugLog("id =", in.ID, ", net = ", in.Network)
	s := servers.GetServerInfosByNetwork(in.Network)
	if s == nil {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	entries, _ := s.CheckGlineByID(in.ID)
	list := buildRetGlineDataList(entries, true)
	return c.JSON(http.StatusOK, &list)
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
	debugLog("ip =", in.Ip, ", net = ", in.Network)
	s := servers.GetServerInfosByNetwork(in.Network)
	if s == nil {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	if glines, exp_glines, err := s.CheckGline(in.Ip, false); err == nil {
		list = buildRetGlineDataList(append(glines, exp_glines...), false)
	} else {
		return c.JSON(http.StatusBadRequest, "Invalid IP")
	}
	return c.JSON(http.StatusOK, &list)
}
