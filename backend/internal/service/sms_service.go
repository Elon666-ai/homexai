package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SMSService struct {
	accountSID  string
	authToken   string
	phoneNumber string
	client      *http.Client
}

func NewSMSService(accountSID, authToken, phoneNumber string) *SMSService {
	return &SMSService{
		accountSID:  accountSID,
		authToken:   authToken,
		phoneNumber: phoneNumber,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendSMS sends SMS using Twilio API
func (s *SMSService) SendSMS(to, message string) error {
	if s.accountSID == "" || s.authToken == "" {
		// Fallback: log to console for development
		fmt.Printf("📱 SMS to %s: %s\n", to, message)
		return nil
	}

	urlStr := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.accountSID)

	data := url.Values{}
	data.Set("To", to)
	data.Set("From", s.phoneNumber)
	data.Set("Body", message)

	req, err := http.NewRequest("POST", urlStr, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(s.accountSID, s.authToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("twilio returned status %d: %v", resp.StatusCode, result)
	}

	return nil
}

// SendVerificationCode sends verification code via SMS
func (s *SMSService) SendVerificationCode(phone, code, codeType string) error {
	message := fmt.Sprintf(
		"Your HomeX verification code is: %s\n\nThis code will expire in 5 minutes.\n\nIf you didn't request this code, please ignore this message.",
		code,
	)

	return s.SendSMS(phone, message)
}

// SendPasswordResetCode sends password reset code via SMS
func (s *SMSService) SendPasswordResetCode(phone, code string) error {
	message := fmt.Sprintf(
		"Your HomeX password reset code is: %s\n\nThis code will expire in 5 minutes.",
		code,
	)

	return s.SendSMS(phone, message)
}

// SendWelcomeSMS sends welcome SMS
func (s *SMSService) SendWelcomeSMS(phone, fullName string) error {
	message := fmt.Sprintf(
		"Welcome to HomeX, %s! Your property management account is now active. Log in to get started.",
		fullName,
	)

	return s.SendSMS(phone, message)
}
