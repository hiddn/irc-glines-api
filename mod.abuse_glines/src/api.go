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
	RegexReason     string `json:"regexreason"`
	MustBeSameIP    bool   `json:"mustbesameip"`
	Autoremove      bool   `json:"autoremove"`
	NeverEmailAbuse bool   `json:"neveremailabuse"`
	Message         string `json:"message"`
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
	Message          string `json:"message"`
}

type RetApiDatas struct {
	RetApiData []RetApiData `json:"glines"`
}

func newRetApiData(mask, reason, ip, message string, expireTS, lastModTS, hoursUntilExpire int64, active, isReqAccepted bool) *RetApiData {
	return &RetApiData{
		Active:           active,
		Mask:             mask,
		ExpireTS:         expireTS,
		LastModTS:        lastModTS,
		HoursUntilExpire: hoursUntilExpire,
		Reason:           reason,
		IP:               ip,
		AutoRemove:       isReqAccepted,
		Message:          message,
	}
}

type api_requestrem_struct struct {
	UUID           string `json:"uuid"`
	Network        string `json:"network"`
	IP             string `json:"ip"`
	Nickname       string `json:"nickname"`
	RealName       string `json:"realname"`
	Email          string `json:"email"`
	UserMessage    string `json:"user_message"`
	RecaptchaToken string `json:"recaptcha_token"`
}

type api_requestrem_ret_struct struct {
	UUID                string        `json:"uuid"`
	Network             string        `json:"network"`
	IP                  string        `json:"ip"`
	Message             string        `json:"message"`
	RequestSentViaEmail bool          `json:"request_sent_via_email"`
	Glines              []*RetApiData `json:"glines"`
}

func Api_init(conf Configuration) *echo.Echo {
	if conf.Testmode {
		conf.AbuseEmail = conf.TestEmail
	}
	e := echo.New()
	a := &ApiData{
		Config:       conf,
		EchoInstance: e,
		//Captcha:      captcha,
	}
	a.ConfirmEmailMap = make(map[string]*confirmemail_struct)
	//config = conf

	a.TasksData = Tasks_init(86400)
	e.Use(middleware.BodyLimit("4K"))
	e.Use(middleware.Logger())
	e.GET("/api/confirmemail/:confirmstring", a.confirmEmailAPIGet)
	e.GET("/api/tasks/:uuid", a.TasksData.GetTasksStatus_api)
	e.POST("/api/requestrem", a.requestRemGlineApi)
	e.POST("/api/confirmemail/:confirmstring", a.confirmEmailAPI)
	e.POST("/api/verify-captcha", a.verifyCaptcha)
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
	switch c.Path() {
	case "/api/requestrem":
		return true
	case "/api/confirmemail/:confirmstring":
		return true
	case "/api/tasks/:uuid":
		return true
	default:
		return false
	}
}

func (a *ApiData) requestRemGlineApi(c echo.Context) error {
	var in api_requestrem_struct
	var emailToAbuseRequired bool

	err := c.Bind(&in)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "bad request")
	}
	webIP, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		return err
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

	ret := &api_requestrem_ret_struct{
		UUID:                in.UUID,
		Network:             in.Network,
		IP:                  in.IP,
		RequestSentViaEmail: false,
		Glines:              []*RetApiData{},
	}

	var UUID string
	if in.UUID == "" {
		UUID = uuid.NewString()
		log.Printf("Generating new UUID for email confirmation: %s\n", UUID)
	} else {
		UUID = in.UUID
	}
	ret.UUID = UUID

	if a.Config.Testmode {
		in.Email = a.Config.TestEmail
	}
	emailConfirmed := false
	email := ""
	for _, task := range a.TasksData.GetAllTasksByUUID(UUID) {
		if task.TaskType == "confirmemail" {
			fmt.Printf("Debug: email = %v\nDebug: task.Data = %v\n", in.Email, task.Data)
			if task.Data.(*confirmemail_struct).EmailAddr == in.Email {
				if !task.IsExpired() && task.Progress == 100 {
					fmt.Printf("Debug: Confirmed email found: %s\n", in.Email)
					emailConfirmed = true
					email = task.DataVisibleToUser
					break
				}
			}
		}
	}
	if !emailConfirmed {
		/*
			if recaptchaSuccess, err := verifyCaptcha(c, a.Config.RecaptchaSecretKey, in.RecaptchaToken); !recaptchaSuccess {
				return err
			}
		*/
		ce := newConfirmEmailStruct(in.Network, in.IP, in.Email, UUID)
		confirmLink := fmt.Sprintf("%s/api/confirmemail/%s", a.Config.URL, url.PathEscape(ce.ConfirmString))
		ce.Task = a.TasksData.AddTask(UUID, "confirmemail")
		ce.Task.SetData(ce)
		ce.Task.DataVisibleToUser = in.Email
		body := fmt.Sprintf("Hi,\n\nIn order to complete the gline removal request on %s, you need to click this link: %s\n\nAbuse's self gline-removal system", in.Network, confirmLink)
		if !IsEmailValid(ce.EmailAddr) {
			log.Printf("Invalid email address: %s\n", ce.EmailAddr)
			return c.JSON(http.StatusBadRequest, fmt.Sprintf("Invalid email address: %s", ce.EmailAddr))
		}
		go func() {
			ce.Task.Start()
			err = SendEmail(ce.EmailAddr, a.Config.FromEmail, "", "Email confirmation required", body, a.Config.Smtp, false)
			if err == nil {
				a.ConfirmEmailMap[ce.ConfirmString] = ce
				ce.Task.SetProgress(50, "Email confirmation sent")
			} else {
				log.Printf("Error sending email: %s\n", err)
				ce.Task.Cancel(fmt.Sprintf("Error sending confirmation email to %s. Please try again later or email %s with this message if it fails again.", ce.EmailAddr, a.Config.AbuseEmail))
			}
		}()
		ret.Message = fmt.Sprintf("Sending email... Check your inbox (%s).", ce.EmailAddr)
		return c.JSON(http.StatusAccepted, ret)
	} else {
		// Email is confirmed. Overwrite the user-supplied email with the one that was confirmed before
		in.Email = email
		emailToAbuseRequired = false
		list := make([]*RetApiData, 0, len(RetGlines))
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
			var autoremove bool
			var emailToAbuseRequiredForThisGline bool
			autoremove, emailToAbuseRequiredForThisGline, retData.Message = a.EvalGlineRules(retData, webIP)
			if emailToAbuseRequiredForThisGline {
				emailToAbuseRequired = true
			}
			if autoremove {
				broadcast_message := fmt.Sprintf("Auto-removed G-line on %s | email: %s | nick: %s | name: %s | Message: %s", gline.Mask, in.Email, in.Nickname, in.RealName, in.UserMessage)
				if a.RemoveGline(in.Network, gline.Mask, broadcast_message) {
					retData.Message = "Your G-line was removed successfully"
				} else {
					emailToAbuseRequired = true
					retData.Message = fmt.Sprintf("Error removing G-line. Please contact %s with this message.", a.Config.AbuseEmail)
				}
			}
			if retData.Message == "" {
				retData.Message = fmt.Sprintf("I don't know what to do with your request. Contact %s with this message.", a.Config.AbuseEmail)
			}
			list = append(list, retData)
		}
		if emailToAbuseRequired {
			fmt.Printf("Debug: Emailing abuse for %s\n", in.IP)
			emailContent := a.PrepareAbuseEmail(list, webIP, &in)
			err = SendEmail(a.Config.AbuseEmail, a.Config.FromEmail, in.Email, "G-line removal request", emailContent, a.Config.Smtp, true)
			if err != nil {
				log.Printf("Error sending email to abuse: %s\n", err)
			}
		}
		ret.Glines = list
		ret.RequestSentViaEmail = emailToAbuseRequired
		return c.JSON(http.StatusOK, ret)
	}
}

// Returns true if the gline is being auto-removed
func (a *ApiData) EvalGlineRules(gline *RetApiData, webIP string) (autoremove bool, emailToAbuseRequired bool, message string) {
	var isGlineActive bool = true
	autoremove = false
	emailToAbuseRequired = true
	message = ""
	if gline.ExpireTS <= time.Now().Unix() {
		message = "Gline already expired"
		isGlineActive = false
	} else if !gline.Active {
		message = "Gline is not active"
		isGlineActive = false
	}
	if !isGlineActive {
		return false, false, message
	}
	for _, rule := range a.Config.Rules {
		matched, err := regexp.MatchString(rule.RegexReason, gline.Reason)
		if err != nil {
			log.Println("Error matching regex:", err)
			message = fmt.Sprintf("Error matching regex. Please report to %s", a.Config.AbuseEmail)
			continue
		}
		if matched {
			fmt.Printf("Debug: Matched rule: %v\n", rule)
			message = rule.Message
			emailToAbuseRequired = !rule.NeverEmailAbuse && !autoremove
			if rule.MustBeSameIP {
				parts := strings.Split(gline.Mask, "@")
				if len(parts) != 2 {
					log.Printf("Error parsing gline mask: %s\n", gline.Mask)
					message = fmt.Sprintf("Error parsing gline mask: %s. Please report to %s", gline.Mask, a.Config.AbuseEmail)
					autoremove = false
					emailToAbuseRequired = !rule.NeverEmailAbuse && !autoremove
					return
				}
				glineIP := parts[1]
				_, cidr, err := net.ParseCIDR(glineIP)
				if err != nil {
					log.Println("Error parsing CIDR:", err)
					message = fmt.Sprintf("Error parsing CIDR. Please report to %s", a.Config.AbuseEmail)
					continue
				}
				webIP_net := net.ParseIP(webIP)
				autoremove = cidr.Contains(webIP_net) && rule.Autoremove
				emailToAbuseRequired = !rule.NeverEmailAbuse && !autoremove
				return
			} else {
				autoremove = rule.Autoremove
				emailToAbuseRequired = !rule.NeverEmailAbuse && !autoremove
				return
			}
		}
	}
	if message == "" {
		message = fmt.Sprintf("No action supported on this app for this gline right now. Contact %s.", a.Config.AbuseEmail)
	}
	autoremove = false
	return
}

func (a *ApiData) PrepareAbuseEmail(list []*RetApiData, webIP string, in *api_requestrem_struct) string {
	var pluralStr string = ""
	if len(list) > 1 {
		pluralStr = "es"
	}
	emailContent := fmt.Sprintf(`
		<div>
		<h2>User infos: </h2>
		<table style="color:black; margin-left: 0.5rem; border-spacing: 0.2rem 0.2rem;">
			<tr>
			<td style="font-weight: bold;">Nick:</td>
			<td>%s</td>
			</tr>
			<tr>
			<td style="font-weight: bold;">Name:</td>
			<td>%s</td>
			</tr>
			<tr>
			<td style="font-weight: bold;">Email:</td>
			<td>%s</td>
			</tr>
			<tr>
			<td style="font-weight: bold;">WebIP:</td>
			<td>%s</td>
			</tr>
			<tr>
			<td style="font-weight: bold;">SearchIP:</td>
			<td>%s</td>
			</tr>
			<tr>
			<td style="font-weight: bold;">Message:</td>
			<td>%s</td>
			</tr>
		</table>
		</div>`, in.Nickname, in.RealName, in.Email, webIP, in.IP, strings.ReplaceAll(in.UserMessage, "\n", "<br>"))
	emailContent += fmt.Sprintf(`
		<table style="max-width: 500px; margin: 0 0; padding: 0 0;">
		<tr>
			<td>
			<div style="max-width: 2xl; margin: 0; margin-bottom: 2rem;">
				<span style="display: block; text-align: left; margin-bottom: 1rem; margin-top: 3rem; font-weight: bold; font-size: 1.25rem;">
				  <a href=\"%v\">G-line match%s for %v</a>:
				</span>`, fmt.Sprintf("%s?ip=%s", a.Config.URL, in.IP), pluralStr, in.IP)
	for _, gline := range list {
		emailContent += fmt.Sprintf(`
				<!-- G-line item template - repeat for each g-line -->
				<div style="background-color: rgb(29, 28, 28); text-align: left; margin: 1rem 0; padding: 0.5rem; border-radius: 0.5rem;">
				<div>
					<span style="margin-left: 0.5rem; color: lightseagreen; font-size: 1.25rem;">%v</span>
				</div>
				<table style="color:gray; margin-left: 0.5rem; border-spacing: 1rem 1rem;">
					<tr>
					<td style="font-weight: bold;">Reason:</td>
					<td>%v</td>
					</tr>
					<tr>
					<td style="font-weight: bold;">Expiration:</td>
					<td>%v</td>
					</tr>
				</table>
				<div style="color: black; background-color: yellow; padding: 0.5rem; border-radius: 0.25rem;">
					%v
				</div>
				</div>`, gline.Mask, gline.Reason, getExpireTSString(gline), gline.Message)
	}
	return emailContent
}

func getExpireTSString(gline *RetApiData) string {
	if !gline.Active {
		return `<span style="color: green;">Deactivated</span>`
	}

	now := time.Now().Unix()
	isExpired := gline.ExpireTS <= now

	exp := time.Unix(gline.ExpireTS, 0).UTC().Format(time.UnixDate) + "<br/>"

	if isExpired {
		exp += `<b><span style="color: green;">EXPIRED</span>: `
	} else {
		exp += "(<b>in "
	}

	duration := time.Duration(gline.ExpireTS-time.Now().Unix()) * time.Second
	exp = fmt.Sprintf("%s%v", exp, duration)

	if isExpired {
		exp += "</b> ago"
	} else {
		exp += "</b>)"
	}

	return exp
}

func (a *ApiData) confirmEmailAPIGet(c echo.Context) error {
	var in confirmemailapi_struct
	err := c.Bind(&in)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Bad request")
	}

	htmlForm := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Email Confirmation</title>
		</head>
		<body>
			<h2>Email Confirmation</h2>
			<form action="/api/confirmemail/%s" method="post">
				<input type="hidden" name="confirmstring" value="%s">
				<p>Click the button below to confirm your email:</p>
				<button type="submit">Confirm Email</button>
			</form>
		</body>
		</html>
	`, in.ConfirmString, in.ConfirmString)

	return c.HTML(http.StatusOK, htmlForm)
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
		return c.JSON(http.StatusNotImplemented, "Confirm string not found or expired. Try again.")
	}
	// Necessary, in case a.IsTimeToCheckExpiredEntries() returned false but that this entry is still expired
	if ce.Expired() {
		delete(a.ConfirmEmailMap, in.ConfirmString)
		return c.JSON(http.StatusNotImplemented, "Confirm string not found or expired. Try again.")
	}
	if !slices.Contains(a.Config.Networks, strings.ToLower(ce.Network)) {
		return c.JSON(http.StatusNotFound, "Network not found")
	}

	// Only keep one completed confirmed email task active at all times
	for _, task := range a.TasksData.GetAllTasksByUUID(ce.Task.UUID) {
		if task.TaskType == "confirmemail" {
			if task.TaskID != ce.Task.TaskID {
				task.Cancel("Email confirmation activated for taskid " + ce.Task.TaskID + "and email " + ce.EmailAddr)
			}
		}
	}

	ce.Task.Done("Email confirmed.")
	return c.HTML(http.StatusOK, "Your email is confirmed.<br><br>You can close this tab and go back to the abuse-glines web application.")
}

func (a *ApiData) RemoveGline(network, glineMask, message string) bool {
	// Remove the gline
	// Define the API endpoint template
	if a.Config.Testmode {
		return true
	}
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
		"message":   message,
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
	return ce.ExpireTS < time.Now().Unix()
}

func (a *ApiData) IsTimeToCheckExpiredEntries() bool {
	return (a.confirmEmailMapLastClean - time.Now().Unix()) > 600
}
