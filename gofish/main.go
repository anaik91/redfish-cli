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
	"net/http/httputil"
	"os"
	"os/exec"
	"time"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

// loggingTransport wraps an http.RoundTripper for logging and safety prompts
type loggingTransport struct {
	transport http.RoundTripper
	verbose   bool
	assumeYes bool
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	if req.Method == "POST" && !t.assumeYes {
		fmt.Printf("\n--- SAFETY CONFIRMATION ---\n")
		fmt.Printf("Method:  %s\n", req.Method)
		fmt.Printf("URL:     %s\n", req.URL.String())
		fmt.Printf("Payload: %s\n", string(bodyBytes))
		fmt.Printf("---------------------------\n")
		fmt.Print("Proceed? (y/N): ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return nil, fmt.Errorf("action cancelled by user")
		}
	}

	if t.verbose {
		dump, _ := httputil.DumpRequestOut(req, true)
		fmt.Printf("\n--- Request ---\n%s\n", string(dump))
	}

	resp, err := t.transport.RoundTrip(req)

	if err == nil && t.verbose {
		dump, _ := httputil.DumpResponse(resp, true)
		fmt.Printf("\n--- Response ---\n%s\n----------------\n", string(dump))
	}

	return resp, err
}

// RedfishClient wraps gofish APIClient
type RedfishClient struct {
	Client *gofish.APIClient
}

// NewRedfishClient creates a new gofish client
func NewRedfishClient(endpoint, username, password string, verbose, assumeYes bool) (*RedfishClient, error) {
	config := gofish.ClientConfig{
		Endpoint:  endpoint,
		Username:  username,
		Password:  password,
		Insecure:  true,
		BasicAuth: true,
		HTTPClient: &http.Client{
			Transport: &loggingTransport{
				transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
				verbose:   verbose,
				assumeYes: assumeYes,
			},
			Timeout: 30 * time.Second,
		},
	}

	c, err := gofish.Connect(config)
	if err != nil {
		return nil, err
	}
	return &RedfishClient{Client: c}, nil
}

// GetPowerState retrieves the power state of a system
func (c *RedfishClient) GetPowerState(systemID string) (string, error) {
	systems, err := c.Client.Service.Systems()
	if err != nil {
		return "", err
	}
	for _, sys := range systems {
		if sys.ID == systemID {
			return string(sys.PowerState), nil
		}
	}
	return "", fmt.Errorf("system %s not found", systemID)
}

// ResetSystem performs a reset action
func (c *RedfishClient) ResetSystem(resetType, systemID string) (bool, error) {
	systems, err := c.Client.Service.Systems()
	if err != nil {
		return false, err
	}
	for _, sys := range systems {
		if sys.ID == systemID {
			err := sys.Reset(redfish.ResetType(resetType))
			return err == nil, err
		}
	}
	return false, fmt.Errorf("system %s not found", systemID)
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
func (c *RedfishClient) ResetManager(resetType, managerID string) (bool, error) {
	managers, err := c.Client.Service.Managers()
	if err != nil {
		return false, err
	}
	for _, mgr := range managers {
		if mgr.ID == managerID {
			err := mgr.Reset(redfish.ResetType(resetType))
			return err == nil, err
		}
	}
	return false, fmt.Errorf("manager %s not found", managerID)
}

// FactoryReset performs a factory reset on a manager
func (c *RedfishClient) FactoryReset(managerID string) (bool, error) {
	url := fmt.Sprintf("/redfish/v1/Managers/%s/Actions/Oem/Hpe/HpeiLO.ResetToFactoryDefaults/", managerID)
	payload := map[string]string{"ResetType": "Default"}
	_, err := c.Client.Post(url, payload)
	if err != nil {
		return false, err
	}
	return c.ResetManager("ForceRestart", managerID)
}

// AuxCycle performs an auxiliary power cycle (OEM)
func (c *RedfishClient) AuxCycle(systemID string) (bool, error) {
	url := fmt.Sprintf("/redfish/v1/Systems/%s/Actions/Oem/Hpe/HpeComputerSystemExt.SystemReset/", systemID)
	payload := map[string]string{"ResetType": "AuxCycle"}
	_, err := c.Client.Post(url, payload)
	return err == nil, err
}

// GetPostState retrieves the HPE PostState (OEM extension)
func (c *RedfishClient) GetPostState(systemID string) (string, error) {
	systems, err := c.Client.Service.Systems()
	if err != nil {
		return "", err
	}
	for _, sys := range systems {
		if sys.ID == systemID {
			var oemData struct {
				Hpe struct {
					PostState string `json:"PostState"`
				} `json:"Hpe"`
			}
			json.Unmarshal(sys.OEM, &oemData)
			return oemData.Hpe.PostState, nil
		}
	}
	return "", fmt.Errorf("system %s not found", systemID)
}

// SecureErase triggers a secure erase (OEM)
func (c *RedfishClient) SecureErase(systemID string) (bool, error) {
	url := fmt.Sprintf("/redfish/v1/Systems/%s/Actions/Oem/Hpe/HpeComputerSystemExt.SecureSystemErase", systemID)
	payload := map[string]bool{"SystemROMAndiLOErase": true, "UserDataErase": true}
	_, err := c.Client.Post(url, payload)
	if err != nil {
		return false, err
	}
	return c.ResetSystem("ForceRestart", systemID)
}

// GetSecureEraseStatus retrieves the status of a secure erase operation
func (c *RedfishClient) GetSecureEraseStatus(systemID string) (interface{}, error) {
	systems, err := c.Client.Service.Systems()
	if err != nil {
		return nil, err
	}
	var sysStatus string
	for _, sys := range systems {
		if sys.ID == systemID {
			var oemData struct {
				Hpe struct {
					Status string `json:"SystemROMAndiLOEraseStatus"`
				} `json:"Hpe"`
			}
			json.Unmarshal(sys.OEM, &oemData)
			sysStatus = oemData.Hpe.Status
			break
		}
	}
	reportURL := fmt.Sprintf("/redfish/v1/Systems/%s/SecureEraseReportService/SecureEraseReportEntries", systemID)
	resp, err := c.Client.Get(reportURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var reportResult interface{}
	json.NewDecoder(resp.Body).Decode(&reportResult)
	return map[string]interface{}{"SystemROMAndiLOEraseStatus": sysStatus, "SecureEraseReportEntries": reportResult}, nil
}

// GetESKMLogs retrieves ESKM events
func (c *RedfishClient) GetESKMLogs(managerID string) (interface{}, error) {
	url := fmt.Sprintf("/redfish/v1/Managers/%s/SecurityService/ESKM/", managerID)
	resp, err := c.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Events []interface{} `json:"ESKMEvents"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Events, nil
}

// TestESKMConnection triggers an ESKM connection test
func (c *RedfishClient) TestESKMConnection(managerID string) (bool, error) {
	url := fmt.Sprintf("/redfish/v1/Managers/%s/SecurityService/ESKM/Actions/HpeESKM.TestESKMConnections/", managerID)
	_, err := c.Client.Post(url, nil)
	return err == nil, err
}

// GetSecurityState retrieves the security state of a manager
func (c *RedfishClient) GetSecurityState(managerID string) (string, error) {
	url := fmt.Sprintf("/redfish/v1/Managers/%s/SecurityService", managerID)
	resp, err := c.Client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		SecurityState string `json:"SecurityState"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.SecurityState, nil
}

// GetServerConfigLockSettings retrieves server config lock settings
func (c *RedfishClient) GetServerConfigLockSettings(systemID string) (interface{}, error) {
	url := fmt.Sprintf("/redfish/v1/systems/%s/bios/oem/hpe/serverconfiglock/settings/", systemID)
	resp, err := c.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

// GetServerDetails retrieves BMC details from Kubernetes using kubectl
func GetServerDetails(serverName string) (string, string, string, error) {
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
	json.Unmarshal(output, &serverData)
	secretCmd := exec.Command("kubectl", "get", "secret", serverData.Spec.Bmc.CredentialsRef.Name, "-n", "gpc-system", "-o", "json")
	secretOutput, err := secretCmd.CombinedOutput()
	if err != nil {
		return "", "", "", fmt.Errorf("error running kubectl get secret: %v (output: %s)", err, string(secretOutput))
	}
	var secretData struct {
		Data map[string]string `json:"data"`
	}
	json.Unmarshal(secretOutput, &secretData)
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
	json.Unmarshal(output, &list)
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
		os.Setenv("KUBECONFIG", "/root/release/root-admin/root-admin-kubeconfig")
	}

	serverName := flag.String("server-name", "", "Name of the server to connect to (via kubectl)")
	listServersOpt := flag.Bool("list-servers", false, "List available servers with IPs")
	verbose := flag.Bool("v", false, "Enable verbose logging of API calls")
	availableActions := "get_power_state, reset_system, power_on, graceful_shutdown, force_off, force_restart, wait_for_power_state, reset_manager, factory_reset, aux_cycle, get_post_state, secure_erase, get_secure_erase_status, get_eskm_logs, test_eskm_connection, get_security_state, get_server_config_lock_settings"
	action := flag.String("action", "", "Action to perform: "+availableActions)
	systemID := flag.String("system-id", "1", "System ID (default: 1)")
	managerID := flag.String("manager-id", "1", "Manager ID (default: 1)")
	resetType := flag.String("reset-type", "", "Reset Type for reset_system or reset_manager")
	targetState := flag.String("target-state", "", "Target Power State for wait_for_power_state")
	timeout := flag.Int("timeout", 60, "Timeout in seconds (default: 60)")
	interval := flag.Int("interval", 5, "Interval in seconds (default: 5)")
	assumeYes := flag.Bool("y", false, "Assume yes; non-interactive mode")
	flag.BoolVar(assumeYes, "yes", false, "Assume yes; assume 'y' as answer to all prompts and run non-interactively")

	flag.Parse()

	if *listServersOpt {
		ListServers()
		os.Exit(0)
	}

	if *serverName == "" || *action == "" {
		fmt.Println("Error: --server-name and --action are required.")
		os.Exit(1)
	}

	ip, user, pass, err := GetServerDetails(*serverName)
	if err != nil {
		fmt.Printf("Failed to retrieve server details: %v\n", err)
		os.Exit(1)
	}

	baseURL := fmt.Sprintf("https://%s", ip)
	client, err := NewRedfishClient(baseURL, user, pass, *verbose, *assumeYes)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Client.Logout()

	var result interface{}
	var actionErr error

	switch *action {
	case "get_power_state":
		result, actionErr = client.GetPowerState(*systemID)
	case "reset_system":
		result, actionErr = client.ResetSystem(*resetType, *systemID)
	case "power_on":
		result, actionErr = client.ResetSystem("On", *systemID)
	case "graceful_shutdown":
		result, actionErr = client.ResetSystem("GracefulShutdown", *systemID)
	case "force_off":
		result, actionErr = client.ResetSystem("ForceOff", *systemID)
	case "force_restart":
		result, actionErr = client.ResetSystem("ForceRestart", *systemID)
	case "wait_for_power_state":
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
