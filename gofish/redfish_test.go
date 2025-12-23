package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGofishGetPowerState(t *testing.T) {
	mux := http.NewServeMux()

	// Redfish Service Root
	mux.HandleFunc("/redfish/v1/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Mock received request: %s\n", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"@odata.id": "/redfish/v1/",
			"@odata.type": "#ServiceRoot.v1_5_0.ServiceRoot",
			"Name": "Root Service",
			"RedfishVersion": "1.5.0",
			"Systems": {"@odata.id": "/redfish/v1/Systems"}
		}`)
	})

	// Systems Collection
	mux.HandleFunc("/redfish/v1/Systems", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"@odata.id": "/redfish/v1/Systems",
			"Members": [{"@odata.id": "/redfish/v1/Systems/1"}],
			"Members@odata.count": 1
		}`)
	})

	// System 1
	mux.HandleFunc("/redfish/v1/Systems/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"@odata.id": "/redfish/v1/Systems/1",
			"ID": "1",
			"PowerState": "On"
		}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewRedfishClient(server.URL, "user", "pass", false, true)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	state, err := client.GetPowerState("1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if state != "On" {
		t.Errorf("Expected power state On, got %s", state)
	}
}

func TestGofishResetSystem(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"@odata.id": "/redfish/v1/",
			"@odata.type": "#ServiceRoot.v1_5_0.ServiceRoot",
			"Systems": {"@odata.id": "/redfish/v1/Systems"}
		}`)
	})
	mux.HandleFunc("/redfish/v1/Systems", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"@odata.id": "/redfish/v1/Systems",
			"Members": [{"@odata.id": "/redfish/v1/Systems/1"}],
			"Members@odata.count": 1
		}`)
	})
	mux.HandleFunc("/redfish/v1/Systems/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"@odata.id": "/redfish/v1/Systems/1",
			"ID": "1",
			"Actions": {
				"#ComputerSystem.Reset": {
					"target": "/redfish/v1/Systems/1/Actions/ComputerSystem.Reset"
				}
			}
		}`)
	})
	mux.HandleFunc("/redfish/v1/Systems/1/Actions/ComputerSystem.Reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewRedfishClient(server.URL, "user", "pass", false, true)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	success, err := client.ResetSystem("On", "1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !success {
		t.Error("Expected success true")
	}
}
