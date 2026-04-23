package report

import (
	"fmt"
	"hik-export/internal/device"
	"log"
	"time"
)

// EventToText converts a device event into a formatted text line.
// You can adjust this format based on your requirements.
func EventToText(event device.Event) string {
	// Fallback handlers if fields are empty
	name := event.Name
	if name == "" {
		name = "N/A"
	}

	cardNo := event.CardNo
	if cardNo == "" {
		cardNo = "N/A"
	}

	employeeNoString := event.EmployeeNoString
	if employeeNoString == "" {
		employeeNoString = "N/A"
	}

	verifyMode := event.CurrentVerifyMode
	if verifyMode == "" {
		verifyMode = fmt.Sprintf("M%d/m%d", event.Major, event.Minor)
	}

	// i want format  cardNo YYYY MM DD HH MM
	// event.Time is RFC3339 format

	// parse event.Time to time.Time
	eventTime, err := time.Parse(time.RFC3339, event.Time)
	if err != nil {
		log.Printf("Error parsing time: %v", err)
		return ""
	}

	// format eventTime to YYYY MM DD HH MM
	formattedTime := eventTime.Format("2006-01-02 15:04")

	return fmt.Sprintf("%s %s\n",
		employeeNoString,
		formattedTime,
	)
}
