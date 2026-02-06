// Package site provides site resolution and enrichment for alerts.
package site

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// IsWithinBusinessHours checks if the given time falls within the site's business hours.
// Returns true if:
// - Business hours are not configured (always business hours)
// - The time is within the configured hours and days
func IsWithinBusinessHours(bh *BusinessHours, t time.Time, timezone string) (bool, error) {
	if bh == nil {
		// No business hours configured means always business hours
		return true, nil
	}

	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	// Convert to site's local time
	localTime := t.In(loc)

	// Check if current day is a business day
	currentDay := int(localTime.Weekday())
	isDayMatch := false

	if len(bh.Days) == 0 {
		// No days specified means all days
		isDayMatch = true
	} else {
		for _, day := range bh.Days {
			if day == currentDay {
				isDayMatch = true
				break
			}
		}
	}

	if !isDayMatch {
		return false, nil
	}

	// Parse start and end times
	startHour, startMin, err := parseTimeString(bh.Start)
	if err != nil {
		return false, fmt.Errorf("invalid start time: %w", err)
	}

	endHour, endMin, err := parseTimeString(bh.End)
	if err != nil {
		return false, fmt.Errorf("invalid end time: %w", err)
	}

	// Calculate minutes from midnight
	currentMinutes := localTime.Hour()*60 + localTime.Minute()
	startMinutes := startHour*60 + startMin
	endMinutes := endHour*60 + endMin

	// Handle overnight business hours (e.g., 22:00 - 06:00)
	if endMinutes < startMinutes {
		// Overnight window: current time is within hours if it's >= start OR < end
		return currentMinutes >= startMinutes || currentMinutes < endMinutes, nil
	}

	// Normal business hours
	return currentMinutes >= startMinutes && currentMinutes < endMinutes, nil
}

// parseTimeString parses a time string in "HH:MM" format.
func parseTimeString(s string) (hour, minute int, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format, expected HH:MM: %s", s)
	}

	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour: %s", parts[0])
	}

	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute: %s", parts[1])
	}

	return hour, minute, nil
}

// GetNextBusinessHoursStart calculates when the next business hours period starts.
// Returns the time when business hours will next begin.
func GetNextBusinessHoursStart(bh *BusinessHours, t time.Time, timezone string) (time.Time, error) {
	if bh == nil {
		// No business hours means always business hours
		return t, nil
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	localTime := t.In(loc)

	startHour, startMin, err := parseTimeString(bh.Start)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid start time: %w", err)
	}

	// Get today's business hours start
	todayStart := time.Date(
		localTime.Year(), localTime.Month(), localTime.Day(),
		startHour, startMin, 0, 0, loc,
	)

	// Check next 7 days to find the next business day
	for i := 0; i < 8; i++ {
		checkTime := todayStart.AddDate(0, 0, i)
		checkDay := int(checkTime.Weekday())

		isDayMatch := len(bh.Days) == 0
		for _, day := range bh.Days {
			if day == checkDay {
				isDayMatch = true
				break
			}
		}

		if isDayMatch && checkTime.After(localTime) {
			return checkTime, nil
		}
	}

	return time.Time{}, fmt.Errorf("no business hours found in next 7 days")
}

// GetBusinessHoursInfo returns a human-readable description of the business hours.
func GetBusinessHoursInfo(bh *BusinessHours, timezone string) string {
	if bh == nil {
		return "24/7 (no business hours configured)"
	}

	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	days := make([]string, 0, len(bh.Days))

	if len(bh.Days) == 0 {
		days = append(days, "every day")
	} else {
		for _, d := range bh.Days {
			if d >= 0 && d < 7 {
				days = append(days, dayNames[d])
			}
		}
	}

	return fmt.Sprintf("%s - %s %s (%s)", bh.Start, bh.End, strings.Join(days, ", "), timezone)
}
