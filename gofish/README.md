# Redfish Client (Gofish Version)

This folder contains a Redfish client implementation using the `github.com/stmcginnis/gofish` library.

## Dependencies
- [gofish](https://github.com/stmcginnis/gofish)
- Standard Go libraries

## Build
```bash
go build -o gofish-cli .
```

## Usage
The CLI arguments are identical to the native version:
```bash
./gofish-cli --server-name <SERVER> --action get_power_state
```

## Safety Features
- **Prompt**: Confirm all `POST` requests before execution.
- **Verbose**: Use `-v` for HTTP request/response traces.
- **Skip Prompt**: Use `-y` or `--yes` for non-interactive mode.
