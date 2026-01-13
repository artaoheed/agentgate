package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/artaoheed/agentgate/internal/events"
	"github.com/artaoheed/agentgate/internal/gemini"
	"github.com/artaoheed/agentgate/internal/policy"
	"github.com/google/uuid"
)

type ChatRequest struct {
	Stream   bool `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func main() {
	log.Println("=== AgentGate boot (Gemini + PII + Events) ===")

	// ---- ENV CHECKS ----
	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Fatal("GEMINI_API_KEY is not set")
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "agent-gate"
	}

	// ---- CLIENTS ----
	client, err := gemini.New("gemini-2.5-flash")
	if err != nil {
		log.Fatalf("failed to init gemini client: %v", err)
	}

	ctx := context.Background()

	logEmitter := events.NewLogEmitter()

	pubsubEmitter, err := events.NewPubSubEmitter(
		ctx,
		projectID,
		"agentgate-governance-events",
	)
	if err != nil {
		log.Printf("pubsub disabled: %v", err)
	}

	var emitter events.Emitter
	if pubsubEmitter != nil {
		emitter = events.NewMultiEmitter(logEmitter, pubsubEmitter)
	} else {
		emitter = logEmitter
	}

	// ---- HTTP HANDLER ----
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(req.Messages) == 0 {
			http.Error(w, "no messages provided", http.StatusBadRequest)
			return
		}

		prompt := req.Messages[len(req.Messages)-1].Content

		// =========================
		// STREAMING PATH
		// =========================
		if req.Stream {
			log.Println("DEBUG: entered streaming path")

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}

			chunks, errs := client.Stream(r.Context(), prompt)
			window := policy.NewRollingWindow(300)
			charsSinceEval := 0

			for {
				select {
				case chunk, ok := <-chunks:
					if !ok {
						// Normal successful completion
						emitter.Emit(events.GovernanceEvent{
							Timestamp: time.Now().UTC().Format(time.RFC3339),
							RequestID: requestID,
							Model:     "gemini-2.5-flash",
							Policy:    "none",
							Decision:  "allow",
							Streaming: true,
							LatencyMs: time.Since(start).Milliseconds(),
						})

						w.Write([]byte("data: [DONE]\n\n"))
						flusher.Flush()
						return
					}

					window.Add(chunk.Text)
					charsSinceEval += len(chunk.Text)

					// Throttled policy evaluation
					if charsSinceEval >= 50 {
						charsSinceEval = 0

						res := policy.EvaluatePII(window.Text())
						if res != nil {
							switch res.Decision {

							case policy.Abort:
								emitter.Emit(events.GovernanceEvent{
									Timestamp: time.Now().UTC().Format(time.RFC3339),
									RequestID: requestID,
									Model:     "gemini-2.5-flash",
									Policy:    "pii",
									Decision:  "abort",
									Reason:    res.Reason,
									Streaming: true,
									LatencyMs: time.Since(start).Milliseconds(),
								})

								w.Write([]byte("data: [BLOCKED: PII DETECTED]\n\n"))
								flusher.Flush()
								return

							case policy.Redact:
								emitter.Emit(events.GovernanceEvent{
									Timestamp: time.Now().UTC().Format(time.RFC3339),
									RequestID: requestID,
									Model:     "gemini-2.5-flash",
									Policy:    "pii",
									Decision:  "redact",
									Reason:    res.Reason,
									Streaming: true,
									LatencyMs: time.Since(start).Milliseconds(),
								})

								w.Write([]byte("data: [REDACTED]\n\n"))
								flusher.Flush()
								continue
							}
						}
					}

					w.Write([]byte("data: " + chunk.Text + "\n\n"))
					flusher.Flush()

				case err := <-errs:
					if err.Error() == "no more items in iterator" {
						log.Println("DEBUG: stream ended via iterator, emitting ALLOW")

						emitter.Emit(events.GovernanceEvent{
							Timestamp: time.Now().UTC().Format(time.RFC3339),
							RequestID: requestID,
							Model:     "gemini-2.5-flash",
							Policy:    "none",
							Decision:  "allow",
							Streaming: true,
							LatencyMs: time.Since(start).Milliseconds(),
						})

						w.Write([]byte("data: [DONE]\n\n"))
						flusher.Flush()
						return
					}

					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		// =========================
		// NON-STREAMING PATH
		// =========================
		resp, err := client.Generate(r.Context(), prompt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		emitter.Emit(events.GovernanceEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			RequestID: requestID,
			Model:     "gemini-2.5-flash",
			Policy:    "none",
			Decision:  "allow",
			Streaming: false,
			LatencyMs: time.Since(start).Milliseconds(),
		})

		out := ChatResponse{
			ID:     "agentgate-1",
			Object: "chat.completion",
			Choices: []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: resp,
				}},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})

	http.HandleFunc("/debug/publish", func(w http.ResponseWriter, r *http.Request) {
		emitter.Emit(events.GovernanceEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			RequestID: "debug",
			Model:     "test",
			Policy:    "test",
			Decision:  "allow",
			Streaming: false,
			LatencyMs: 1,
		})
		w.Write([]byte("published"))
	})

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
