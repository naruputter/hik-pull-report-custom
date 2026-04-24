package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	cfg, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 3. Fetch Events from all devices
	allEvents, deviceLastTimes := fetchEventsFromDevices(cfg, appState)
	if len(allEvents) == 0 {
		fmt.Println("No new events found from any device.")
		return
	}

	// 4. Filter and Merge New Events (Deduplicate logs based on threshold)
	filteredEvents := filterEvents(allEvents, cfg.MergeThresholdSeconds)

	// 5. Group events by Month/Year and Sync to separate files
	if err := syncEventsToMonthlyFiles(cfg.ReportFile, filteredEvents); err != nil {
		log.Fatalf("Failed to sync report files: %v", err)
	}

	// 6. Update and Save State
	updateState(appState, allEvents, deviceLastTimes)
	if err := state.SaveState(stateFile, appState); err != nil {
		log.Fatalf("Failed to save state: %v", err)
	}

	fmt.Printf("Successfully processed %d events (%d merged).\n",
		len(allEvents), len(allEvents)-len(filteredEvents))
}

func syncEventsToMonthlyFiles(basePath string, events []device.Event) error {
	// Group events by MMYYYY
	groups := make(map[string][]device.Event)
	for _, event := range events {
		t, err := time.Parse(time.RFC3339, event.Time)
		if err != nil {
			continue
		}
		key := t.Format("012006") // MMYYYY
		groups[key] = append(groups[key], event)
	}

	// For each month group, determine filename and sync
	ext := filepath.Ext(basePath)
	nameWithoutExt := basePath[:len(basePath)-len(ext)]

	for key, groupEvents := range groups {
		monthlyFile := fmt.Sprintf("%s_%s%s", nameWithoutExt, key, ext)
		fmt.Printf("Updating report: %s (%d events)\n", monthlyFile, len(groupEvents))
		if err := syncReportFile(monthlyFile, groupEvents); err != nil {
			return err
		}
	}
	return nil
}

func loadConfig(path string) (*Config, error) {
	cfgData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		return nil, err
	}
	if cfg.ReportFile == "" {
		cfg.ReportFile = "report_export.txt"
	}
	return &cfg, nil
}

func fetchEventsFromDevices(cfg *Config, appState *state.AppState) ([]device.Event, map[string]time.Time) {
	var allEvents []device.Event
	deviceLastTimes := make(map[string]time.Time)

	for _, devCfg := range cfg.Devices {
		client := device.NewClient(devCfg.URL, devCfg.Username, devCfg.Password)

		lastTime := time.Now().Add(-24 * time.Hour) // Default fallback
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

	// Initial sort of fetched events by time
	sort.Slice(allEvents, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, allEvents[i].Time)
		tj, _ := time.Parse(time.RFC3339, allEvents[j].Time)
		return ti.Before(tj)
	})

	return allEvents, deviceLastTimes
}

func filterEvents(events []device.Event, threshold int) []device.Event {
	var filtered []device.Event
	lastProcessedByEmployee := make(map[string]time.Time)

	for _, event := range events {
		if event.EmployeeNoString == "" {
			continue
		}

		eventTime, err := time.Parse(time.RFC3339, event.Time)
		if err != nil {
			log.Printf("Warning: failed to parse time for event %d (%s): %v", event.SerialNo, event.Time, err)
			continue
		}

		lastTime, exists := lastProcessedByEmployee[event.EmployeeNoString]
		if exists && eventTime.Sub(lastTime).Seconds() < float64(threshold) {
			continue
		}

		filtered = append(filtered, event)
		lastProcessedByEmployee[event.EmployeeNoString] = eventTime
	}
	return filtered
}

type reportEntry struct {
	time time.Time
	line string
}

func syncReportFile(reportFile string, newEvents []device.Event) error {
	var finalEntries []reportEntry

	// A. Load existing entries
	existingData, err := os.ReadFile(reportFile)
	if err == nil {
		lines := strings.Split(string(existingData), "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			t, err := report.ParseLineToTime(l)
			if err != nil {
				log.Printf("Warning: skipping invalid report line: %s", l)
				continue
			}
			finalEntries = append(finalEntries, reportEntry{time: t, line: l + "\n"})
		}
	}

	// B. Add new events
	for _, event := range newEvents {
		line := report.EventToText(event)
		if line == "" {
			continue
		}
		t, _ := report.ParseLineToTime(line)
		finalEntries = append(finalEntries, reportEntry{time: t, line: line})
	}

	// C. Sort all entries by time
	sort.SliceStable(finalEntries, func(i, j int) bool {
		return finalEntries[i].time.Before(finalEntries[j].time)
	})

	// D. Deduplicate identical lines
	if len(finalEntries) > 0 {
		uniqueEntries := make([]reportEntry, 0, len(finalEntries))
		seen := make(map[string]bool)
		for _, entry := range finalEntries {
			if !seen[entry.line] {
				uniqueEntries = append(uniqueEntries, entry)
				seen[entry.line] = true
			}
		}
		finalEntries = uniqueEntries
	}

	// E. Overwrite Report File
	// Ensure the directory exists
	dir := filepath.Dir(reportFile)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create report directory: %w", err)
		}
	}

	f, err := os.OpenFile(reportFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range finalEntries {
		if _, err := f.WriteString(entry.line); err != nil {
			return err
		}
	}
	return nil
}

func updateState(appState *state.AppState, allFetchedEvents []device.Event, deviceLastTimes map[string]time.Time) {
	// Track the latest time for each device from all fetched events
	for _, event := range allFetchedEvents {
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
}
