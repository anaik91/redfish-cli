# Redfish Client Utility

A Go-based utility to interact with Redfish APIs on servers, supporting credential fetching via `kubectl`.

## Implementation Details

The utility is implemented in Go and leverages the `net/http` library for communication with the BMC's Redfish API. It integrates with `kubectl` to automatically discover server credentials from the `gpc-system` namespace.

Key features:
- **Automatic Authentication**: Fetches IP and credentials from Kubernetes `server` and `secret` resources.
- **Secure Communication**: Uses HTTPS (insecure skip verify for self-signed certs).
- **Retry Logic**: Automatically retries failed requests (up to 3 times) for improved stability.
- **Portability**: Compiled to a single static binary for easy distribution.
- **Robust Error Handling**: Gracefully handles missing credentials and connection errors.

## Building Portable Binary (Static Linking)
To ensure the binary works across different Linux distributions (e.g., Debian, Ubuntu) without `glibc` version errors, build it as a static binary:

1. Initialize/Audit dependencies: `go mod tidy`
2. Build the static binary:
   ```bash
   CGO_ENABLED=0 go build -ldflags="-extldflags=-static" -o redfish-cli
   ```
3. The executable will be available as `./redfish-cli`.

## Prerequisites
- Go 1.18+ (for building)
- `kubectl` installed and configured to access the cluster.

## Environment Variables
- `KUBECONFIG`: Can be set to specify the path to the kubeconfig file. If not set, it defaults to `/root/release/root-admin/root-admin-kubeconfig`.

## Arguments

| Argument | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `--server-name` | Conditional | N/A | Name of the server. Required if not using `--list-servers`. |
| `--list-servers` | No | `false` | Lists available servers with IPs. |
| `--action` | Conditional | N/A | The operation to perform (e.g., `power_on`, `get_power_state`). See help for full list. |
| `--system-id` | No | `1` | The functionality ID of the system (e.g. `Systems/1`). |
| `--manager-id` | No | `1` | The ID of the manager (e.g. `Managers/1`). |
| `--reset-type` | Conditional | N/A | Required for `reset_system` and optional for `reset_manager`. |
| `--target-state` | Conditional | N/A | Required for `wait_for_power_state`. |
| `--timeout` | No | `60` | Timeout in seconds for wait operations. |
| `--interval` | No | `5` | Polling interval in seconds for wait operations. |
| `-v` | No | `false` | Enable verbose logging. |

## Help / Usage Examples

**List Servers**
```bash
./redfish-cli --list-servers
```

### Power Management

**Get Power State**
```bash
./redfish-cli --server-name <server-name> --action get_power_state
```

**Power On**
```bash
./redfish-cli --server-name <server-name> --action power_on
```

**Wait for Power State**
```bash
./redfish-cli --server-name <server-name> --action wait_for_power_state --target-state On --timeout 120
```

### Advanced Reset

**Reset Manager (BMC)**
```bash
./redfish-cli --server-name <server-name> --action reset_manager --reset-type ForceRestart
```

**Aux Cycle (Power Cycle Machine & iLO)**
```bash
./redfish-cli --server-name <server-name> --action aux_cycle
```

**Secure Erase**
```bash
./redfish-cli --server-name <server-name> --action secure_erase
```

**Get Secure Erase Status**
```bash
./redfish-cli --server-name <server-name> --action get_secure_erase_status
```

### Security & Logs

**Get ESKM Logs**
```bash
./redfish-cli --server-name <server-name> --action get_eskm_logs
```

**Get Security State**
```bash
./redfish-cli --server-name <server-name> --action get_security_state
```
