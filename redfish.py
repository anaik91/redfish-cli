import requests
import time

class RedfishClient:
    def __init__(self, base_url, username, password):
        self.base_url = base_url
        self.username = username
        self.password = password
        self.session = None

    def login(self):
        try:
            self.session = requests.Session()
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


