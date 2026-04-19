package notifications

import (
	"fmt"
	"time"
)

// emailTemplate represents an email template with subject and body generators.
type emailTemplate struct {
	label           string
	generateSubject func(templateData) string
	generateBody    func(templateData) string
}

// templateData contains all data needed for email templates.
type templateData struct {
	userName         string
	subscriptionName string
	renewalDate      string
	planName         string
	price            string
	accountURL       string
	supportURL       string
	daysLeft         int
}

// getTemplate returns the appropriate email template based on days before renewal
func getTemplate(daysBefore int) emailTemplate {
	template := emailTemplate{
		generateBody: func(data templateData) string {
			return generateEmailTemplate(data)
		},
	}

	switch daysBefore {
	case 7:
		template.generateSubject = func(data templateData) string {
			return fmt.Sprintf("📅 Reminder: Your %s Subscription Renews in 7 Days!", data.subscriptionName)
		}
	case 5:
		template.generateSubject = func(data templateData) string {
			return fmt.Sprintf("⏳ %s Renews in 5 Days - Stay Subscribed!", data.subscriptionName)
		}
	case 3:
		template.generateSubject = func(data templateData) string {
			return fmt.Sprintf("🚀 3 Days Left! %s Subscription Renewal", data.subscriptionName)
		}
	case 1:
		template.generateSubject = func(data templateData) string {
			return fmt.Sprintf("⚡ Final Reminder: %s Renews Tomorrow!", data.subscriptionName)
		}
	default:
		template.generateSubject = func(data templateData) string {
			if data.daysLeft > 7 {
				return fmt.Sprintf("📆 Your %s Subscription Renews in %d Days", data.subscriptionName, data.daysLeft)
			} else if data.daysLeft > 1 {
				return fmt.Sprintf("🔔 %s Subscription Renews in %d Days!", data.subscriptionName, data.daysLeft)
			} else if data.daysLeft == 0 {
				return fmt.Sprintf("⚠️ URGENT: %s Subscription Renews Today!", data.subscriptionName)
			} else {
				return fmt.Sprintf("⚠️ %s Subscription Renewal Notice", data.subscriptionName)
			}
		}
	}

	return template
}

// FormatTime formats time.Time into a readable date string.
func FormatTime(t time.Time) string {
	return t.Format("Jan 2, 2006")
}

// generateEmailTemplate creates HTML email content based on template data.
func generateEmailTemplate(data templateData) string {
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
`,
		data.userName,
		data.subscriptionName,
		data.renewalDate,
		data.daysLeft,
		data.planName,
		data.price,
		data.accountURL,
		data.supportURL,
	)
}
