package utils

import (
	"context"
	"fmt"

	"github.com/resend/resend-go/v3"
)

type EmailSender struct {
	APIKey string
	From   string
}

func NewEmailSender(apiKey, from string) EmailSender {
	return EmailSender{APIKey: apiKey, From: from}
}

func (s EmailSender) SendOTP(ctx context.Context, to, name, otp string) error {
	if s.APIKey == "" {
		fmt.Printf("OTP untuk %s (%s): %s\n", name, to, otp)
		return nil
	}

	client := resend.NewClient(s.APIKey)
	params := &resend.SendEmailRequest{
		From:    s.From,
		To:      []string{to},
		Subject: "Kode OTP Reset Password",
		Html:    fmt.Sprintf("<p>Halo %s,</p><p>Kode OTP reset password Anda adalah:</p><h2>%s</h2><p>Kode ini hanya berlaku beberapa menit.</p>", name, otp),
	}

	_, err := client.Emails.SendWithContext(ctx, params)
	return err
}
