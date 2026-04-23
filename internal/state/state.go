package state

import (
	"encoding/json"
	"os"
	"time"
)

type DeviceState struct {
	LastFetchTime time.Time `json:"last_fetch_time"`
}

type AppState struct {
	DeviceStates map[string]DeviceState `json:"device_states"`
}

func LoadState(filePath string) (*AppState, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default state if file doesn't exist
			return &AppState{
				DeviceStates: make(map[string]DeviceState),
			}, nil
		}
		return nil, err
	}

	var state AppState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.DeviceStates == nil {
		state.DeviceStates = make(map[string]DeviceState)
	}

	return &state, nil
}

func SaveState(filePath string, state *AppState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
