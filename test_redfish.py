import sys
import json
import base64
from unittest.mock import MagicMock

# Mock requests module before importing redfish
mock_requests = MagicMock()
mock_requests.exceptions.RequestException = Exception
sys.modules['requests'] = mock_requests

import unittest
from redfish import RedfishClient

class TestRedfishClient(unittest.TestCase):
    def setUp(self):
        self.client = RedfishClient("http://mock.url", "user", "pass")
        # Mock the session directly
        self.client.session = MagicMock()
        # Mock headers update which is done in login but we skip login for unit tests usage of 'session'
        # Actually in the code, session is created in login().
        # So we just mock it on the object directly as we did above.

    def test_get_power_state(self):
        self.client.session.get.return_value.json.return_value = {'PowerState': 'On'}
        state = self.client.get_power_state()
        self.assertEqual(state, 'On')
        self.client.session.get.assert_called_with("http://mock.url/redfish/v1/Systems/1")

    def test_reset_system(self):
        self.client.reset_system("On")
        self.client.session.post.assert_called_with(
            "http://mock.url/redfish/v1/Systems/1/Actions/ComputerSystem.Reset",
            json={'ResetType': 'On'}
        )

    def test_convenience_methods(self):
        # We replace the bound method with a mock to check calls from convenience methods
        self.client.reset_system = MagicMock()
        
        self.client.power_on()
        self.client.reset_system.assert_called_with("On", "1")
        
        self.client.graceful_shutdown()
        self.client.reset_system.assert_called_with("GracefulShutdown", "1")

        self.client.force_off()
        self.client.reset_system.assert_called_with("ForceOff", "1")

        self.client.force_restart()
        self.client.reset_system.assert_called_with("ForceRestart", "1")


    def test_wait_for_power_state(self):
        # Mock time to control loop and avoid actual sleep
        import time
        original_sleep = time.sleep
        original_time = time.time
        
        try:
            time.sleep = MagicMock()
            # Mock time.time to simulate passage of time or just enough iterations
            # First call: start_time
            # Second call: loop check 1 (enters)
            # Third call: loop check 2 (enters)
            # Fourth call: loop check 3 (timeout)
            
            # Scenario: Successful after one retry
            # time.time() side effects: start, check 1 (ok), check 2 (timeout if needed)
            # Actually simpler:
            # We want get_power_state to return "Off" then "On"
            
            self.client.get_power_state = MagicMock(side_effect=["Off", "On"])
            
            # reset time.time to just increase normally if we want, or mock it.
            # simpler to not mock time.time if we just mock get_power_state to succeed quickly.
            # But we should mock sleep to speed up test.
            
            result = self.client.wait_for_power_state("On", interval=0.1, timeout=1)
            self.assertTrue(result)
            self.assertEqual(self.client.get_power_state.call_count, 2)
            
        finally:
            time.sleep = original_sleep
            time.time = original_time


    def test_reset_manager(self):
        self.client.reset_manager("ForceRestart")
        self.client.session.post.assert_called_with(
            "http://mock.url/redfish/v1/Managers/1/Actions/Manager.Reset",
            json={'ResetType': 'ForceRestart'}
        )

    def test_factory_reset(self):
        # Flatten mock calls list
        self.client.session.post.reset_mock()
        self.client.factory_reset()
        
        # Check calls
        # 1. ResetToFactoryDefaults
        # 2. Manager Reset
        calls = self.client.session.post.call_args_list
        self.assertEqual(len(calls), 2)
        
        args1, kwargs1 = calls[0]
        self.assertIn("HpeiLO.ResetToFactoryDefaults/", args1[0])
        self.assertEqual(kwargs1['json'], {'ResetType': 'Default'})
        
        args2, kwargs2 = calls[1]
        self.assertIn("Actions/Manager.Reset", args2[0])
        self.assertEqual(kwargs2['json'], {'ResetType': 'ForceRestart'})

    def test_aux_cycle(self):
        self.client.session.post.reset_mock()
        # Mock force_off to avoid calling original reset_system which calls session.post
        # But we want to test that force_off is called, which calls reset_system("ForceOff")
        # So we let it call through.
        
        self.client.aux_cycle()
        
        calls = self.client.session.post.call_args_list
        # 1. ForceOff
        # 2. AuxCycle
        self.assertEqual(len(calls), 2)
        
        args1, kwargs1 = calls[0]
        self.assertIn("ComputerSystem.Reset", args1[0])
        self.assertEqual(kwargs1['json'], {'ResetType': 'ForceOff'})
        
        args2, kwargs2 = calls[1]
        self.assertIn("HpeComputerSystemExt.SystemReset/", args2[0])
        self.assertEqual(kwargs2['json'], {'ResetType': 'AuxCycle'})

    def test_get_post_state(self):
        self.client.session.get.return_value.json.return_value = {
            'Oem': {'Hpe': {'PostState': 'FinishedPost'}}
        }
        state = self.client.get_post_state()
        self.assertEqual(state, 'FinishedPost')
        self.client.session.get.assert_called_with("http://mock.url/redfish/v1/Systems/1")

    def test_secure_erase(self):
        self.client.session.post.reset_mock()
        self.client.secure_erase()
        
        calls = self.client.session.post.call_args_list
        # 1. SecureSystemErase
        # 2. ForceRestart
        self.assertEqual(len(calls), 2)
        
        args1, kwargs1 = calls[0]
        self.assertIn("HpeComputerSystemExt.SecureSystemErase", args1[0])
        self.assertEqual(kwargs1['json'], {'SystemROMAndiLOErase': True, 'UserDataErase': True})
        
        args2, kwargs2 = calls[1]
        self.assertIn("ComputerSystem.Reset", args2[0])
        self.assertEqual(kwargs2['json'], {'ResetType': 'ForceRestart'})

    def test_get_secure_erase_status(self):
        self.client.session.get.side_effect = [
            MagicMock(json=MagicMock(return_value={'Oem': {'Hpe': {'SystemROMAndiLOEraseStatus': 'Completed'}}})),
            MagicMock(json=MagicMock(return_value={'Entries': []})),
        ]
        
        status = self.client.get_secure_erase_status()
        self.assertEqual(status['SystemROMAndiLOEraseStatus'], 'Completed')
        self.assertEqual(self.client.session.get.call_count, 2)



    def test_get_eskm_logs(self):
        self.client.session.get.return_value.json.return_value = {'ESKMEvents': ['log1', 'log2']}
        logs = self.client.get_eskm_logs()
        self.assertEqual(logs, ['log1', 'log2'])
        self.client.session.get.assert_called_with("http://mock.url/redfish/v1/Managers/1/SecurityService/ESKM/")

    def test_test_eskm_connection(self):
        self.client.test_eskm_connection()
        self.client.session.post.assert_called_with(
            "http://mock.url/redfish/v1/Managers/1/SecurityService/ESKM/Actions/HpeESKM.TestESKMConnections/"
        )

    def test_get_security_state(self):
        self.client.session.get.return_value.json.return_value = {'SecurityState': 'Production'}
        state = self.client.get_security_state()
        self.assertEqual(state, 'Production')
        self.client.session.get.assert_called_with("http://mock.url/redfish/v1/Managers/1/SecurityService")


    def test_get_get_server_details(self):
        with unittest.mock.patch('subprocess.run') as mock_run:
            # Mock get server
            mock_run.side_effect = [
                MagicMock(stdout=json.dumps({
                    'spec': {'bmc': {'ip': '1.2.3.4', 'credentialsRef': {'name': 'secret-name'}}}
                })),
                # Mock get secret
                MagicMock(stdout=json.dumps({
                    'data': {
                        'username': base64.b64encode(b'user').decode('utf-8'),
                        'password': base64.b64encode(b'pass').decode('utf-8')
                    }
                }))
            ]
            
            from redfish import get_server_details
            ip, user, password = get_server_details("server1")
            
            self.assertEqual(ip, '1.2.3.4')
            self.assertEqual(user, 'user')
            self.assertEqual(password, 'pass')


    def test_list_servers(self):
        with unittest.mock.patch('subprocess.run') as mock_run:
            mock_run.return_value = MagicMock(stdout=json.dumps({
                "items": [
                    {
                        "metadata": {"name": "server1"},
                        "spec": {
                            "managementNetwork": {"ips": ["1.1.1.1"]},
                            "bmc": {"ip": "2.2.2.2"}
                        }
                    }
                ]
            }))
            
            from redfish import list_servers
            # Capture print output
            from io import StringIO
            captured_output = StringIO()
            sys.stdout = captured_output
            try:
                list_servers()
            finally:
                sys.stdout = sys.__stdout__
            
            output = captured_output.getvalue()
            self.assertIn("server1", output)
            self.assertIn("1.1.1.1", output)
            self.assertIn("2.2.2.2", output)

if __name__ == '__main__':
    unittest.main()
