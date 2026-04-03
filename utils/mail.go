package utils

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

type MailConfig struct {
	Host     string
	Port     int
	Email    string
	Password string
	From     string
}

func LoadMailConfig() (MailConfig, error) {
	port, err := strconv.Atoi(os.Getenv("MAIL_PORT"))
	if err != nil {
		return MailConfig{}, fmt.Errorf("invalid MAIL_PORT: %w", err)
	}

	config := MailConfig{
		Host:     os.Getenv("MAIL_HOST"),
		Port:     port,
		Email:    os.Getenv("MAIL_USER"),
		Password: os.Getenv("MAIL_PASS"),
		From:     os.Getenv("MAIL_FROM"),
	}

	if config.Host == "" || config.Email == "" || config.Password == "" || config.From == "" {
		return MailConfig{}, errors.New("mail config is incomplete")
	}

	return config, nil
}

func (config MailConfig) SendForgotPasswordEmail(toEmail string, resetLink string) error {
	mail := gomail.NewMessage()

	mail.SetHeader("From", config.From)
	mail.SetHeader("To", toEmail)
	mail.SetHeader("Subject", "7Planner | Reset your password")

	htmlBody := fmt.Sprintf(`
    <div style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: auto; border: 1px solid #eee; border-radius: 10px; padding: 20px; color: #333;">
        <div style="text-align: center; margin-bottom: 20px;">
            <h1 style="color: #2d3436; margin: 0;">7Planner</h1>
        </div>
        
        <h2 style="font-size: 1.2em; color: #2d3436;">Password Reset Request</h2>
        <p>We received a request to reset the password for your account. No changes have been made yet.</p>
        
        <div style="text-align: center; margin: 35px 0;">
            <a href="%s" 
               style="background-color: #3399ff; color: white; padding: 14px 30px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block; box-shadow: 0 4px 6px rgba(0,0,0,0.1);"
               target="_blank">
               Reset My Password
            </a>
        </div>
        
        <p style="font-size: 0.9em; color: #636e72; line-height: 1.6;">
            If you did not request a password reset, please ignore this email or reply to let us know. This link is only valid for the next 15 minutes.
        </p>
        
        <div style="margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 0.8em; color: #b2bec3; text-align: center;">
            <p>Sent with 💙 by 7Planner Team</p>
        </div>
    </div>`, resetLink)

	plainBody := fmt.Sprintf(
		"We received a request to reset your password.\n\nReset link: %s\n\nIf you did not request this, you can ignore this email.",
		resetLink,
	)

	mail.SetBody("text/plain", plainBody)
	mail.AddAlternative("text/html", htmlBody)

	d := gomail.NewDialer(
		config.Host,
		config.Port,
		config.Email,
		config.Password,
	)

	return d.DialAndSend(mail)
}

func (config MailConfig) SendVerificationEmail(toEmail string, verificationLink string) error {
	mail := gomail.NewMessage()

	mail.SetHeader("From", config.From)
	mail.SetHeader("To", toEmail)
	mail.SetHeader("Subject", "7Planner | Verificate your account")

	htmlBody := fmt.Sprintf(`
    <div style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: auto; border: 1px solid #eee; border-radius: 10px; padding: 20px; color: #333;">
        <div style="text-align: center; margin-bottom: 20px;">
            <h1 style="color: #3399ff; margin: 0;">7Planner</h1>
        </div>
        
        <h2 style="font-size: 1.2em; color: #2d3436;">Welcome to 7Planner</h2>
        <p>Thanks for signing up. To get started, please verify your email address to activate your account.</p>
        
        <div style="text-align: center; margin: 35px 0;">
            <a href="%s" 
               style="background-color: #3399ff; color: white; padding: 14px 30px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block; box-shadow: 0 4px 6px rgba(0,0,0,0.1);"
               target="_blank">
               Verify My Account
            </a>
        </div>
        
        <p style="font-size: 0.9em; color: #636e72; line-height: 1.6;">
            If you did not create an account on 7Planner, you can safely ignore this email.
        </p>
        <p style="font-size: 0.8em; color: #999;">This link will expire in 24 hours.</p>
        
        <div style="margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 0.8em; color: #b2bec3; text-align: center;">
            <p>Sent with 💙 by 7Planner Team</p>
        </div>
    </div>`, verificationLink)

	plainBody := fmt.Sprintf(
		"Welcome to 7Planner!\n\nPlease verify your account by clicking the link below:\n\n%s\n\nIf you did not create an account, please ignore this email.",
		verificationLink,
	)

	mail.SetBody("text/plain", plainBody)
	mail.AddAlternative("text/html", htmlBody)

	d := gomail.NewDialer(
		config.Host,
		config.Port,
		config.Email,
		config.Password,
	)

	return d.DialAndSend(mail)
}
