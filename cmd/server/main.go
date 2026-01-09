package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/artaoheed/agentgate/internal/gemini"
	"github.com/artaoheed/agentgate/internal/policy"
)

type ChatRequest struct {
	Stream   bool `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type ChatResponse struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func main() {
	log.Println("=== AgentGate boot with Gemini + PII policy ===")

	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Fatal("GEMINI_API_KEY is not set")
	}

	client, err := gemini.New("gemini-2.5-flash")
	if err != nil {
		log.Fatalf("failed to init gemini client: %v", err)
	}

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
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
		// STREAMING PATH (SSE)
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

			for {
				select {
				case chunk, ok := <-chunks:
					if !ok {
						w.Write([]byte("data: [DONE]\n\n"))
						flusher.Flush()
						return
					}

					// Update rolling window
					window.Add(chunk.Text)

					// Evaluate PII
					if res := policy.EvaluatePII(window.Text()); res != nil {
						switch res.Decision {
						case policy.Abort:
							w.Write([]byte("data: [BLOCKED: PII DETECTED]\n\n"))
							flusher.Flush()
							return

						case policy.Redact:
							w.Write([]byte("data: [REDACTED]\n\n"))
							flusher.Flush()
							continue
						}
					}

					// Forward normal content
					w.Write([]byte("data: " + chunk.Text + "\n\n"))
					flusher.Flush()

				case err := <-errs:
					// Gemini end-of-stream signal
					if err.Error() == "no more items in iterator" {
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

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
