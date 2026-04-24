package report

import (
	"fmt"
	"hik-export/internal/device"
	"log"
	"strings"
	"time"
)

const (
	TimeFormat = "20060102-15:04"
)

// EventToText converts a device event into a formatted text line.
func EventToText(event device.Event) string {
	employeeNoString := event.EmployeeNoString
	if employeeNoString == "" {
		employeeNoString = "N/A"
	}

	eventTime, err := time.Parse(time.RFC3339, event.Time)
	if err != nil {
		log.Printf("Error parsing time: %v", err)
		return ""
	}

	formattedTime := eventTime.Format(TimeFormat)

	return fmt.Sprintf("%s-%s\n",
		employeeNoString,
		formattedTime,
	)
}

// ParseLineToTime extracts the timestamp from a report line.
// Assumes format: [EmployeeID]-[YYYYMMDD]-[HH:MM]
func ParseLineToTime(line string) (time.Time, error) {
	cleanLine := strings.TrimSpace(line)
	// The time part always has a fixed length: YYYYMMDD-HH:MM = 14 chars
	if len(cleanLine) < 14 {
		return time.Time{}, fmt.Errorf("line too short")
	}

	timePart := cleanLine[len(cleanLine)-14:]
	return time.Parse(TimeFormat, timePart)
}
