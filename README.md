# Redfish Client Utility

A Python-based utility to interact with Redfish APIs on servers, supporting credential fetching via `kubectl`.

## Implementation Details

The utility is implemented in Python and leverages the `requests` library for HTTP communication with the BMC's Redfish API. It integrates with `kubectl` to automatically discover server credentials from the `gpc-system` namespace.

Key features:
- **Automatic Authentication**: Fetches IP and credentials from Kubernetes `server` and `secret` resources.
- **Secure Communication**: Uses HTTPS (though verification can be suppressed for self-signed certs).
- **Extensible Architecture**: Maps CLI actions to `RedfishClient` methods.
- **Robust Error Handling**:Gracefully handles missing dependencies and connection errors.

## Arguments

| Argument | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `server_name` | Yes | N/A | Name of the server resource in `gpc-system` namespace. |
| `--action` | Yes | N/A | The operation to perform. See usages below. |
| `--system-id` | No | `1` | The functionality ID of the system (e.g. `Systems/1`). |
| `--manager-id` | No | `1` | The ID of the manager (e.g. `Managers/1`). |
| `--reset-type` | Conditional | N/A | Required for `reset_system` and optional for `reset_manager`. |
| `--target-state` | Conditional | N/A | Required for `wait_for_power_state`. |
| `--timeout` | No | `60` | Timeout in seconds for wait operations. |
| `--interval` | No | `5` | Polling interval in seconds for wait operations. |

## Help / Usage Examples

Below are examples for all supported actions. Replace `<server-name>` with your actual server name.

### Power Management

**Get Power State**
```bash
python3 redfish.py <server-name> --action get_power_state
```

**Power On**
```bash
python3 redfish.py <server-name> --action power_on
```

**Graceful Shutdown**
```bash
python3 redfish.py <server-name> --action graceful_shutdown
```

**Force Off**
```bash
python3 redfish.py <server-name> --action force_off
```

**Force Restart**
```bash
python3 redfish.py <server-name> --action force_restart
```

**Reset System (Custom Type)**
```bash
python3 redfish.py <server-name> --action reset_system --reset-type On
```

**Wait for Power State**
```bash
python3 redfish.py <server-name> --action wait_for_power_state --target-state On --timeout 120
```

### Advanced Reset

**Reset Manager (BMC)**
```bash
python3 redfish.py <server-name> --action reset_manager --reset-type ForceRestart
```

**Factory Reset (Manager)**
```bash
python3 redfish.py <server-name> --action factory_reset
```

**Aux Cycle (Power Cycle Machine & iLO)**
```bash
python3 redfish.py <server-name> --action aux_cycle
```

**Secure Erase**
```bash
python3 redfish.py <server-name> --action secure_erase
```

**Get Secure Erase Status**
```bash
python3 redfish.py <server-name> --action get_secure_erase_status
```

**Get POST State**
```bash
python3 redfish.py <server-name> --action get_post_state
```

### Security & Logs

**Get ESKM Logs**
```bash
python3 redfish.py <server-name> --action get_eskm_logs
```

**Test ESKM Connection**
```bash
python3 redfish.py <server-name> --action test_eskm_connection
```

**Get Security State**
```bash
python3 redfish.py <server-name> --action get_security_state
```

**Get Server Config Lock Settings**
```bash
python3 redfish.py <server-name> --action get_server_config_lock_settings
```
