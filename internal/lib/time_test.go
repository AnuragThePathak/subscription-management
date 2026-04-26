package lib_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/lib"
)

func TestCalcRenewalDate(t *testing.T) {
	makeDate := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	}

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		start     time.Time
		frequency models.Frequency
		want      time.Time
	}{
		// Standard Monthly
		{
			name:      "Standard Monthly (31 days -> 30 days)",
			start:     makeDate(2023, time.January, 15),
			frequency: models.Monthly,
			want:      makeDate(2023, time.February, 15),
		},
		{
			name:      "Standard Monthly (30 days -> 31 days)",
			start:     makeDate(2023, time.April, 15),
			frequency: models.Monthly,
			want:      makeDate(2023, time.May, 15),
		},
		{
			name:      "Standard Monthly (31 days -> 31 days)",
			start:     makeDate(2025, time.July, 20),
			frequency: models.Monthly,
			want:      makeDate(2025, time.August, 20),
		},

		// End of Month
		{
			name:      "End of Month (31 -> 30 days)",
			start:     makeDate(2025, time.May, 31),
			frequency: models.Monthly,
			want:      makeDate(2025, time.June, 30),
		},
		{
			name:      "End of Month (30 -> 31 days)",
			start:     makeDate(2023, time.April, 30),
			frequency: models.Monthly,
			want:      makeDate(2023, time.May, 30),
		},

		// Year End
		{
			name:      "Standard Dec to Jan",
			start:     makeDate(2022, time.December, 25),
			frequency: models.Monthly,
			want:      makeDate(2023, time.January, 25),
		},
		{
			name:      "Month End Dec to Jan",
			start:     makeDate(2022, time.December, 31),
			frequency: models.Monthly,
			want:      makeDate(2023, time.January, 31),
		},

		// Jan to Feb Edge cases
		{
			name:      "Month End Jan to Feb",
			start:     makeDate(2025, time.January, 31),
			frequency: models.Monthly,
			want:      makeDate(2025, time.February, 28),
		},
		{
			name:      "Jan 29th to Feb End",
			start:     makeDate(2025, time.January, 29),
			frequency: models.Monthly,
			want:      makeDate(2025, time.February, 28),
		},

		// Feb to Mar Edge Cases
		{
			name:      "Standard Feb to Mar",
			start:     makeDate(2025, time.February, 10),
			frequency: models.Monthly,
			want:      makeDate(2025, time.March, 10),
		},
		{
			name:      "Month End Feb to Mar",
			start:     makeDate(2025, time.February, 28),
			frequency: models.Monthly,
			want:      makeDate(2025, time.March, 28),
		},

		// Leap Year Month Edge Cases
		{
			name:      "Leap Year Standard Feb to Mar",
			start:     makeDate(2020, time.February, 15),
			frequency: models.Monthly,
			want:      makeDate(2020, time.March, 15),
		},
		{
			name:      "Leap Year Month End Jan to Feb",
			start:     makeDate(2024, time.January, 31),
			frequency: models.Monthly,
			want:      makeDate(2024, time.February, 29),
		},
		{
			name:      "Leap Year Jan 29th to Feb",
			start:     makeDate(2024, time.January, 29),
			frequency: models.Monthly,
			want:      makeDate(2024, time.February, 29),
		},
		{
			name:      "Leap Year Month End Feb to Mar",
			start:     makeDate(2024, time.February, 29),
			frequency: models.Monthly,
			want:      makeDate(2024, time.March, 29),
		},
		{
			name:      "Leap Year Feb 28th to Mar",
			start:     makeDate(2024, time.February, 28),
			frequency: models.Monthly,
			want:      makeDate(2024, time.March, 28),
		},

		// Yearly
		{
			name:      "Standard Yearly",
			start:     makeDate(2025, time.January, 31),
			frequency: models.Yearly,
			want:      makeDate(2026, time.January, 31),
		},
		{
			name:      "Leap Year -> Non-leap year",
			start:     makeDate(2020, time.October, 15),
			frequency: models.Yearly,
			want:      makeDate(2021, time.October, 15),
		},
		{
			name:      "Leap Year -> Non-leap year Feb",
			start:     makeDate(2000, time.February, 15),
			frequency: models.Yearly,
			want:      makeDate(2001, time.February, 15),
		},
		{
			name:      "Leap Year -> Non-leap year Feb End",
			start:     makeDate(2020, time.February, 29),
			frequency: models.Yearly,
			want:      makeDate(2021, time.February, 28),
		},
		{
			name:      "Leap Year Feb 28th -> Non-leap year 28th",
			start:     makeDate(2020, time.February, 28),
			frequency: models.Yearly,
			want:      makeDate(2021, time.February, 28),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lib.CalcRenewalDate(tt.start, tt.frequency)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDaysBetween(t *testing.T) {
	// Helper to build a time at a specific hour (not necessarily midnight),
	// so we can verify the function normalises to midnight correctly.
	makeDateTime := func(year int, month time.Month, day, hour, minute int) time.Time {
		return time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
	}

	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		loc   *time.Location
		want  int
	}{
		// Zero difference
		{
			name:  "Same day same time",
			start: makeDateTime(2025, time.March, 10, 0, 0),
			end:   makeDateTime(2025, time.March, 10, 0, 0),
			loc:   time.UTC,
			want:  0,
		},
		{
			name:  "Same day different times",
			start: makeDateTime(2025, time.March, 10, 6, 0),
			end:   makeDateTime(2025, time.March, 10, 23, 59),
			loc:   time.UTC,
			want:  0,
		},

		// Midnight normalization — without normalising, 23:59→00:01 would be
		// less than one hour apart and would return 0 instead of 1.
		{
			name:  "Late night to early next morning (normalises to 1 day)",
			start: makeDateTime(2025, time.March, 10, 23, 59),
			end:   makeDateTime(2025, time.March, 11, 0, 1),
			loc:   time.UTC,
			want:  1,
		},

		// Standard positive cases
		{
			name:  "One day apart",
			start: makeDateTime(2025, time.March, 10, 0, 0),
			end:   makeDateTime(2025, time.March, 11, 0, 0),
			loc:   time.UTC,
			want:  1,
		},
		{
			name:  "Multiple days apart",
			start: makeDateTime(2025, time.January, 1, 0, 0),
			end:   makeDateTime(2025, time.January, 31, 0, 0),
			loc:   time.UTC,
			want:  30,
		},

		// Negative — end is before start
		{
			name:  "End before start returns negative",
			start: makeDateTime(2025, time.March, 15, 0, 0),
			end:   makeDateTime(2025, time.March, 10, 0, 0),
			loc:   time.UTC,
			want:  -5,
		},

		// Nil loc — must fall back to time.Local without panicking
		{
			name:  "Nil loc falls back to Local",
			start: time.Date(2025, time.March, 10, 0, 0, 0, 0, time.Local),
			end:   time.Date(2025, time.March, 15, 0, 0, 0, 0, time.Local),
			loc:   nil,
			want:  5,
		},

		// Boundary crossings
		{
			name:  "Cross month boundary",
			start: makeDateTime(2025, time.January, 28, 0, 0),
			end:   makeDateTime(2025, time.February, 3, 0, 0),
			loc:   time.UTC,
			want:  6,
		},
		{
			name:  "Cross year boundary",
			start: makeDateTime(2024, time.December, 31, 0, 0),
			end:   makeDateTime(2025, time.January, 1, 0, 0),
			loc:   time.UTC,
			want:  1,
		},

		// February / leap year
		// A naive "every month has 30 days" bug would return 1 here instead of 2.
		{
			name:  "Leap year: Feb 28 to Mar 1 spans Feb 29",
			start: makeDateTime(2024, time.February, 28, 0, 0),
			end:   makeDateTime(2024, time.March, 1, 0, 0),
			loc:   time.UTC,
			want:  2,
		},
		// Non-leap year: the same calendar dates are only 1 day apart.
		{
			name:  "Non-leap year: Feb 28 to Mar 1",
			start: makeDateTime(2025, time.February, 28, 0, 0),
			end:   makeDateTime(2025, time.March, 1, 0, 0),
			loc:   time.UTC,
			want:  1,
		},
		// Full February in a leap year must be 29 days, not 28.
		{
			name:  "Leap year: full February (Feb 1 to Mar 1 = 29 days)",
			start: makeDateTime(2024, time.February, 1, 0, 0),
			end:   makeDateTime(2024, time.March, 1, 0, 0),
			loc:   time.UTC,
			want:  29,
		},
		{
			name:  "Non-leap year: full February (Feb 1 to Mar 1 = 28 days)",
			start: makeDateTime(2025, time.February, 1, 0, 0),
			end:   makeDateTime(2025, time.March, 1, 0, 0),
			loc:   time.UTC,
			want:  28,
		},
		// 4-year span landing on a leap day: 365*4 + 1 extra leap day = 1461 days.
		{
			name:  "Leap day to leap day across 4 years (1461 days)",
			start: makeDateTime(2020, time.February, 29, 0, 0),
			end:   makeDateTime(2024, time.February, 29, 0, 0),
			loc:   time.UTC,
			want:  1461,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lib.DaysBetween(tt.start, tt.end, tt.loc)
			assert.Equal(t, tt.want, got)
		})
	}
}
