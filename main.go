package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// RedfishClient handles communication with the Redfish API
type RedfishClient struct {
	BaseURL  string
	Username string
	Password string
	Client   *http.Client
}

// NewRedfishClient creates a new Redfish client with an insecure TLS configuration
func NewRedfishClient(baseURL, username, password string) *RedfishClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &RedfishClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Client: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
	}
}

// DoRequest performs an HTTP request with retries and basic auth
func (c *RedfishClient) DoRequest(method, url string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("error marshaling payload: %v", err)
		}
		body = bytes.NewBuffer(jsonBody)
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %v", err)
		}

		req.SetBasicAuth(c.Username, c.Password)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.Client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return io.ReadAll(resp.Body)
		}

		if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode <= 504) {
			lastErr = fmt.Errorf("server returned status: %s", resp.Status)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	return nil, fmt.Errorf("request failed after retries: %v", lastErr)
}

// Login verifies connectivity to the Redfish API
func (c *RedfishClient) Login() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/redfish/v1/Systems", c.BaseURL)
	body, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling login response: %v", err)
	}
	return result, nil
}

// GetPowerState retrieves the power state of a system
func (c *RedfishClient) GetPowerState(systemID string) (string, error) {
	url := fmt.Sprintf("%s/redfish/v1/Systems/%s", c.BaseURL, systemID)
	body, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		PowerState string `json:"PowerState"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("error unmarshaling power state: %v", err)
	}
	return result.PowerState, nil
}

// ResetSystem performs a reset action on a system
func (c *RedfishClient) ResetSystem(resetType, systemID string) (interface{}, error) {
	url := fmt.Sprintf("%s/redfish/v1/Systems/%s/Actions/ComputerSystem.Reset", c.BaseURL, systemID)
	payload := map[string]string{"ResetType": resetType}
	body, err := c.DoRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return true, nil
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// PowerOn auxiliary method
func (c *RedfishClient) PowerOn(systemID string) (interface{}, error) {
	return c.ResetSystem("On", systemID)
}

// GracefulShutdown auxiliary method
func (c *RedfishClient) GracefulShutdown(systemID string) (interface{}, error) {
	return c.ResetSystem("GracefulShutdown", systemID)
}

// ForceOff auxiliary method
func (c *RedfishClient) ForceOff(systemID string) (interface{}, error) {
	return c.ResetSystem("ForceOff", systemID)
}

// ForceRestart auxiliary method
func (c *RedfishClient) ForceRestart(systemID string) (interface{}, error) {
	return c.ResetSystem("ForceRestart", systemID)
}

// WaitForPowerState polls for a target power state
func (c *RedfishClient) WaitForPowerState(targetState, systemID string, timeout, interval int) (bool, error) {
	start := time.Now()
	for time.Since(start) < time.Duration(timeout)*time.Second {
		current, err := c.GetPowerState(systemID)
		if err == nil && current == targetState {
			return true, nil
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return false, fmt.Errorf("timed out waiting for power state %s", targetState)
}

// ResetManager performs a reset action on a manager
func (c *RedfishClient) ResetManager(resetType, managerID string) (interface{}, error) {
	url := fmt.Sprintf("%s/redfish/v1/Managers/%s/Actions/Manager.Reset", c.BaseURL, managerID)
	payload := map[string]string{"ResetType": resetType}
	body, err := c.DoRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return true, nil
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// FactoryReset performs a factory reset on a manager
func (c *RedfishClient) FactoryReset(managerID string) (interface{}, error) {
	// 1. Reset to Factory Defaults
	url := fmt.Sprintf("%s/redfish/v1/Managers/%s/Actions/Oem/Hpe/HpeiLO.ResetToFactoryDefaults/", c.BaseURL, managerID)
	payload := map[string]string{"ResetType": "Default"}
	_, err := c.DoRequest("POST", url, payload)
	if err != nil {
		return nil, fmt.Errorf("error resetting to factory defaults: %v", err)
	}

	// 2. Force Restart Manager
	return c.ResetManager("ForceRestart", managerID)
}

// AuxCycle performs an auxiliary power cycle
func (c *RedfishClient) AuxCycle(systemID string) (interface{}, error) {
	// 1. Force Off
	if _, err := c.ForceOff(systemID); err != nil {
		return nil, err
	}

	// 2. AuxCycle
	url := fmt.Sprintf("%s/redfish/v1/Systems/%s/Actions/Oem/Hpe/HpeComputerSystemExt.SystemReset/", c.BaseURL, systemID)
	payload := map[string]string{"ResetType": "AuxCycle"}
	body, err := c.DoRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return true, nil
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// GetPostState retrieves the HPE PostState
func (c *RedfishClient) GetPostState(systemID string) (string, error) {
	url := fmt.Sprintf("%s/redfish/v1/Systems/%s", c.BaseURL, systemID)
	body, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Oem struct {
			Hpe struct {
				PostState string `json:"PostState"`
			} `json:"Hpe"`
		} `json:"Oem"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("error unmarshaling PostState: %v", err)
	}
	return result.Oem.Hpe.PostState, nil
}

// GetServerConfigLockSettings retrieves server config lock settings
func (c *RedfishClient) GetServerConfigLockSettings(systemID string) (interface{}, error) {
	url := fmt.Sprintf("%s/redfish/v1/systems/%s/bios/oem/hpe/serverconfiglock/settings/", c.BaseURL, systemID)
	body, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling settings: %v", err)
	}
	return result, nil
}

// SecureErase triggers a secure erase of the system
func (c *RedfishClient) SecureErase(systemID string) (interface{}, error) {
	url := fmt.Sprintf("%s/redfish/v1/Systems/%s/Actions/Oem/Hpe/HpeComputerSystemExt.SecureSystemErase", c.BaseURL, systemID)
	payload := map[string]bool{"SystemROMAndiLOErase": true, "UserDataErase": true}
	if _, err := c.DoRequest("POST", url, payload); err != nil {
		return nil, err
	}

	return c.ForceRestart(systemID)
}

// GetSecureEraseStatus retrieves the status of a secure erase operation
func (c *RedfishClient) GetSecureEraseStatus(systemID string) (interface{}, error) {
	// Status from System
	url := fmt.Sprintf("%s/redfish/v1/Systems/%s", c.BaseURL, systemID)
	body, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var sysResult struct {
		Oem struct {
			Hpe struct {
				Status string `json:"SystemROMAndiLOEraseStatus"`
			} `json:"Hpe"`
		} `json:"Oem"`
	}
	json.Unmarshal(body, &sysResult)

	// Report from report entries
	reportURL := fmt.Sprintf("%s/redfish/v1/Systems/%s/SecureEraseReportService/SecureEraseReportEntries", c.BaseURL, systemID)
	reportBody, err := c.DoRequest("GET", reportURL, nil)
	if err != nil {
		return nil, err
	}

	var reportResult interface{}
	json.Unmarshal(reportBody, &reportResult)

	return map[string]interface{}{
		"SystemROMAndiLOEraseStatus": sysResult.Oem.Hpe.Status,
		"SecureEraseReportEntries":   reportResult,
	}, nil
}

// GetESKMLogs retrieves ESKM events
func (c *RedfishClient) GetESKMLogs(managerID string) (interface{}, error) {
	url := fmt.Sprintf("%s/redfish/v1/Managers/%s/SecurityService/ESKM/", c.BaseURL, managerID)
	body, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Events []interface{} `json:"ESKMEvents"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling ESKM logs: %v", err)
	}
	return result.Events, nil
}

// TestESKMConnection triggers an ESKM connection test
func (c *RedfishClient) TestESKMConnection(managerID string) (interface{}, error) {
	url := fmt.Sprintf("%s/redfish/v1/Managers/%s/SecurityService/ESKM/Actions/HpeESKM.TestESKMConnections/", c.BaseURL, managerID)
	body, err := c.DoRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return true, nil
	}

	var result interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// GetSecurityState retrieves the security state of a manager
func (c *RedfishClient) GetSecurityState(managerID string) (string, error) {
	url := fmt.Sprintf("%s/redfish/v1/Managers/%s/SecurityService", c.BaseURL, managerID)
	body, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		SecurityState string `json:"SecurityState"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("error unmarshaling security state: %v", err)
	}
	return result.SecurityState, nil
}

// GetServerDetails retrieves BMC details from Kubernetes using kubectl
func GetServerDetails(serverName string) (string, string, string, error) {
	// Get server JSON
	cmd := exec.Command("kubectl", "get", "server/"+serverName, "-n", "gpc-system", "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", "", fmt.Errorf("error running kubectl get server: %v (output: %s)", err, string(output))
	}

	var serverData struct {
		Spec struct {
			Bmc struct {
				IP             string `json:"ip"`
				CredentialsRef struct {
					Name string `json:"name"`
				} `json:"credentialsRef"`
			} `json:"bmc"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(output, &serverData); err != nil {
		return "", "", "", fmt.Errorf("error decoding server JSON: %v", err)
	}

	// Get secret JSON
	secretCmd := exec.Command("kubectl", "get", "secret", serverData.Spec.Bmc.CredentialsRef.Name, "-n", "gpc-system", "-o", "json")
	secretOutput, err := secretCmd.CombinedOutput()
	if err != nil {
		return "", "", "", fmt.Errorf("error running kubectl get secret: %v (output: %s)", err, string(secretOutput))
	}

	var secretData struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(secretOutput, &secretData); err != nil {
		return "", "", "", fmt.Errorf("error decoding secret JSON: %v", err)
	}

	userB64, okUser := secretData.Data["username"]
	passB64, okPass := secretData.Data["password"]
	if !okUser || !okPass {
		return "", "", "", fmt.Errorf("username or password missing in secret")
	}

	user, _ := base64.StdEncoding.DecodeString(userB64)
	pass, _ := base64.StdEncoding.DecodeString(passB64)

	return serverData.Spec.Bmc.IP, string(user), string(pass), nil
}

// ListServers lists all servers from Kubernetes using kubectl
func ListServers() error {
	cmd := exec.Command("kubectl", "get", "servers", "-n", "gpc-system", "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running kubectl get servers: %v (output: %s)", err, string(output))
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				ManagementNetwork struct {
					IPs []string `json:"ips"`
				} `json:"managementNetwork"`
				Bmc struct {
					IP string `json:"ip"`
				} `json:"bmc"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return fmt.Errorf("error decoding servers list: %v", err)
	}

	fmt.Printf("%-30s %-20s %-20s\n", "NAME", "MANAGEMENT IP", "BMC IP")
	fmt.Println("------------------------------------------------------------------------")
	for _, item := range list.Items {
		mgmtIP := "N/A"
		if len(item.Spec.ManagementNetwork.IPs) > 0 {
			mgmtIP = item.Spec.ManagementNetwork.IPs[0]
		}
		bmcIP := item.Spec.Bmc.IP
		if bmcIP == "" {
			bmcIP = "N/A"
		}
		fmt.Printf("%-30s %-20s %-20s\n", item.Metadata.Name, mgmtIP, bmcIP)
	}
	return nil
}

func main() {
	if os.Getenv("KUBECONFIG") == "" {
		fmt.Println("Warning: KUBECONFIG is not specified. Setting default to /root/release/root-admin/root-admin-kubeconfig")
		os.Setenv("KUBECONFIG", "/root/release/root-admin/root-admin-kubeconfig")
	} else {
		fmt.Printf("KUBECONFIG is set to: %s\n", os.Getenv("KUBECONFIG"))
	}

	serverName := flag.String("server-name", "", "Name of the server to connect to (via kubectl)")
	listServersOpt := flag.Bool("list-servers", false, "List available servers with IPs")
	verbose := flag.Bool("v", false, "Enable verbose logging of API calls (placeholder for future go-native logging)")
	action := flag.String("action", "", "Action to perform")
	systemID := flag.String("system-id", "1", "System ID (default: 1)")
	managerID := flag.String("manager-id", "1", "Manager ID (default: 1)")
	resetType := flag.String("reset-type", "", "Reset Type for reset_system or reset_manager")
	targetState := flag.String("target-state", "", "Target Power State for wait_for_power_state")
	timeout := flag.Int("timeout", 60, "Timeout in seconds (default: 60)")
	interval := flag.Int("interval", 5, "Interval in seconds (default: 5)")

	flag.Parse()

	if *listServersOpt {
		if err := ListServers(); err != nil {
			fmt.Printf("Error listing servers: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *serverName == "" {
		fmt.Println("Error: --server-name is required unless --list-servers is used.")
		os.Exit(1)
	}

	if *action == "" {
		fmt.Println("Error: --action is required.")
		os.Exit(1)
	}

	ip, user, pass, err := GetServerDetails(*serverName)
	if err != nil {
		fmt.Printf("Failed to retrieve server details: %v\n", err)
		os.Exit(1)
	}

	baseURL := fmt.Sprintf("https://%s", ip)
	client := NewRedfishClient(baseURL, user, pass)

	if *verbose {
		// Go native http logging can be complex to match Python's simple setup,
		// but for now we just acknowledge the flag.
		fmt.Println("Verbose mode enabled")
	}

	if _, err := client.Login(); err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	var result interface{}
	var actionErr error

	switch *action {
	case "get_power_state":
		result, actionErr = client.GetPowerState(*systemID)
	case "reset_system":
		if *resetType == "" {
			fmt.Println("Error: --reset-type is required for reset_system")
			os.Exit(1)
		}
		result, actionErr = client.ResetSystem(*resetType, *systemID)
	case "power_on":
		result, actionErr = client.PowerOn(*systemID)
	case "graceful_shutdown":
		result, actionErr = client.GracefulShutdown(*systemID)
	case "force_off":
		result, actionErr = client.ForceOff(*systemID)
	case "force_restart":
		result, actionErr = client.ForceRestart(*systemID)
	case "wait_for_power_state":
		if *targetState == "" {
			fmt.Println("Error: --target-state is required for wait_for_power_state")
			os.Exit(1)
		}
		result, actionErr = client.WaitForPowerState(*targetState, *systemID, *timeout, *interval)
	case "reset_manager":
		rt := *resetType
		if rt == "" {
			rt = "ForceRestart"
		}
		result, actionErr = client.ResetManager(rt, *managerID)
	case "factory_reset":
		result, actionErr = client.FactoryReset(*managerID)
	case "aux_cycle":
		result, actionErr = client.AuxCycle(*systemID)
	case "get_post_state":
		result, actionErr = client.GetPostState(*systemID)
	case "secure_erase":
		result, actionErr = client.SecureErase(*systemID)
	case "get_secure_erase_status":
		result, actionErr = client.GetSecureEraseStatus(*systemID)
	case "get_eskm_logs":
		result, actionErr = client.GetESKMLogs(*managerID)
	case "test_eskm_connection":
		result, actionErr = client.TestESKMConnection(*managerID)
	case "get_security_state":
		result, actionErr = client.GetSecurityState(*managerID)
	case "get_server_config_lock_settings":
		result, actionErr = client.GetServerConfigLockSettings(*systemID)
	default:
		fmt.Printf("Error: Unknown action %s\n", *action)
		os.Exit(1)
	}

	if actionErr != nil {
		fmt.Printf("Error performing action: %v\n", actionErr)
		os.Exit(1)
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(output))
}
