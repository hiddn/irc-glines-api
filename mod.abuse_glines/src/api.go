package abuse_glines

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var config Configuration

type rules struct {
	RegexReason  string `json:"regexreason"`
	MustBeSameIP bool   `json:"mustbesameip"`
	Autoremove   bool   `json:"autoremove"`
}

type RetApiData struct {
	Active           bool   `json:"active"`
	Mask             string `json:"mask"`
	ExpireTS         int64  `json:"expirets"`
	LastModTS        int64  `json:"lastmodts"`
	HoursUntilExpire int64  `json:"hoursuntilexpire"`
	Reason           string `json:"reason"`
	Ip               string `json:"ip"`
	AutoRemove       bool   `json:"autoremove"`
	Msg              string `json:"msg"`
}

type RetApiDatas struct {
	RetApiData []RetApiData `json:"glines"`
}

func newRetApiData(mask, reason, ip, msg string, expireTS, lastModTS, hoursUntilExpire int64, active, isReqAccepted bool) *RetApiData {
	return &RetApiData{
		Active:           active,
		Mask:             mask,
		ExpireTS:         expireTS,
		LastModTS:        lastModTS,
		HoursUntilExpire: hoursUntilExpire,
		Reason:           reason,
		Ip:               ip,
		AutoRemove:       isReqAccepted,
		Msg:              msg,
	}
}

type api_struct struct {
	Network  string `param:"network"`
	Ip       string `param:"ip"`
	Nickname string `param:"nickname"`
	RealName string `param:"realname"`
	Email    string `param:"email"`
	Message  string `param:"message"`
}

func Api_init(conf Configuration) *echo.Echo {
	e := echo.New()
	config = conf

	e.Use(middleware.BodyLimit("1K"))
	e.Use(middleware.Logger())
	e.POST("/api/requestrem", requestRemGlineApi)
	e.Use(middleware.Recover())
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper: isAPIOpen,
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == config.ApiKey, nil
		},
	}))
	e.Logger.Fatal(e.Start("127.0.0.1:2001"))
	return e
}

func isAPIOpen(c echo.Context) bool {
	return c.Path() == "/api/requestrem"
}

func requestRemGlineApi(c echo.Context) error {
	var in api_struct
	err := c.Bind(&in)

	if err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	log.Println("ip =", in.Ip, ", net =", in.Network)
	if !slices.Contains(config.Networks, in.Network) {
		return c.JSON(http.StatusNotFound, "Network not found")
	}
	list := make([]*RetApiData, 0, 10)

	RetGlines, err := lookupGlineAPI(in.Ip, in.Network)
	if err != nil {
		fmt.Println("Error:", err)
		return c.JSON(http.StatusBadRequest, "Error in gline lookup")
	}

	if len(RetGlines) == 0 {
		return c.JSON(http.StatusNotFound, "No gline found for that ip address")
	}
	for _, gline := range RetGlines {
		retData := newRetApiData(
			gline.Mask,
			gline.Reason,
			in.Ip,
			"",
			gline.ExpireTS,
			gline.LastModTS,
			gline.HoursUntilExpire,
			gline.Active,
			false,
		)
		autoremove := EvalRequest(retData)
		if autoremove {
			if RemoveGline(gline.Mask) == true {
				retData.Msg = "Your G-line was removed successfully"
			} else {
				retData.Msg = "Error removing G-line. Please contact abuse@undernet.org with this message."
			}
		}
		if retData.Msg == "" {
			retData.Msg = "I don't know what to do with your request. Contact abuse@undernet.org with this message."
		}
		list = append(list, retData)
	}

	return c.JSON(http.StatusOK, &list)
}

// Returns true if the gline is being auto-removed
func EvalRequest(gline *RetApiData) bool {
	if gline.ExpireTS <= time.Now().Unix() {
		gline.Msg = "Gline already expired"
		return false
	} else if !gline.Active {
		gline.Msg = "Gline is not active"
		return false
	}
	for _, rule := range config.Rules {
		matched, err := regexp.MatchString(rule.RegexReason, gline.Reason)
		if err != nil {
			log.Println("Error matching regex:", err)
			gline.Msg = "Error matching regex. Please report to abuse@undernet.org"
			continue
		}
		if matched {
			gline.Msg = "Please contact abuse@undernet.org for this gline"
			fmt.Printf("Debug: Matched rule: %v\n", rule)
			if rule.MustBeSameIP {
				parts := strings.Split(gline.Mask, "@")
				if len(parts) != 2 {
					log.Printf("Error parsing gline mask: %s\n", gline.Mask)
					gline.Msg = "Error parsing gline mask: %s. Please report to abuse@undernet.org"
					return false
				}
				glineIP := parts[1]
				_, cidr, err := net.ParseCIDR(glineIP)
				if err != nil {
					log.Println("Error parsing CIDR:", err)
					gline.Msg = "Error parsing CIDR. Please report to abuse@undernet.org"
					continue
				}
				ip := net.ParseIP(gline.Ip)
				return cidr.Contains(ip) && rule.Autoremove
			} else {
				return rule.Autoremove
			}
		}
	}
	gline.Msg = "No action supported on this app for this gline right now. Contact abuse@undernet.org."
	return false
}

func RemoveGline(glineMask string) bool {
	// Remove the gline
	return true
}

type RetGlineData struct {
	Active           bool   `json:"active"`
	Mask             string `json:"mask"`
	ExpireTS         int64  `json:"expirets"`
	LastModTS        int64  `json:"lastmodts"`
	HoursUntilExpire int64  `json:"hoursuntilexpire"`
	Reason           string `json:"reason"`
}

func lookupGlineAPI(ip, network string) ([]RetGlineData, error) {
	// Define the API endpoint template
	baseURL := "http://127.0.0.1:2000/api2/glinelookup/%s/%s"
	url := fmt.Sprintf(baseURL, network, ip)
	//fmt.Printf("checkgline lookup via URL: %s\n", url)

	// API Key (Bearer token)
	apiKey := config.ApiKey // Replace with your actual API key

	// Create a new HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second, // Set a timeout for the request
	}

	// Create a new HTTP GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add the Bearer token to the Authorization header
	req.Header.Add("Authorization", "Bearer "+apiKey)

	// Perform the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check for a non-200 status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Optionally read response body for debugging
		return nil, fmt.Errorf("API call failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Debug: %s\n", body)
	// Unmarshal JSON response into RetGlineData
	var retGlines []RetGlineData
	err = json.Unmarshal(body, &retGlines)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response JSON: %w", err)
	}

	// Return the parsed data
	return retGlines, nil
}
