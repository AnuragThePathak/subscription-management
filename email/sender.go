package email

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/models"
	"gopkg.in/gomail.v2"
)

// EmailConfig holds email configuration
type EmailConfig struct {
	SMTPHost     string `mapstructure:"smtp_host"`
	SMTPPort     int    `mapstructure:"smtp_port"`
	FromEmail    string `mapstructure:"from_email"`
	FromName     string `mapstructure:"from_name"`
	SMTPUsername string `mapstructure:"smtp_username"`
	SMTPPassword string `mapstructure:"smtp_password"`
	AccountURL   string `mapstructure:"account_url"`
	SupportURL   string `mapstructure:"support_url"`
}

// EmailSender handles email sending operations
type EmailSender struct {
	config EmailConfig
	dialer *gomail.Dialer
}

// NewEmailSender creates a new email service
func NewEmailSender(config EmailConfig) *EmailSender {
	dialer := gomail.NewDialer(
		config.SMTPHost,
		config.SMTPPort,
		config.SMTPUsername,
		config.SMTPPassword,
	)

	return &EmailSender{
		config,
		dialer,
	}
}

// SendReminderEmail sends a subscription reminder email
func (es *EmailSender) SendReminderEmail(ctx context.Context, toEmail string, userName string, subscription *models.Subscription, daysBefore int) error {
	// Check context to allow for cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// Find appropriate template
	templateType, found := FindTemplateByDays(daysBefore)
	if !found {
		return fmt.Errorf("no template found for %d days before reminder", daysBefore)
	}

	// Get the template
	templates := GetTemplates()
	template, exists := templates[templateType]
	if !exists {
		return fmt.Errorf("template not found: %s", templateType)
	}

	// Format price string
	priceStr := fmt.Sprintf("%s %.2f (%s)",
		subscription.Currency,
		subscription.Price,
		subscription.Frequency,
	)

	// Create template data
	data := TemplateData{
		UserName:         userName,
		SubscriptionName: subscription.Name,
		RenewalDate:      FormatTime(subscription.RenewalDate),
		PlanName:         subscription.Name,
		Price:            priceStr,
		PaymentMethod:    subscription.PaymentMethod,
		AccountURL:       es.config.AccountURL,
		SupportURL:       es.config.SupportURL,
		DaysLeft:         daysBefore,
	}

	// Generate email content
	subject := template.GenerateSubject(data)
	htmlBody := template.GenerateBody(data)

	// Create email message
	message := gomail.NewMessage()
	message.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	message.SetHeader("To", toEmail)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", htmlBody)

	// Send the email
	if err := es.dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Log the successful email sending
	slog.Info("Reminder email sent successfully",
		slog.String("component", "email_service"),
		slog.String("to", toEmail),
		slog.String("template", string(templateType)),
		slog.String("subscription", subscription.Name),
	)

	return nil
}

// Close cleans up resources if needed
func (es *EmailSender) Close() error {
	// Nothing to clean up with gomail
	return nil
}
