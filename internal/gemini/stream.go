package gemini

import (
	"context"
	"errors"
	"time"

	"github.com/artaoheed/agentgate/internal/obs"
	"github.com/google/generative-ai-go/genai"
)

type StreamChunk struct {
	Text string
}

func (c *Client) Stream(ctx context.Context, prompt string) (<-chan StreamChunk, <-chan error) {
	out := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		start := time.Now()
		// Only `out` is closed — consumers select on `<-out` (with the ok
		// flag) to detect normal end-of-stream. `errCh` is only written
		// on a real error; closing it would race with the close of `out`
		// and cause a nil read in the consumer's select.
		defer close(out)

		iter := c.model.GenerateContentStream(
			ctx,
			genai.Text(prompt),
		)

		var streamErr error
		for {
			resp, err := iter.Next()
			if err != nil {
				// "no more items in iterator" is the SDK's EOF; not a
				// real error. context.Canceled means the client gave up.
				if !errors.Is(err, context.Canceled) && err.Error() != "no more items in iterator" {
					streamErr = err
				}
				break
			}

			for _, cand := range resp.Candidates {
				for _, part := range cand.Content.Parts {
					if text, ok := part.(genai.Text); ok {
						out <- StreamChunk{Text: string(text)}
					}
				}
			}
		}

		obs.GeminiDuration.WithLabelValues(c.modelName, "stream").Observe(time.Since(start).Seconds())
		outcome := "success"
		if streamErr != nil {
			outcome = "error"
			errCh <- streamErr
		}
		obs.GeminiRequests.WithLabelValues(c.modelName, "stream", outcome).Inc()
	}()

	return out, errCh
}
