package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/artaoheed/agentgate/api"
	"github.com/artaoheed/agentgate/internal/gemini"
)

func main() {
	client, err := gemini.New("gemini-2.5-flash")
	if err != nil {
		log.Fatalf("failed to init gemini client: %v", err)
	}

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req api.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		prompt := req.Messages[len(req.Messages)-1].Content
		resp, err := client.Generate(r.Context(), prompt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out := api.ChatCompletionResponse{
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

	log.Println("AgentGate running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}





