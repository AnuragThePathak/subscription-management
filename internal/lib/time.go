package lib

import (
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/models"
)

func CalcRenewalDate(start time.Time, frequency models.Frequency) time.Time {
	switch frequency {
	case models.Daily:
		return start.AddDate(0, 0, 1)
	case models.Weekly:
		return start.AddDate(0, 0, 7)
	case models.Monthly:
		// Get original day to preserve
		originalDay := start.Day()

		// Get next month date
		nextMonth := time.Date(
			start.Year(),
			start.Month()+1,
			1, // temporarily use 1st of month
			start.Hour(),
			start.Minute(),
			start.Second(),
			start.Nanosecond(),
			start.Location(),
		)

		// Handle December → January transition
		if start.Month() == time.December {
			nextMonth = time.Date(
				start.Year()+1,
				time.January,
				1,
				start.Hour(),
				start.Minute(),
				start.Second(),
				start.Nanosecond(),
				start.Location(),
			)
		}

		// Find out how many days are in the next month
		lastDayOfNextMonth := time.Date(
			nextMonth.Year(),
			nextMonth.Month()+1,
			0, // This gives the last day of nextMonth
			0, 0, 0, 0,
			nextMonth.Location(),
		).Day()

		// Use either the original day or the last day of the month, whichever is smaller
		renewalDay := min(originalDay, lastDayOfNextMonth)

		return time.Date(
			nextMonth.Year(),
			nextMonth.Month(),
			renewalDay,
			start.Hour(),
			start.Minute(),
			start.Second(),
			start.Nanosecond(),
			start.Location(),
		)
	case models.Yearly:
		return start.AddDate(1, 0, 0)
	default:
		return start // fallback, no change
	}
}

func DaysBetween(start, end time.Time, loc *time.Location) int {
	if loc == nil {
		loc = time.Local
	}

	// Normalize both dates to midnight in the given location
	yearStart, monthStart, dayStart := start.In(loc).Date()
	yearEnd, monthEnd, dayEnd := end.In(loc).Date()

	startDate := time.Date(yearStart, monthStart, dayStart, 0, 0, 0, 0, loc)
	endDate := time.Date(yearEnd, monthEnd, dayEnd, 0, 0, 0, 0, loc)

	return int(endDate.Sub(startDate).Hours() / 24)
}
