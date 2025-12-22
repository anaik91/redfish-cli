package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPowerState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/redfish/v1/Systems/1" {
			t.Errorf("Expected path /redfish/v1/Systems/1, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"PowerState": "On"})
	}))
	defer server.Close()

	client := NewRedfishClient(server.URL, "user", "pass")
	state, err := client.GetPowerState("1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if state != "On" {
		t.Errorf("Expected On, got %s", state)
	}
}

func TestResetSystem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["ResetType"] != "On" {
			t.Errorf("Expected ResetType On, got %s", payload["ResetType"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewRedfishClient(server.URL, "user", "pass")
	_, err := client.ResetSystem("On", "1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestResetManager(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/redfish/v1/Managers/1/Actions/Manager.Reset" {
			t.Errorf("Expected manager reset path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewRedfishClient(server.URL, "user", "pass")
	_, err := client.ResetManager("ForceRestart", "1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestGetPostState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Oem": map[string]interface{}{
				"Hpe": map[string]interface{}{
					"PostState": "FinishedPost",
				},
			},
		})
	}))
	defer server.Close()

	client := NewRedfishClient(server.URL, "user", "pass")
	state, err := client.GetPostState("1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if state != "FinishedPost" {
		t.Errorf("Expected FinishedPost, got %s", state)
	}
}
