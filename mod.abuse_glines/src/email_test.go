package abuse_glines

import "testing"

var config Configuration = ReadConf("../config.json")

// func SendEmail(to, from, subject, body string, smtp SmtpConfig, useHTML bool) error {
func TestSendEmail(t *testing.T) {
	to := config.TestEmail
	from := config.FromEmail
	subject := "Test email"
	body := "This is a test email"
	smtp := config.Smtp
	useHTML := false
	err := SendEmail(to, from, subject, body, smtp, useHTML)
	if err != nil {
		t.Errorf("SendEmail() failed: %s", err)
	}
}
