package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// events streams every topic the browser asks for over one SSE connection.
// Subscribe with ?topic=board:<id>&topic=run:<id>, or ?topic=* for everything.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	topics := r.URL.Query()["topic"]
	if len(topics) == 0 {
		topics = []string{"*"}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	merged := make(chan busMessage, 128)
	for _, topic := range topics {
		ch, cancel := s.Bus.Subscribe(topic)
		defer cancel()
		go func() {
			for m := range ch {
				select {
				case merged <- busMessage{Topic: m.Topic, Kind: m.Kind, Data: m.Data}:
				default: // slow browser: drop rather than stall the publisher
				}
			}
		}()
	}

	// Proxies drop an idle SSE connection; a comment every 25s keeps it open.
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case m := <-merged:
			payload, err := json.Marshal(m)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", m.Kind, payload)
			flusher.Flush()
		}
	}
}

type busMessage struct {
	Topic string `json:"topic"`
	Kind  string `json:"kind"`
	Data  any    `json:"data"`
}
