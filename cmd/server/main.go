package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/artaoheed/agentgate/internal/events"
	"github.com/artaoheed/agentgate/internal/gemini"
	"github.com/artaoheed/agentgate/internal/obs"
	"github.com/artaoheed/agentgate/internal/policy"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	log := obs.NewLogger()
	slog.SetDefault(log)
	log.Info("agentgate boot", "component", "gemini+pii+events")

	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Error("GEMINI_API_KEY is not set")
		os.Exit(1)
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "agent-gate"
	}

	client, err := gemini.New("gemini-2.5-flash")
	if err != nil {
		log.Error("gemini client init failed", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	logEmitter := events.NewLogEmitter(log)
	metricsEmitter := events.NewMetricsEmitter()
	pubsubEmitter, err := events.NewPubSubEmitter(
		ctx,
		projectID,
		"agentgate-governance-events",
		log,
	)
	if err != nil {
		log.Warn("pubsub disabled", "err", err)
	}

	var emitter events.Emitter
	if pubsubEmitter != nil {
		emitter = events.NewMultiEmitter(logEmitter, metricsEmitter, pubsubEmitter)
	} else {
		emitter = events.NewMultiEmitter(logEmitter, metricsEmitter)
	}

	// ---- ROUTES ----
	var ready atomic.Bool
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/v1/chat/completions", instrumented("/v1/chat/completions", chatHandler(log, client, emitter)))

	// ---- SERVER + GRACEFUL SHUTDOWN ----
	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Info("listening", "addr", srv.Addr)
		ready.Store(true)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutdown signal received")
	ready.Store(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "err", err)
	}
	if pubsubEmitter != nil {
		if err := pubsubEmitter.Close(); err != nil {
			log.Error("pubsub close error", "err", err)
		}
	}
	log.Info("server stopped")
}

// statusRecorder captures the response status for metrics and preserves
// Flusher semantics so the streaming handler still works.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func instrumented(path string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		h(rec, r)
		dur := time.Since(start).Seconds()
		obs.HTTPDuration.WithLabelValues(path).Observe(dur)
		obs.HTTPRequests.WithLabelValues(path, r.Method, strconv.Itoa(rec.status)).Inc()
	}
}

func chatHandler(baseLog *slog.Logger, client *gemini.Client, emitter events.Emitter) http.HandlerFunc {
	model := client.ModelName()

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		log := baseLog.With("request_id", requestID)

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Warn("bad request body", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(req.Messages) == 0 {
			http.Error(w, "no messages provided", http.StatusBadRequest)
			return
		}

		prompt := req.Messages[len(req.Messages)-1].Content

		if req.Stream {
			handleStream(log, w, r, client, emitter, requestID, model, prompt, start)
			return
		}
		handleUnary(log, w, r, client, emitter, requestID, model, prompt, start)
	}
}

func handleStream(log *slog.Logger, w http.ResponseWriter, r *http.Request, client *gemini.Client, emitter events.Emitter, requestID, model, prompt string, start time.Time) {
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

	for {
		select {
		case <-r.Context().Done():
			return

		case chunk, ok := <-chunks:
			if !ok {
				// Final guaranteed policy check on stream close.
				if res := policy.EvaluatePII(window.Text()); res != nil {
					emitter.Emit(events.GovernanceEvent{
						Timestamp: time.Now().UTC().Format(time.RFC3339),
						RequestID: requestID,
						Model:     model,
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
						Model:     model,
						Policy:    "none",
						Decision:  "allow",
						Streaming: true,
						LatencyMs: time.Since(start).Milliseconds(),
					})
				}

				w.Write([]byte("data: [DONE]\n\n"))
				flusher.Flush()
				return
			}

			window.Add(chunk.Text)

			// Evaluate every chunk before flushing it to the wire.
			// Throttling here would leak short payloads that finish
			// under the threshold (e.g. a 25-char phone reply).
			if res := policy.EvaluatePII(window.Text()); res != nil {
				if res.Decision == policy.Abort {
					emitter.Emit(events.GovernanceEvent{
						Timestamp: time.Now().UTC().Format(time.RFC3339),
						RequestID: requestID,
						Model:     model,
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
						Model:     model,
						Policy:    "pii",
						Decision:  "redact",
						Reason:    res.Reason,
						Streaming: true,
						LatencyMs: time.Since(start).Milliseconds(),
					})

					window.Mask(res.Matches)

					w.Write([]byte("data: [REDACTED]\n\n"))
					flusher.Flush()
					continue
				}
			}

			w.Write([]byte("data: " + chunk.Text + "\n\n"))
			flusher.Flush()

		case err := <-errs:
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Error("stream error", "err", err)
			w.Write([]byte("data: [ERROR]\n\n"))
			flusher.Flush()
			return
		}
	}
}

func handleUnary(log *slog.Logger, w http.ResponseWriter, r *http.Request, client *gemini.Client, emitter events.Emitter, requestID, model, prompt string, start time.Time) {
	resp, err := client.Generate(r.Context(), prompt)
	if err != nil {
		log.Error("gemini generate failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	piiRes := policy.EvaluatePII(resp)

	evt := events.GovernanceEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
		Model:     model,
		Policy:    "none",
		Decision:  "allow",
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
}
