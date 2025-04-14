package email

import (
	"fmt"
	"time"
)

// TemplateType represents different email template types.
type TemplateType string

const (
	SevenDaysReminder TemplateType = "7 days before reminder"
	FiveDaysReminder  TemplateType = "5 days before reminder"
	TwoDaysReminder   TemplateType = "2 days before reminder"
	OneDayReminder    TemplateType = "1 day before reminder"
)

// EmailTemplate represents an email template with subject and body generators.
type EmailTemplate struct {
	Label           string
	GenerateSubject func(TemplateData) string
	GenerateBody    func(TemplateData) string
}

// TemplateData contains all data needed for email templates.
type TemplateData struct {
	UserName         string
	SubscriptionName string
	RenewalDate      string
	PlanName         string
	Price            string
	PaymentMethod    string
	AccountURL       string
	SupportURL       string
	DaysLeft         int
}

// GetTemplates returns all available email templates.
func GetTemplates() map[TemplateType]EmailTemplate {
	return map[TemplateType]EmailTemplate{
		SevenDaysReminder: {
			Label: "7 days before reminder",
			GenerateSubject: func(data TemplateData) string {
				return fmt.Sprintf("📅 Reminder: Your %s Subscription Renews in 7 Days!", data.SubscriptionName)
			},
			GenerateBody: func(data TemplateData) string {
				return generateEmailTemplate(data)
			},
		},
		FiveDaysReminder: {
			Label: "5 days before reminder",
			GenerateSubject: func(data TemplateData) string {
				return fmt.Sprintf("⏳ %s Renews in 5 Days - Stay Subscribed!", data.SubscriptionName)
			},
			GenerateBody: func(data TemplateData) string {
				return generateEmailTemplate(data)
			},
		},
		TwoDaysReminder: {
			Label: "2 days before reminder",
			GenerateSubject: func(data TemplateData) string {
				return fmt.Sprintf("🚀 2 Days Left! %s Subscription Renewal", data.SubscriptionName)
			},
			GenerateBody: func(data TemplateData) string {
				return generateEmailTemplate(data)
			},
		},
		OneDayReminder: {
			Label: "1 day before reminder",
			GenerateSubject: func(data TemplateData) string {
				return fmt.Sprintf("⚡ Final Reminder: %s Renews Tomorrow!", data.SubscriptionName)
			},
			GenerateBody: func(data TemplateData) string {
				return generateEmailTemplate(data)
			},
		},
	}
}

// FindTemplateByDays returns the appropriate template based on days before renewal.
func FindTemplateByDays(daysBefore int) (TemplateType, bool) {
	switch daysBefore {
	case 7:
		return SevenDaysReminder, true
	case 5:
		return FiveDaysReminder, true
	case 2:
		return TwoDaysReminder, true
	case 1:
		return OneDayReminder, true
	default:
		return "", false
	}
}

// FormatTime formats time.Time into a readable date string.
func FormatTime(t time.Time) string {
	return t.Format("Jan 2, 2006")
}

// generateEmailTemplate creates HTML email content based on template data.
func generateEmailTemplate(data TemplateData) string {
	return fmt.Sprintf(`
<div style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 0; background-color: #f4f7fa;">
    <table cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #ffffff; border-radius: 10px; overflow: hidden; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);">
        <tr>
            <td style="background-color: #4a90e2; text-align: center;">
                <p style="font-size: 54px; line-height: 54px; font-weight: 800;">SubDub</p>
            </td>
        </tr>
        <tr>
            <td style="padding: 40px 30px;">                
                <p style="font-size: 16px; margin-bottom: 25px;">Hello <strong style="color: #4a90e2;">%s</strong>,</p>
                <p style="font-size: 16px; margin-bottom: 25px;">Your <strong>%s</strong> subscription is set to renew on <strong style="color: #4a90e2;">%s</strong> (%d days from today).</p>
                <table cellpadding="15" cellspacing="0" border="0" width="100%%" style="background-color: #f0f7ff; border-radius: 10px; margin-bottom: 25px;">
                    <tr>
                        <td style="font-size: 16px; border-bottom: 1px solid #d0e3ff;">
                            <strong>Plan:</strong> %s
                        </td>
                    </tr>
                    <tr>
                        <td style="font-size: 16px; border-bottom: 1px solid #d0e3ff;">
                            <strong>Price:</strong> %s
                        </td>
                    </tr>
                    <tr>
                        <td style="font-size: 16px;">
                            <strong>Payment Method:</strong> %s
                        </td>
                    </tr>
                </table>
                <p style="font-size: 16px; margin-bottom: 25px;">If you'd like to make changes or cancel your subscription, please visit your <a href="%s" style="color: #4a90e2; text-decoration: none;">account settings</a> before the renewal date.</p>
                <p style="font-size: 16px; margin-top: 30px;">Need help? <a href="%s" style="color: #4a90e2; text-decoration: none;">Contact our support team</a> anytime.</p>
                <p style="font-size: 16px; margin-top: 30px;">
                    Best regards,<br>
                    <strong>The SubDub Team</strong>
                </p>
            </td>
        </tr>
        <tr>
            <td style="background-color: #f0f7ff; padding: 20px; text-align: center; font-size: 14px;">
                <p style="margin: 0 0 10px;">
                    SubDub Inc. | 123 Main St, Anytown, AN 12345
                </p>
                <p style="margin: 0;">
                    <a href="#" style="color: #4a90e2; text-decoration: none; margin: 0 10px;">Unsubscribe</a> | 
                    <a href="#" style="color: #4a90e2; text-decoration: none; margin: 0 10px;">Privacy Policy</a> | 
                    <a href="#" style="color: #4a90e2; text-decoration: none; margin: 0 10px;">Terms of Service</a>
                </p>
            </td>
        </tr>
    </table>
</div>
`, data.UserName, data.SubscriptionName, data.RenewalDate, data.DaysLeft, data.PlanName, data.Price, data.PaymentMethod, data.AccountURL, data.SupportURL)
}
