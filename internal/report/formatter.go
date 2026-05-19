package report

import (
	"fmt"
	"hik-export/internal/device"
	"log"
	"strings"
	"time"
)

const (
	TimeFormat = "200601021504"
)

// EventToText converts a device event into a formatted text line.
func EventToText(event device.Event) string {
	employeeNoString := event.EmployeeNoString
	if len(employeeNoString) < 4 {
		employeeNoString = strings.Repeat("0", 4-len(employeeNoString)) + employeeNoString
	} else if len(employeeNoString) > 4 {
		employeeNoString = employeeNoString[:4]
	}

	eventTime, err := time.Parse(time.RFC3339, event.Time)
	if err != nil {
		log.Printf("Error parsing time: %v", err)
		return ""
	}

	formattedTime := eventTime.Format(TimeFormat)

	return fmt.Sprintf("%s      %s %s\n",
		employeeNoString,
		formattedTime,
		event.DeviceName,
	)
}

// ParseLineToTime extracts the timestamp from a report line.
// Assumes format: [EmployeeID]      [YYYYMMDDHHMM] [DeviceName]
func ParseLineToTime(line string) (time.Time, error) {
	cleanLine := strings.TrimSpace(line)
	parts := strings.Fields(cleanLine)
	if len(parts) < 3 {
		return time.Time{}, fmt.Errorf("invalid line format: expected at least 3 parts, got %d", len(parts))
	}

	timePart := parts[1]
	return time.Parse(TimeFormat, timePart)
}
