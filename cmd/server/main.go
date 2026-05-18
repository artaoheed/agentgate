package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	// ---- HTTP HANDLERS ----
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
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
				case <-r.Context().Done():
					return

				case chunk, ok := <-chunks:
					if !ok {
						// 🔒 FINAL GUARANTEED POLICY CHECK
						if res := policy.EvaluatePII(window.Text()); res != nil {
							// reason := res.Reason
							emitter.Emit(events.GovernanceEvent{
								Timestamp: time.Now().UTC().Format(time.RFC3339),
								RequestID: requestID,
								Model:     "gemini-2.5-flash",
								Policy:    "pii",
								Decision:  string(res.Decision),
								Reason:    res.Reason,
								Streaming: true,
								LatencyMs: time.Since(start).Milliseconds(),
							})
						} else {
							emitter.Emit(events.GovernanceEvent{
								Timestamp: time.Now().UTC().Format(time.RFC3339),
								RequestID: requestID,
								Model:     "gemini-2.5-flash",
								Policy:    "none",
								Decision:  "allow",
								Reason:    "",
								Streaming: true,
								LatencyMs: time.Since(start).Milliseconds(),
							})
						}

						w.Write([]byte("data: [DONE]\n\n"))
						flusher.Flush()
						return
					}

					window.Add(chunk.Text)
					charsSinceEval += len(chunk.Text)

					// ⚡ THROTTLED MID-STREAM CHECK
					if charsSinceEval >= 50 {
						charsSinceEval = 0

						if res := policy.EvaluatePII(window.Text()); res != nil {
							// eason := res.Reason

							if res.Decision == policy.Abort {
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
							}

							if res.Decision == policy.Redact {
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

								// Mask matched span in the window so the same
								// hit doesn't re-fire on the next eval.
								window.Mask(res.Matches)

								w.Write([]byte("data: [REDACTED]\n\n"))
								flusher.Flush()
								continue
							}
						}
					}
					log.Printf("FINAL OUTPUT BUFFER: %q", window.Text())

					w.Write([]byte("data: " + chunk.Text + "\n\n"))
					flusher.Flush()

				case err := <-errs:
					// Expected when we intentionally abort/redact mid-stream
					// or the client disconnects.
					if errors.Is(err, context.Canceled) ||
						err.Error() == "stream terminated" ||
						err.Error() == "no more items in iterator" {
						return
					}
					log.Printf("stream error: %v", err)
					w.Write([]byte("data: [ERROR]\n\n"))
					flusher.Flush()
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

		piiRes := policy.EvaluatePII(resp)

		// 2. Prepare the event (defaults to allow)
		evt := events.GovernanceEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			RequestID: requestID,
			Model:     "gemini-2.5-flash",
			Policy:    "none",
			Decision:  "allow",
			Reason:    "",
			Streaming: false,
			LatencyMs: time.Since(start).Milliseconds(),
		}

		if piiRes != nil {
			evt.Policy = "pii"
			evt.Decision = string(piiRes.Decision)
			evt.Reason = piiRes.Reason

			if piiRes.Decision == policy.Abort {
				emitter.Emit(evt)
				http.Error(w, "Blocked: PII Detected", http.StatusForbidden)
				return
			}

			if piiRes.Decision == policy.Redact {
				resp = policy.RedactSpans(resp, piiRes.Matches)
			}
		}

		emitter.Emit(evt)

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

	// ---- SERVER + GRACEFUL SHUTDOWN ----
	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("Listening on :8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	if pubsubEmitter != nil {
		if err := pubsubEmitter.Close(); err != nil {
			log.Printf("pubsub close error: %v", err)
		}
	}
	log.Println("server stopped")
}
