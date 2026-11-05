package main

import (
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

func sendEmail(to, subject, body string) error {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	username := strings.TrimSpace(os.Getenv("SMTP_EMAIL"))
	password := strings.ReplaceAll(os.Getenv("SMTP_APP_PASSWORD"), " ", "")
	from := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))

	if host == "" || port == "" || username == "" || password == "" || from == "" {
		return fmt.Errorf("SMTP configuration is incomplete")
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("invalid SMTP port")
	}

	if strings.ContainsAny(to+from, "\r\n") ||
		strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("invalid email header")
	}

	auth := smtp.PlainAuth("", username, password, host)
	addr := fmt.Sprintf("%s:%d", host, portNumber)

	message := []byte(
		"From: OpsMind AI <" + from + ">\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" + body + "\r\n",
	)

	return smtp.SendMail(addr, auth, from, []string{to}, message)
}
