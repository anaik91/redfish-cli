package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPowerState(t *testing.T) {
	tests := []struct {
		name       string
		powerState string
	}{
		{"Power On", "On"},
		{"Power Off", "Off"},
		{"Null State", "Null"},
		{"Unknown state", "Unknown"},
		{"Reset state", "Reset"},
		{"Powering On", "PoweringOn"},
		{"Powering Off", "PoweringOff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"PowerState": tt.powerState})
			}))
			defer server.Close()

			client := NewRedfishClient(server.URL, "user", "pass", false)
			state, err := client.GetPowerState("1")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if state != tt.powerState {
				t.Errorf("Expected %s, got %s", tt.powerState, state)
			}
		})
	}
}

func TestResetSystem(t *testing.T) {
	tests := []struct {
		name      string
		resetType string
	}{
		{"Reset On", "On"},
		{"Graceful Shutdown", "GracefulShutdown"},
		{"Force Off", "ForceOff"},
		{"Force Restart", "ForceRestart"},
		{"Aux Cycle", "AuxCycle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("Expected POST, got %s", r.Method)
				}
				var payload map[string]string
				json.NewDecoder(r.Body).Decode(&payload)
				if payload["ResetType"] != tt.resetType {
					t.Errorf("Expected ResetType %s, got %s", tt.resetType, payload["ResetType"])
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			client := NewRedfishClient(server.URL, "user", "pass", false)
			_, err := client.ResetSystem(tt.resetType, "1")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestResetManager(t *testing.T) {
	tests := []struct {
		name      string
		resetType string
	}{
		{"Force Restart", "ForceRestart"},
		{"Graceful Restart", "GracefulRestart"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/redfish/v1/Managers/1/Actions/Manager.Reset" {
					t.Errorf("Expected manager reset path, got %s", r.URL.Path)
				}
				var payload map[string]string
				json.NewDecoder(r.Body).Decode(&payload)
				if payload["ResetType"] != tt.resetType {
					t.Errorf("Expected ResetType %s, got %s", tt.resetType, payload["ResetType"])
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			client := NewRedfishClient(server.URL, "user", "pass", false)
			_, err := client.ResetManager(tt.resetType, "1")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGetPostState(t *testing.T) {
	tests := []struct {
		name      string
		postState string
	}{
		{"Null", "Null"},
		{"Unknown", "Unknown"},
		{"Reset", "Reset"},
		{"PowerOff", "PowerOff"},
		{"InPost", "InPost"},
		{"InPostDiscoveryStart", "InPostDiscoveryStart"},
		{"InPostDiscoveryComplete", "InPostDiscoveryComplete"},
		{"FinishedPost", "FinishedPost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"Oem": map[string]interface{}{
						"Hpe": map[string]interface{}{
							"PostState": tt.postState,
						},
					},
				})
			}))
			defer server.Close()

			client := NewRedfishClient(server.URL, "user", "pass", false)
			state, err := client.GetPostState("1")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if state != tt.postState {
				t.Errorf("Expected %s, got %s", tt.postState, state)
			}
		})
	}
}
