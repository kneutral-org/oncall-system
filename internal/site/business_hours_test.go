package site

import (
	"testing"
	"time"
)

func TestIsWithinBusinessHours(t *testing.T) {
	tests := []struct {
		name     string
		bh       *BusinessHours
		t        time.Time
		timezone string
		want     bool
		wantErr  bool
	}{
		{
			name:     "nil business hours - always business hours",
			bh:       nil,
			t:        time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC), // Monday 2pm
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name: "within business hours - Monday 10am",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5}, // Mon-Fri
			},
			t:        time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Monday 10am
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name: "outside business hours - Monday 8am",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			t:        time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC), // Monday 8am
			timezone: "UTC",
			want:     false,
			wantErr:  false,
		},
		{
			name: "outside business hours - Monday 6pm",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			t:        time.Date(2024, 1, 15, 18, 0, 0, 0, time.UTC), // Monday 6pm
			timezone: "UTC",
			want:     false,
			wantErr:  false,
		},
		{
			name: "weekend - not a business day",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5}, // Mon-Fri only
			},
			t:        time.Date(2024, 1, 13, 10, 0, 0, 0, time.UTC), // Saturday 10am
			timezone: "UTC",
			want:     false,
			wantErr:  false,
		},
		{
			name: "overnight hours - within late night",
			bh: &BusinessHours{
				Start: "22:00",
				End:   "06:00",
				Days:  []int{0, 1, 2, 3, 4, 5, 6}, // All days
			},
			t:        time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC), // Monday 11pm
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name: "overnight hours - within early morning",
			bh: &BusinessHours{
				Start: "22:00",
				End:   "06:00",
				Days:  []int{0, 1, 2, 3, 4, 5, 6},
			},
			t:        time.Date(2024, 1, 15, 3, 0, 0, 0, time.UTC), // Monday 3am
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name: "overnight hours - outside window",
			bh: &BusinessHours{
				Start: "22:00",
				End:   "06:00",
				Days:  []int{0, 1, 2, 3, 4, 5, 6},
			},
			t:        time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), // Monday noon
			timezone: "UTC",
			want:     false,
			wantErr:  false,
		},
		{
			name: "no days specified - all days active",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{}, // Empty = all days
			},
			t:        time.Date(2024, 1, 13, 10, 0, 0, 0, time.UTC), // Saturday 10am
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name: "edge case - exactly at start time",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			t:        time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC), // Monday 9am exactly
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name: "edge case - exactly at end time (should be false)",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			t:        time.Date(2024, 1, 15, 17, 0, 0, 0, time.UTC), // Monday 5pm exactly
			timezone: "UTC",
			want:     false,
			wantErr:  false,
		},
		{
			name: "different timezone - America/New_York",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			t:        time.Date(2024, 1, 15, 19, 0, 0, 0, time.UTC), // 2pm ET
			timezone: "America/New_York",
			want:     true,
			wantErr:  false,
		},
		{
			name: "invalid start time",
			bh: &BusinessHours{
				Start: "invalid",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			t:        time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  true,
		},
		{
			name: "invalid end time",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "25:00", // Invalid hour
				Days:  []int{1, 2, 3, 4, 5},
			},
			t:        time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsWithinBusinessHours(tt.bh, tt.t, tt.timezone)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsWithinBusinessHours() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsWithinBusinessHours() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTimeString(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		wantHour   int
		wantMinute int
		wantErr    bool
	}{
		{
			name:       "valid time 09:00",
			s:          "09:00",
			wantHour:   9,
			wantMinute: 0,
			wantErr:    false,
		},
		{
			name:       "valid time 17:30",
			s:          "17:30",
			wantHour:   17,
			wantMinute: 30,
			wantErr:    false,
		},
		{
			name:       "valid time 00:00",
			s:          "00:00",
			wantHour:   0,
			wantMinute: 0,
			wantErr:    false,
		},
		{
			name:       "valid time 23:59",
			s:          "23:59",
			wantHour:   23,
			wantMinute: 59,
			wantErr:    false,
		},
		{
			name:       "invalid format - no colon",
			s:          "0900",
			wantHour:   0,
			wantMinute: 0,
			wantErr:    true,
		},
		{
			name:       "invalid hour - 25",
			s:          "25:00",
			wantHour:   0,
			wantMinute: 0,
			wantErr:    true,
		},
		{
			name:       "invalid minute - 60",
			s:          "09:60",
			wantHour:   0,
			wantMinute: 0,
			wantErr:    true,
		},
		{
			name:       "invalid format - letters",
			s:          "ab:cd",
			wantHour:   0,
			wantMinute: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHour, gotMinute, err := parseTimeString(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimeString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotHour != tt.wantHour {
				t.Errorf("parseTimeString() hour = %v, want %v", gotHour, tt.wantHour)
			}
			if gotMinute != tt.wantMinute {
				t.Errorf("parseTimeString() minute = %v, want %v", gotMinute, tt.wantMinute)
			}
		})
	}
}

func TestGetBusinessHoursInfo(t *testing.T) {
	tests := []struct {
		name     string
		bh       *BusinessHours
		timezone string
		want     string
	}{
		{
			name:     "nil business hours",
			bh:       nil,
			timezone: "UTC",
			want:     "24/7 (no business hours configured)",
		},
		{
			name: "weekday hours",
			bh: &BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			timezone: "UTC",
			want:     "09:00 - 17:00 Mon, Tue, Wed, Thu, Fri (UTC)",
		},
		{
			name: "no days specified",
			bh: &BusinessHours{
				Start: "00:00",
				End:   "23:59",
				Days:  []int{},
			},
			timezone: "UTC",
			want:     "00:00 - 23:59 every day (UTC)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetBusinessHoursInfo(tt.bh, tt.timezone); got != tt.want {
				t.Errorf("GetBusinessHoursInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
