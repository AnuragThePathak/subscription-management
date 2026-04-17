package notifications

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/core/traceattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"go.opentelemetry.io/otel"
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
func (es *EmailSender) SendReminderEmail(ctx context.Context, toEmail string, userName string, subscription *models.Subscription, daysBefore int) error {
	// Check context to allow for cancellation.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Start the child span for the SMTP call
	ctx, span := es.tracer.Start(ctx, "Send Reminder Email",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			traceattr.EmailDaysBefore(daysBefore),
		),
	)
	defer span.End()
	observability.EnrichSpan(ctx)

	// Find appropriate template.
	templateType := FindTemplateByDays(daysBefore)

	// Get the template.
	templates := GetTemplates()
	template, exists := templates[templateType]
	if !exists {
		return fmt.Errorf("template not found: %s", templateType)
	}

	// Format price string.
	priceStr := fmt.Sprintf("%s %d (%s)",
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
	slog.InfoContext(ctx, "Reminder email sent",
		logattr.Template(string(templateType)),
		logattr.SubscriptionName(subscription.Name),
	)

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
	observability.EnrichSpan(ctx)

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
		return fmt.Errorf("failed to send renewal confirmation email: %w", err)
	}
	// Log the successful email sending.
	slog.InfoContext(ctx, "Renewal confirmation email sent",
		logattr.SubscriptionName(subscription.Name),
	)
	return nil
}

// Close cleans up resources if needed.
func (es *EmailSender) Close() error {
	// Nothing to clean up with gomail.
	return nil
}
