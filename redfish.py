import sys
import time
import subprocess
import json
import base64
import argparse

try:
    import requests
except ImportError:
    print("Error: 'requests' module not found. Please install it using 'pip install requests'")
    sys.exit(1)


class RedfishClient:
    def __init__(self, base_url, username, password):
        self.base_url = base_url
        self.username = username
        self.password = password
        self.session = None

    def login(self):
        try:
            self.session = requests.Session()
            self.session.verify = False
            self.session.auth = (self.username, self.password)
            self.session.headers.update({"Content-Type": "application/json"})
            response = self.session.get(self.base_url)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            print(f"Error logging in: {e}")
            return None

    def logout(self):
        try:
            if self.session:
                self.session.close()
            return True
        except requests.exceptions.RequestException as e:
            print(f"Error logging out: {e}")
            return False

    def get_power_state(self, system_id="1"):
        try:
            response = self.session.get(f"{self.base_url}/redfish/v1/Systems/{system_id}")
            response.raise_for_status()
            return response.json().get('PowerState')
        except requests.exceptions.RequestException as e:
            print(f"Error getting power state: {e}")
            return None

    def reset_system(self, reset_type, system_id="1"):
        payload = {'ResetType': reset_type}
        try:
            response = self.session.post(
                f"{self.base_url}/redfish/v1/Systems/{system_id}/Actions/ComputerSystem.Reset",
                json=payload
            )
            response.raise_for_status()
            return response.json() if response.content else True
        except requests.exceptions.RequestException as e:
            print(f"Error resetting system ({reset_type}): {e}")
            return None

    def power_on(self, system_id="1"):
        return self.reset_system("On", system_id)

    def graceful_shutdown(self, system_id="1"):
        return self.reset_system("GracefulShutdown", system_id)

    def force_off(self, system_id="1"):
        return self.reset_system("ForceOff", system_id)

    def force_restart(self, system_id="1"):
        return self.reset_system("ForceRestart", system_id)

    def wait_for_power_state(self, target_state, system_id="1", timeout=60, interval=5):
        start_time = time.time()
        while time.time() - start_time < timeout:
            current_state = self.get_power_state(system_id)
            if current_state == target_state:
                return True
            time.sleep(interval)
        print(f"Timed out waiting for power state {target_state}")
        return False

    def reset_manager(self, reset_type="ForceRestart", manager_id="1"):
        payload = {'ResetType': reset_type}
        try:
            response = self.session.post(
                f"{self.base_url}/redfish/v1/Managers/{manager_id}/Actions/Manager.Reset",
                json=payload
            )
            response.raise_for_status()
            return response.json() if response.content else True
        except requests.exceptions.RequestException as e:
            print(f"Error resetting manager ({reset_type}): {e}")
            return None

    def factory_reset(self, manager_id="1"):
        try:
            # 1. Reset to Factory Defaults
            payload = {'ResetType': 'Default'}
            response = self.session.post(
                f"{self.base_url}/redfish/v1/Managers/{manager_id}/Actions/Oem/Hpe/HpeiLO.ResetToFactoryDefaults/",
                json=payload
            )
            response.raise_for_status()
            
            # 2. Force Restart Manager
            return self.reset_manager("ForceRestart", manager_id)
        except requests.exceptions.RequestException as e:
            print(f"Error performing factory reset: {e}")
            return None

    def aux_cycle(self, system_id="1"):
        try:
            # 1. Force Off
            self.force_off(system_id)
            
            # 2. AuxCycle
            payload = {'ResetType': 'AuxCycle'}
            response = self.session.post(
                f"{self.base_url}/redfish/v1/Systems/{system_id}/Actions/Oem/Hpe/HpeComputerSystemExt.SystemReset/",
                json=payload
            )
            response.raise_for_status()
            return response.json() if response.content else True
        except requests.exceptions.RequestException as e:
            print(f"Error performing AuxCycle: {e}")
            return None

    def get_post_state(self, system_id="1"):
        try:
            response = self.session.get(f"{self.base_url}/redfish/v1/Systems/{system_id}")
            response.raise_for_status()
            return response.json().get('Oem', {}).get('Hpe', {}).get('PostState')
        except requests.exceptions.RequestException as e:
            print(f"Error getting PostState: {e}")
            return None

    def get_server_config_lock_settings(self, system_id="1"):
         try:
            response = self.session.get(f"{self.base_url}/redfish/v1/systems/{system_id}/bios/oem/hpe/serverconfiglock/settings/")
            response.raise_for_status()
            return response.json()
         except requests.exceptions.RequestException as e:
            print(f"Error getting ServerConfigLock settings: {e}")
            return None

    def secure_erase(self, system_id="1"):
        try:
            payload = {'SystemROMAndiLOErase': True, 'UserDataErase': True}
            response = self.session.post(
                f"{self.base_url}/redfish/v1/Systems/{system_id}/Actions/Oem/Hpe/HpeComputerSystemExt.SecureSystemErase",
                json=payload
            )
            response.raise_for_status()
            
            # Trigger ForceRestart as per instructions
            return self.force_restart(system_id)
        except requests.exceptions.RequestException as e:
            print(f"Error performing Secure Erase: {e}")
            return None

    def get_secure_erase_status(self, system_id="1"):
        try:
            response = self.session.get(f"{self.base_url}/redfish/v1/Systems/{system_id}")
            response.raise_for_status()
            status = response.json().get('Oem', {}).get('Hpe', {}).get('SystemROMAndiLOEraseStatus')
            
            report_response = self.session.get(f"{self.base_url}/redfish/v1/Systems/{system_id}/SecureEraseReportService/SecureEraseReportEntries")
            report_response.raise_for_status()
            report = report_response.json()
            
            return {'SystemROMAndiLOEraseStatus': status, 'SecureEraseReportEntries': report}
        except requests.exceptions.RequestException as e:
            print(f"Error getting Secure Erase status: {e}")
            return None

    def get_eskm_logs(self, manager_id="1"):
        try:
            response = self.session.get(f"{self.base_url}/redfish/v1/Managers/{manager_id}/SecurityService/ESKM/")
            response.raise_for_status()
            # The curl command ends with | jq '.ESKMEvents[]' which implies returning the list
            return response.json().get('ESKMEvents', [])
        except requests.exceptions.RequestException as e:
            print(f"Error getting ESKM logs: {e}")
            return None

    def test_eskm_connection(self, manager_id="1"):
        try:
            response = self.session.post(
                f"{self.base_url}/redfish/v1/Managers/{manager_id}/SecurityService/ESKM/Actions/HpeESKM.TestESKMConnections/"
            )
            response.raise_for_status()
            return response.json() if response.content else True
        except requests.exceptions.RequestException as e:
            print(f"Error testing ESKM connection: {e}")
            return None

    def get_security_state(self, manager_id="1"):
        try:
            response = self.session.get(f"{self.base_url}/redfish/v1/Managers/{manager_id}/SecurityService")
            response.raise_for_status()
            return response.json().get('SecurityState')
        except requests.exceptions.RequestException as e:
            print(f"Error getting Security State: {e}")
            return None


def get_server_details(server_name):
    try:
        # Get server details
        cmd = ["kubectl", "get", f"server/{server_name}", "-n", "gpc-system", "-o", "json"]
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        server_data = json.loads(result.stdout)
        
        ip = server_data.get('spec', {}).get('bmc', {}).get('ip')
        secret_name = server_data.get('spec', {}).get('bmc', {}).get('credentialsRef', {}).get('name')
        
        if not ip or not secret_name:
            print(f"Error: Could not find IP or credentialsRef for server {server_name}")
            return None, None, None

        # Get secret details
        cmd_secret = ["kubectl", "get", "secret", secret_name, "-n", "gpc-system", "-o", "json"]
        result_secret = subprocess.run(cmd_secret, capture_output=True, text=True, check=True)
        secret_data = json.loads(result_secret.stdout)
        
        username_b64 = secret_data.get('data', {}).get('username')
        password_b64 = secret_data.get('data', {}).get('password')
        
        if not username_b64 or not password_b64:
             print(f"Error: Could not find username or password in secret {secret_name}")
             return None, None, None

        username = base64.b64decode(username_b64).decode('utf-8')
        password = base64.b64decode(password_b64).decode('utf-8')
        
        return ip, username, password

    except subprocess.CalledProcessError as e:
        print(f"Error running kubectl: {e}")
        return None, None, None
    except Exception as e:
         print(f"Error processing server details: {e}")
         return None, None, None

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Redfish Client Utility")
    parser.add_argument("server_name", help="Name of the server to connect to (via kubectl)")
    parser.add_argument("--action", required=True, help="Action to perform", choices=[
        "get_power_state", "reset_system", "power_on", "graceful_shutdown", "force_off", "force_restart",
        "wait_for_power_state", "reset_manager", "factory_reset", "aux_cycle", "get_post_state",
        "secure_erase", "get_secure_erase_status", "get_eskm_logs", "test_eskm_connection",
        "get_security_state", "get_server_config_lock_settings"
    ])
    parser.add_argument("--system-id", default="1", help="System ID (default: 1)")
    parser.add_argument("--manager-id", default="1", help="Manager ID (default: 1)")
    parser.add_argument("--reset-type", help="Reset Type for reset_system or reset_manager")
    parser.add_argument("--target-state", help="Target Power State for wait_for_power_state")
    parser.add_argument("--timeout", type=int, default=60, help="Timeout in seconds (default: 60)")
    parser.add_argument("--interval", type=int, default=5, help="Interval in seconds (default: 5)")
    
    args = parser.parse_args()

    ip, username, password = get_server_details(args.server_name)
    
    if ip and username and password:
        # Suppress insecure request warnings
        requests.packages.urllib3.disable_warnings(requests.packages.urllib3.exceptions.InsecureRequestWarning)
        
        base_url = f"https://{ip}"
        client = RedfishClient(base_url, username, password)
        
        if client.login():
            try:
                result = None
                if args.action == "get_power_state":
                    result = client.get_power_state(args.system_id)
                elif args.action == "reset_system":
                    if not args.reset_type:
                        print("Error: --reset-type is required for reset_system")
                    else:
                        result = client.reset_system(args.reset_type, args.system_id)
                elif args.action == "power_on":
                    result = client.power_on(args.system_id)
                elif args.action == "graceful_shutdown":
                    result = client.graceful_shutdown(args.system_id)
                elif args.action == "force_off":
                    result = client.force_off(args.system_id)
                elif args.action == "force_restart":
                    result = client.force_restart(args.system_id)
                elif args.action == "wait_for_power_state":
                    if not args.target_state:
                         print("Error: --target-state is required for wait_for_power_state")
                    else:
                        result = client.wait_for_power_state(args.target_state, args.system_id, args.timeout, args.interval)
                elif args.action == "reset_manager":
                    # Default reset type handled in method if not provided, but here we can pass it if provided
                    rt = args.reset_type if args.reset_type else "ForceRestart"
                    result = client.reset_manager(rt, args.manager_id)
                elif args.action == "factory_reset":
                    result = client.factory_reset(args.manager_id)
                elif args.action == "aux_cycle":
                    result = client.aux_cycle(args.system_id)
                elif args.action == "get_post_state":
                    result = client.get_post_state(args.system_id)
                elif args.action == "secure_erase":
                    result = client.secure_erase(args.system_id)
                elif args.action == "get_secure_erase_status":
                    result = client.get_secure_erase_status(args.system_id)
                elif args.action == "get_eskm_logs":
                    result = client.get_eskm_logs(args.manager_id)
                elif args.action == "test_eskm_connection":
                    result = client.test_eskm_connection(args.manager_id)
                elif args.action == "get_security_state":
                    result = client.get_security_state(args.manager_id)
                elif args.action == "get_server_config_lock_settings":
                    result = client.get_server_config_lock_settings(args.system_id)
                
                print(json.dumps(result, indent=2))
            finally:
                client.logout()
        else:
            print("Login failed.")
            sys.exit(1)
    else:
        print("Failed to retrieve server details.")
        sys.exit(1)


