package abuse_glines

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)

type RecaptchaResponse struct {
	Success     bool     `json:"success"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
}

func (a *ApiData) verifyCaptcha(c echo.Context) error {
	type request struct {
		Token string `json:"token"`
	}

	var req request
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	verifyURL := "https://www.google.com/recaptcha/api/siteverify"
	resp, err := http.PostForm(verifyURL, url.Values{
		"secret":   {a.Config.RecaptchaSecretKey},
		"response": {req.Token},
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Verification failed"})
	}
	defer resp.Body.Close()

	var recaptchaResp RecaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&recaptchaResp); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Invalid response from Google"})
	}

	if !recaptchaResp.Success {
		return c.JSON(http.StatusForbidden, recaptchaResp)
	}

	// Success: Proceed with your logic
	return c.JSON(http.StatusOK, map[string]string{"message": "Captcha verified successfully"})
}

func verifyCaptcha_tmp(c echo.Context, secret string, token string) (bool, error) {
	verifyURL := "https://www.google.com/recaptcha/api/siteverify"
	resp, err := http.PostForm(verifyURL, url.Values{
		"secret":   {secret},
		"response": {token},
	})
	if err != nil {
		return false, c.JSON(http.StatusInternalServerError, map[string]string{"error": "Verification failed"})
	}
	defer resp.Body.Close()

	var recaptchaResp RecaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&recaptchaResp); err != nil {
		return false, c.JSON(http.StatusInternalServerError, map[string]string{"error": "Invalid response from Google"})
	}

	if !recaptchaResp.Success {
		return false, c.JSON(http.StatusForbidden, recaptchaResp)
	}

	// Success: Proceed with your logic
	return true, c.JSON(http.StatusOK, map[string]string{"message": "Captcha verified successfully"})
}
