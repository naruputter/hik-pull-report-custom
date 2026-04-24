package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/icholy/digest"
)

// AcsEventCond represents the search conditions
type AcsEventCond struct {
	SearchID             string `json:"searchID"`
	SearchResultPosition int    `json:"searchResultPosition"`
	MaxResults           int    `json:"maxResults"`
	Major                int    `json:"major"`
	Minor                int    `json:"minor"`
	StartTime            string `json:"startTime,omitempty"`
	EndTime              string `json:"endTime,omitempty"`
}

// AcsEventRequest is the payload for ISAPI
type AcsEventRequest struct {
	AcsEventCond AcsEventCond `json:"AcsEventCond"`
}

// Event represents an individual access control event from the response
type Event struct {
	Major             int    `json:"major"`
	Minor             int    `json:"minor"`
	Time              string `json:"time"`
	RemoteHostAddr    string `json:"remoteHostAddr,omitempty"`
	SerialNo          int    `json:"serialNo"`
	DoorNo            int    `json:"doorNo,omitempty"`
	CardNo            string `json:"cardNo,omitempty"`
	CardType          int    `json:"cardType,omitempty"`
	Name              string `json:"name,omitempty"`
	CardReaderNo      int    `json:"cardReaderNo,omitempty"`
	EmployeeNoString  string `json:"employeeNoString,omitempty"`
	UserType          string `json:"userType,omitempty"`
	CurrentVerifyMode string `json:"currentVerifyMode,omitempty"`
	Mask              string `json:"mask,omitempty"`
	DeviceURL         string `json:"-"` // Not from JSON, but populated by client
}

// AcsEventResponse is the response from ISAPI
type AcsEventResponse struct {
	SearchID           string  `json:"searchID"`
	TotalMatches       int     `json:"totalMatches"`
	ResponseStatusStrg string  `json:"responseStatusStrg"`
	NumOfMatches       int     `json:"numOfMatches"`
	InfoList           []Event `json:"InfoList"`
}

// AcsEventResponseWrapper wraps the ISAPI response
type AcsEventResponseWrapper struct {
	AcsEvent AcsEventResponse `json:"AcsEvent"`
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL, user, pass string) *Client {
	// Use Digest Authentication which is standard for Hikvision ISAPI
	client := &http.Client{
		Transport: &digest.Transport{
			Username: user,
			Password: pass,
		},
		Timeout: 30 * time.Second,
	}
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: client,
	}
}

// FetchEvents calls the ISAPI endpoint to search for events
func (c *Client) FetchEvents(startTime time.Time) ([]Event, error) {
	fmt.Printf("Fetching events from device since %v...\n", startTime.Format(time.RFC3339))

	var allEvents []Event
	searchID := "fetch_logs_" + time.Now().Format("150405")
	position := 0
	maxResults := 30
	endTime := time.Now()

	for {
		reqBody := AcsEventRequest{
			AcsEventCond: AcsEventCond{
				SearchID:             searchID,
				SearchResultPosition: position,
				MaxResults:           maxResults,
				// Setting 0 often implies 'all' types for Hikvision, adjust if specific major/minors are strictly required
				Major:     0,
				Minor:     0,
				StartTime: startTime.Format(time.RFC3339),
				EndTime:   endTime.Format(time.RFC3339),
			},
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		reqURL := fmt.Sprintf("%s/ISAPI/AccessControl/AcsEvent?format=json", c.BaseURL)
		req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}

		// print resp json
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("failed to read response body: %v\n", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("device returned status code: %d", resp.StatusCode)
		}

		var wrapper AcsEventResponseWrapper
		if err := json.Unmarshal(respBody, &wrapper); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		eventResp := wrapper.AcsEvent
		for i := range eventResp.InfoList {
			eventResp.InfoList[i].DeviceURL = c.BaseURL
		}
		allEvents = append(allEvents, eventResp.InfoList...)

		// Handle pagination if more records exist
		if eventResp.ResponseStatusStrg == "MORE" || eventResp.ResponseStatusStrg == "more" {
			position += eventResp.NumOfMatches
			if eventResp.NumOfMatches == 0 {
				break
			}
		} else {
			break
		}
	}

	return allEvents, nil
}
