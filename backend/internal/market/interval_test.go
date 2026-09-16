package market

import (
	"testing"
	"time"
)

func TestCandleIntervalCalendarBoundaries(t *testing.T) {
	at := time.Date(2024, time.March, 31, 23, 45, 0, 0, time.FixedZone("test", 2*60*60))
	tests := []struct {
		interval CandleInterval
		open     time.Time
		next     time.Time
	}{
		{IntervalHour, time.Date(2024, 3, 31, 21, 0, 0, 0, time.UTC), time.Date(2024, 3, 31, 22, 0, 0, 0, time.UTC)},
		{IntervalDay, time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC), time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)},
		{IntervalWeek, time.Date(2024, 3, 25, 0, 0, 0, 0, time.UTC), time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)},
		{IntervalMonth, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, test := range tests {
		if got := test.interval.OpenTime(at); !got.Equal(test.open) {
			t.Errorf("%s OpenTime = %s, want %s", test.interval, got, test.open)
		}
		if got := test.interval.NextOpenTime(test.open); !got.Equal(test.next) {
			t.Errorf("%s NextOpenTime = %s, want %s", test.interval, got, test.next)
		}
		if got := test.interval.LastClosedOpenTime(test.next); !got.Equal(test.open) {
			t.Errorf("%s LastClosedOpenTime = %s, want %s", test.interval, got, test.open)
		}
	}
}
