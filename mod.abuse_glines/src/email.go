package abuse_glines

import (
	"crypto/tls"
	"fmt"
	"net/mail"

	"gopkg.in/gomail.v2"
)

type SmtpConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

func SendEmail(to, from, replyTo, subject, body string, smtp SmtpConfig, useHTML bool) error {
	return SendEmail_NoStruct(to, from, replyTo, subject, body, smtp.Host, smtp.Port, smtp.User, smtp.Pass, useHTML)
}

func SendEmail_NoStruct(to, from, replyTo, subject, body, smtpHost string, smtpPort int, smtpUser, smtpPass string, useHTML bool) error {
	var err error
	if !IsEmailValid(to) {
		return fmt.Errorf("invalid email address: %s", to)
	}
	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	if replyTo != "" {
		m.SetHeader("Reply-To", replyTo)
	}
	if useHTML {
		m.SetBody("text/html", body)
	} else {
		m.SetBody("text/plain", body)
	}
	d := gomail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPass)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	if err = d.DialAndSend(m); err != nil {
		fmt.Printf("Error in SendEmail(): %s\n", err)
	}
	return err
}

func IsEmailValid(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
