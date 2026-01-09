package gemini

import (
	"context"

	"github.com/google/generative-ai-go/genai"
)

type StreamChunk struct {
	Text string
}

func (c *Client) Stream(ctx context.Context, prompt string) (<-chan StreamChunk, <-chan error) {
	out := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)

		iter := c.model.GenerateContentStream(
			ctx,
			genai.Text(prompt),
		)

		for {
			resp, err := iter.Next()
			if err != nil {
				if err == context.Canceled {
					return
				}
				errCh <- err
				return
			}

			for _, cand := range resp.Candidates {
				for _, part := range cand.Content.Parts {
					if text, ok := part.(genai.Text); ok {
						out <- StreamChunk{Text: string(text)}
					}
				}
			}
		}
	}()

	return out, errCh
}
