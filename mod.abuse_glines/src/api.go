package abuse_glines

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/exp/rand"
)

type ApiData struct {
	Config                   Configuration
	EchoInstance             *echo.Echo
	ConfirmEmailMap          map[string]*confirmemail_struct
	confirmEmailMapLastClean int64
	TasksData                *TasksData
	//Captcha                  recaptcha.ReCAPTCHA
}

type confirmemailapi_struct struct {
	ConfirmString string `param:"confirmstring"`
}

type confirmemail_struct struct {
	UUID          string
	Network       string
	IP            string
	EmailAddr     string
	ConfirmString string
	IsSameIP      bool
	ExpireTS      int64
	Glines        []*RetApiData
	Task          *TaskStruct
}

func newConfirmEmailStruct(network, ip, email, uuid_str string) *confirmemail_struct {
	return &confirmemail_struct{
		UUID:          uuid_str,
		Network:       network,
		IP:            ip,
		EmailAddr:     email,
		ConfirmString: RandStringBytesRmndr(128),
		IsSameIP:      false,
		ExpireTS:      time.Now().Unix() + 86400,
		Glines:        nil,
		Task:          nil,
	}
}

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
	IP               string `json:"ip"`
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
		IP:               ip,
		AutoRemove:       isReqAccepted,
		Msg:              msg,
	}
}

type api_struct struct {
	UUID     string `param:"uuid"`
	Network  string `param:"network"`
	IP       string `param:"ip"`
	Nickname string `param:"nickname"`
	RealName string `param:"realname"`
	Email    string `param:"email"`
	Message  string `param:"message"`
}

func Api_init(conf Configuration) *echo.Echo {
	e := echo.New()
	a := &ApiData{
		Config:       conf,
		EchoInstance: e,
		//Captcha:      captcha,
	}
	a.ConfirmEmailMap = make(map[string]*confirmemail_struct)
	//config = conf

	a.TasksData = Tasks_init(86400)
	e.Use(middleware.BodyLimit("1K"))
	e.Use(middleware.Logger())
	e.POST("/api/requestrem", a.requestRemGlineApi)
	e.GET("/confirmemail/:confirmstring", a.confirmEmailAPI)
	e.GET("/tasks/:uuid", a.TasksData.GetTasksStatus_api)
	e.Use(middleware.Recover())
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper: isAPIOpen,
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == conf.ApiKey, nil
		},
	}))
	e.Logger.Fatal(e.Start("127.0.0.1:2001"))
	return e
}

func isAPIOpen(c echo.Context) bool {
	return c.Path() == "/api/requestrem"
}

func (a *ApiData) requestRemGlineApi(c echo.Context) error {
	var in api_struct
	err := c.Bind(&in)

	if err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	log.Println("ip =", in.IP, ", net =", in.Network)
	if !slices.Contains(a.Config.Networks, strings.ToLower(in.Network)) {
		return c.JSON(http.StatusNotFound, "Network not found")
	}

	RetGlines, err := a.lookupGlineAPI(in.IP, in.Network)
	if err != nil {
		fmt.Println("Error:", err)
		return c.JSON(http.StatusBadRequest, "Error in gline lookup")
	}

	if len(RetGlines) == 0 {
		return c.JSON(http.StatusNotFound, "No gline found for that ip address")
	}
	var UUID string
	if in.UUID == "" {
		UUID = uuid.NewString()
		log.Printf("Generating new UUID for email confirmation: %s\n", UUID)
	} else {
		UUID = in.UUID
	}

	emailConfirmed := false
	for _, task := range a.TasksData.GetTasks(UUID) {
		if task.TaskType == "confirmemail" {
			if task.Data.(*confirmemail_struct).EmailAddr == in.Email {
				if task.CompletedTS > 0 && time.Now().Unix()-task.CompletedTS < 86400 {
					emailConfirmed = true
					break
				}
			}
		}
	}
	if !emailConfirmed {
		ce := newConfirmEmailStruct(in.Network, in.IP, in.Email, UUID)
		confirmLink := fmt.Sprintf("/confirmemail/%s", url.PathEscape(ce.ConfirmString))
		ce.Task = a.TasksData.AddTask(UUID, "confirmemail")
		ce.Task.SetData(ce)
		body := fmt.Sprintf("Hi,\n\nIn order to complete the gline removal request on %s, you need to click this link: %s\n\nAbuse's self gline-removal system", in.Network, confirmLink)
		if !IsEmailValid(ce.EmailAddr) {
			log.Printf("Invalid email address: %s\n", ce.EmailAddr)
			return c.JSON(http.StatusBadRequest, fmt.Sprintf("Invalid email address: %s", ce.EmailAddr))
		}
		go func() {
			ce.Task.Start()
			ce.Task.SetProgress(20)
			err = SendEmail(ce.EmailAddr, a.Config.FromEmail, "Email confirmation required", body, a.Config.Smtp, false)
			if err == nil {
				a.ConfirmEmailMap[ce.ConfirmString] = ce
				ce.Task.Done()
			} else {
				log.Printf("Error sending email: %s\n", err)
				ce.Task.SetMessage(fmt.Sprintf("Error sending confirmation email to %s. Please try again later or email %s with this message if it fails again.", ce.EmailAddr, a.Config.AbuseEmail))
				ce.Task.Cancel()
			}
		}()
		return c.JSON(http.StatusOK, fmt.Sprintf("Sending email... Check your inbox for %s", ce.EmailAddr))
	} else {
		// Email is confirmed
		list := make([]*RetApiData, 0, 10)
		for _, gline := range RetGlines {
			retData := newRetApiData(
				gline.Mask,
				gline.Reason,
				in.IP,
				"",
				gline.ExpireTS,
				gline.LastModTS,
				gline.HoursUntilExpire,
				gline.Active,
				false,
			)
			autoremove := a.EvalRequest(retData)
			if autoremove {
				if a.RemoveGline(in.Network, gline.Mask) {
					retData.Msg = "Your G-line was removed successfully"
				} else {
					retData.Msg = fmt.Sprintf("Error removing G-line. Please contact %s with this message.", a.Config.AbuseEmail)
				}
			}
			if retData.Msg == "" {
				retData.Msg = fmt.Sprintf("I don't know what to do with your request. Contact %s with this message.", a.Config.AbuseEmail)
			}
			list = append(list, retData)
		}

		return c.JSON(http.StatusOK, &list)
	}
}

func (a *ApiData) confirmEmailAPI(c echo.Context) error {
	var in confirmemailapi_struct
	err := c.Bind(&in)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Bad request")
	}

	if a.IsTimeToCheckExpiredEntries() {
		for i_cs, i_ce := range a.ConfirmEmailMap {
			if i_ce.Expired() {
				delete(a.ConfirmEmailMap, i_cs)
			}
		}
	}
	ce, ok := a.ConfirmEmailMap[in.ConfirmString]
	if !ok {
		return c.JSON(http.StatusNotImplemented, fmt.Sprintf("Confirm string not found or expired. Please contact %s.", a.Config.AbuseEmail))
	}
	// Necessary, in case a.IsTimeToCheckExpiredEntries() returned false but that this entry is still expired
	if ce.Expired() {
		delete(a.ConfirmEmailMap, in.ConfirmString)
		return c.JSON(http.StatusNotImplemented, fmt.Sprintf("Confirm string not found or expired. Please contact %s.", a.Config.AbuseEmail))
	}
	if !slices.Contains(a.Config.Networks, strings.ToLower(ce.Network)) {
		return c.JSON(http.StatusNotFound, "Network not found")
	}

	ce.Task.SetMessage("Email confirmed.")
	ce.Task.Done()
	return c.HTML(http.StatusOK, "Your email is confirmed.<br><br>You can close this tab and go back to the abuse-glines web application.")
}

// Returns true if the gline is being auto-removed
func (a *ApiData) EvalRequest(gline *RetApiData) bool {
	if gline.ExpireTS <= time.Now().Unix() {
		gline.Msg = "Gline already expired"
		return false
	} else if !gline.Active {
		gline.Msg = "Gline is not active"
		return false
	}
	for _, rule := range a.Config.Rules {
		matched, err := regexp.MatchString(rule.RegexReason, gline.Reason)
		if err != nil {
			log.Println("Error matching regex:", err)
			gline.Msg = fmt.Sprintf("Error matching regex. Please report to %s", a.Config.AbuseEmail)
			continue
		}
		if matched {
			gline.Msg = fmt.Sprintf("Please contact %s for this gline", a.Config.AbuseEmail)
			fmt.Printf("Debug: Matched rule: %v\n", rule)
			if rule.MustBeSameIP {
				parts := strings.Split(gline.Mask, "@")
				if len(parts) != 2 {
					log.Printf("Error parsing gline mask: %s\n", gline.Mask)
					gline.Msg = fmt.Sprintf("Error parsing gline mask: %s. Please report to %s", gline.Mask, a.Config.AbuseEmail)
					return false
				}
				glineIP := parts[1]
				_, cidr, err := net.ParseCIDR(glineIP)
				if err != nil {
					log.Println("Error parsing CIDR:", err)
					gline.Msg = fmt.Sprintf("Error parsing CIDR. Please report to %s", a.Config.AbuseEmail)
					continue
				}
				ip := net.ParseIP(gline.IP)
				return cidr.Contains(ip) && rule.Autoremove
			} else {
				return rule.Autoremove
			}
		}
	}
	gline.Msg = fmt.Sprintf("No action supported on this app for this gline right now. Contact %s.", a.Config.AbuseEmail)
	return false
}

func (a *ApiData) RemoveGline(network, glineMask string) bool {
	// Remove the gline
	// Define the API endpoint template
	baseURL := "http://127.0.0.1:2000/api2/remgline/%s"
	url := fmt.Sprintf(baseURL, network)

	// API Key (Bearer token)
	apiKey := a.Config.ApiKey

	// Create a new HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second, // Set a timeout for the request
	}

	// Create the request body
	requestBody, err := json.Marshal(map[string]string{
		"glinemask": glineMask,
		"network":   network,
	})
	if err != nil {
		log.Println("Error marshalling request body:", err)
		return false
	}

	// Create a new HTTP POST request
	req, err := http.NewRequest("POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		log.Println("Failed to create HTTP request:", err)
		return false
	}

	// Add headers
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+apiKey)

	// Perform the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Failed to make HTTP request:", err)
		return false
	}
	defer resp.Body.Close()

	// Check for a non-200 status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Optionally read response body for debugging
		log.Printf("API call failed with status %d: %s", resp.StatusCode, string(body))
		return false
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read response body:", err)
		return false
	}

	fmt.Printf("Debug: %s\n", body)
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

func (a *ApiData) lookupGlineAPI(ip, network string) ([]RetGlineData, error) {
	// Define the API endpoint template
	baseURL := "http://127.0.0.1:2000/api2/glinelookup/%s/%s"
	url := fmt.Sprintf(baseURL, network, ip)
	//fmt.Printf("checkgline lookup via URL: %s\n", url)

	// API Key (Bearer token)
	apiKey := a.Config.ApiKey // Replace with your actual API key

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

// Credit for this function: icza on https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func RandStringBytesRmndr(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

func (ce *confirmemail_struct) Expired() bool {
	return ce.ExpireTS > time.Now().Unix()
}

func (a *ApiData) IsTimeToCheckExpiredEntries() bool {
	return (a.confirmEmailMapLastClean - time.Now().Unix()) > 600
}
