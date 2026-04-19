package notifications

import (
	"context"
	"fmt"

	"github.com/anuragthepathak/subscription-management/internal/core/traceattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	Name         string `mapstructure:"name"`
}

// EmailSender handles email sending operations.
type EmailSender struct {
	config EmailConfig
	dialer *gomail.Dialer
	tracer trace.Tracer
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
		otel.Tracer(config.Name),
	}
}

// SendReminderEmail sends a subscription reminder email.
func (es *EmailSender) SendReminderEmail(
	ctx context.Context,
	toEmail string,
	userName string,
	subscription *models.Subscription,
	daysBefore int,
) error {
	// Check context to allow for cancellation.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Start the child span for the SMTP call
	ctx, span := es.tracer.Start(ctx, "Send Reminder Email",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			traceattr.DaysBefore(daysBefore),
		),
	)
	defer span.End()

	// Get the template
	template := getTemplate(daysBefore)

	// Format price string.
	priceStr := fmt.Sprintf("%s %d (%s)",
		subscription.Currency,
		subscription.Price,
		subscription.Frequency,
	)

	// Create template data.
	data := templateData{
		userName:         userName,
		subscriptionName: subscription.Name,
		renewalDate:      FormatTime(subscription.ValidTill.Local()),
		planName:         subscription.Name,
		price:            priceStr,
		accountURL:       es.config.AccountURL,
		supportURL:       es.config.SupportURL,
		daysLeft:         daysBefore,
	}

	// Generate email content.
	subject := template.generateSubject(data)
	htmlBody := template.generateBody(data)

	// Create email message.
	message := gomail.NewMessage()
	message.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	message.SetHeader("To", toEmail)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", htmlBody)

	// Send the email.
	if err := es.dialer.DialAndSend(message); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to send reminder email")
		return fmt.Errorf("failed to send reminder email: %w", err)
	}

	return nil
}

// SendRenewalConfirmationEmail sends an email notifying a user that their subscription has been automatically renewed
func (es *EmailSender) SendRenewalConfirmationEmail(
	ctx context.Context,
	userEmail string,
	userName string,
	subscription *models.Subscription,
) error {
	// Check context to allow for cancellation.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Start the child span for the SMTP call
	ctx, span := es.tracer.Start(ctx, "Send Renewal Confirmation Email",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

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
	message.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	message.SetHeader("To", userEmail)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", body)

	// Send the email.
	if err := es.dialer.DialAndSend(message); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to send renewal confirmation email")
		return fmt.Errorf("failed to send renewal confirmation email: %w", err)
	}
	return nil
}

// Close cleans up resources if needed.
func (es *EmailSender) Close() error {
	// Nothing to clean up with gomail.
	return nil
}
