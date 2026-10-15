// Command ingest is the HTTP entry point for device events.
// Phase 1 goal: accept POST /v1/events, validate, and return 202.
// Later phases: rate limit (Redis), publish to Kafka, update device status.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Event struct {
	DeviceID  string    `json:"device_id"`
	EventID   string    `json:"event_id"` // idempotency key; must be unique per event
	Type      string    `json:"type"`     // e.g. "motion"
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload,omitempty"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/events", func(w http.ResponseWriter, r *http.Request) {
		var e Event
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if e.DeviceID == "" || e.EventID == "" {
			http.Error(w, "device_id and event_id required", http.StatusBadRequest)
			return
		}
		// TODO(phase 2): publish to Kafka topic "device.events" keyed by DeviceID.
		w.WriteHeader(http.StatusAccepted)
	})

	log.Println("ingest listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
