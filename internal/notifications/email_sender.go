package notifications

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"gopkg.in/gomail.v2"
)

// EmailConfig holds email configuration.
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

// EmailSender handles email sending operations.
type EmailSender struct {
	config EmailConfig
	dialer *gomail.Dialer
}

// NewEmailSender creates a new email service.
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

// SendReminderEmail sends a subscription reminder email.
func (es *EmailSender) SendReminderEmail(ctx context.Context, toEmail string, userName string, subscription *models.Subscription, daysBefore int) error {
	// Check context to allow for cancellation.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Find appropriate template.
	templateType, found := FindTemplateByDays(daysBefore)
	if !found {
		return fmt.Errorf("no template found for %d days before reminder", daysBefore)
	}

	// Get the template.
	templates := GetTemplates()
	template, exists := templates[templateType]
	if !exists {
		return fmt.Errorf("template not found: %s", templateType)
	}

	// Format price string.
	priceStr := fmt.Sprintf("%s %.2d (%s)",
		subscription.Currency,
		subscription.Price,
		subscription.Frequency,
	)

	// Create template data.
	data := TemplateData{
		UserName:         userName,
		SubscriptionName: subscription.Name,
		RenewalDate:      FormatTime(subscription.ValidTill.Local()),
		PlanName:         subscription.Name,
		Price:            priceStr,
		AccountURL:       es.config.AccountURL,
		SupportURL:       es.config.SupportURL,
		DaysLeft:         daysBefore,
	}

	// Generate email content.
	subject := template.GenerateSubject(data)
	htmlBody := template.GenerateBody(data)

	// Create email message.
	message := gomail.NewMessage()
	message.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	message.SetHeader("To", toEmail)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", htmlBody)

	// Send the email.
	if err := es.dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Log the successful email sending.
	slog.Info("Reminder email sent successfully",
		slog.String("component", "email_service"),
		slog.String("to", toEmail),
		slog.String("template", string(templateType)),
		slog.String("subscription", subscription.Name),
	)

	return nil
}

// SendRenewalConfirmationEmail sends an email notifying a user that their subscription has been automatically renewed
func (e *EmailSender) SendRenewalConfirmationEmail(
	ctx context.Context,
	userEmail string,
	userName string,
	subscription *models.Subscription,
) error {
	// Check context to allow for cancellation.
	if err := ctx.Err(); err != nil {
		return err
	}

	subject := fmt.Sprintf("Your %s subscription has been renewed", subscription.Name)
	renewalAmount := fmt.Sprintf("%d %s", subscription.Price, subscription.Currency)
	// Format the email body
	body := fmt.Sprintf(`
	Hello %s,
	
	Your subscription to %s has been automatically renewed.
	
	Subscription Details:
	- Name: %s
	- Amount: %s
	- Valid Till: %s
	
	If you did not want this renewal, you can cancel your subscription through your account.
	
	Thank you for your continued subscription!
	
	Best regards,
	The Subscription Management Team
	`,
		userName,
		subscription.Name,
		subscription.Name,
		renewalAmount,
		subscription.ValidTill.Format("January 2, 2006"),
	)

	// Create the email message.
	message := gomail.NewMessage()
	message.SetHeader("From", fmt.Sprintf("%s <%s>", e.config.FromName, e.config.FromEmail))
	message.SetHeader("To", userEmail)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", body)

	// Send the email.
	if err := e.dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("failed to send renewal confirmation email: %w", err)
	}
	// Log the successful email sending.
	slog.Info("Renewal confirmation email sent successfully",
		slog.String("component", "email_service"),
		slog.String("to", userEmail),
		slog.String("subscription", subscription.Name),
	)
	return nil
}

// Close cleans up resources if needed.
func (es *EmailSender) Close() error {
	// Nothing to clean up with gomail.
	return nil
}
