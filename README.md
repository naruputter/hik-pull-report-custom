# Hikvision Log Export Utility

This utility fetches access control logs from a Hikvision face scanner and exports them to a text file.

## Configuration

This tool uses environment variables for configuration. You can copy the template file to get started:

```bash
cp .env.example .env
```

Edit `.env` to match your device settings:
- `HIK_URL`: The full URL to your device (e.g., `http://192.168.1.100`)
- `HIK_USERNAME`: Device admin username
- `HIK_PASSWORD`: Device admin password

## How to Build

### For Mac/Linux
```bash
go build -o hik-export main.go
```

### For Windows (to use with Task Scheduler)
```bash
GOOS=windows GOARCH=amd64 go build -o hik-export.exe main.go
```

## How it works
1. **State Persistence**: The tool stores the timestamp of the last processed log in `state.json`.
2. **Incremental Fetch**: On each run, it only fetches logs newer than the timestamp in `state.json`.
3. **Report Generation**: Logs are formatted and appended to `report_export.txt`.

## Project Structure
- `main.go`: Orchestrates the export process.
- `internal/device/`: Hikvision ISAPI communication logic.
- `internal/report/`: Formatting logic for the text output.
- `internal/state/`: Logic for saving/loading the last fetch position.
