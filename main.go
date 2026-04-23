package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"hik-export/internal/device"
	"hik-export/internal/report"
	"hik-export/internal/state"
)

const (
	stateFile  = "state.json"
	configFile = "config.json"
)

type DeviceConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	Devices               []DeviceConfig `json:"devices"`
	MergeThresholdSeconds int            `json:"merge_threshold_seconds"`
	ReportFile            string         `json:"report_file"`
}

func main() {
	fmt.Println("=== Hikvision Log Export Utility (Multi-Device) ===")

	// 1. Load Last State
	appState, err := state.LoadState(stateFile)
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	// 2. Load Config
	cfgData, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to load config.json: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		log.Fatalf("Failed to parse config.json: %v", err)
	}

	if cfg.ReportFile == "" {
		cfg.ReportFile = "report_export.txt"
	}

	// 3. Fetch Events from all devices
	var allEvents []device.Event
	deviceLastTimes := make(map[string]time.Time)

	for _, devCfg := range cfg.Devices {
		client := device.NewClient(devCfg.URL, devCfg.Username, devCfg.Password)

		// Get last fetch time for this specific device
		lastTime := time.Now().Add(-24 * time.Hour) // Default
		if ds, ok := appState.DeviceStates[devCfg.URL]; ok {
			lastTime = ds.LastFetchTime
		}

		events, err := client.FetchEvents(lastTime)
		if err != nil {
			log.Printf("Error fetching from %s: %v", devCfg.URL, err)
			continue
		}

		allEvents = append(allEvents, events...)
		deviceLastTimes[devCfg.URL] = lastTime
	}

	if len(allEvents) == 0 {
		fmt.Println("No new events found from any device.")
		return
	}

	// 4. Sort Events by Time
	sort.Slice(allEvents, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, allEvents[i].Time)
		tj, _ := time.Parse(time.RFC3339, allEvents[j].Time)
		return ti.Before(tj)
	})

	// 5. Deduplicate / Merge logs based on threshold
	var filteredEvents []device.Event
	lastProcessedByEmployee := make(map[string]time.Time)

	for _, event := range allEvents {
		if event.EmployeeNoString == "" {
			continue
		}

		eventTime, err := time.Parse(time.RFC3339, event.Time)
		if err != nil {
			log.Printf("Warning: failed to parse time for event %d (%s): %v", event.SerialNo, event.Time, err)
			continue
		}

		lastTime, exists := lastProcessedByEmployee[event.EmployeeNoString]
		if exists && eventTime.Sub(lastTime).Seconds() < float64(cfg.MergeThresholdSeconds) {
			// Skip this event as it's within the threshold of the previous one for the same employee
			continue
		}

		filteredEvents = append(filteredEvents, event)
		lastProcessedByEmployee[event.EmployeeNoString] = eventTime

		// Update device's last processed time in appState
		// Note: we can't easily know which device this came from without modifying Event struct,
		// but we can just update all device states to the latest event time we've seen if we want,
		// OR we can add a DeviceURL field to the Event struct.
		// Let's add DeviceURL to the Event struct to be more precise.
	}

	// 6. Append to Report File
	f, err := os.OpenFile(cfg.ReportFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open report file: %v", err)
	}
	defer f.Close()

	for _, event := range filteredEvents {
		line := report.EventToText(event)
		if _, err := f.WriteString(line); err != nil {
			log.Printf("Error writing event: %v", err)
			continue
		}
	}

	// 7. Update State - track the latest time for each device from all fetched events
	for _, event := range allEvents {
		eventTime, err := time.Parse(time.RFC3339, event.Time)
		if err != nil {
			continue
		}

		currentMax, exists := deviceLastTimes[event.DeviceURL]
		if !exists || eventTime.After(currentMax) {
			deviceLastTimes[event.DeviceURL] = eventTime
		}
	}

	for url, lastTime := range deviceLastTimes {
		appState.DeviceStates[url] = state.DeviceState{
			LastFetchTime: lastTime,
		}
	}

	if err := state.SaveState(stateFile, appState); err != nil {
		log.Fatalf("Failed to save state: %v", err)
	}

	fmt.Printf("Successfully processed %d events (%d merged). Logs exported to %s\n",
		len(allEvents), len(allEvents)-len(filteredEvents), cfg.ReportFile)
}
