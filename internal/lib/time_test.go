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
			name:      "Standard Jan to Feb",
			start:     makeDate(2025, time.January, 15),
			frequency: models.Monthly,
			want:      makeDate(2025, time.February, 15),
		},
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
		{
			name:      "Jan 28th to Feb End",
			start:     makeDate(2025, time.January, 28),
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
		{
			name:      "Non Leap Year -> Leap year Feb End",
			start:     makeDate(2019, time.February, 28),
			frequency: models.Yearly,
			want:      makeDate(2020, time.February, 28),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lib.CalcRenewalDate(tc.start, tc.frequency)
			assert.Equal(t, tc.want, got)
		})
	}
}
