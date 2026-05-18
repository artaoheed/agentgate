package gemini

import (
	"context"
	"os"
	"time"

	"github.com/artaoheed/agentgate/internal/obs"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Client struct {
	model     *genai.GenerativeModel
	modelName string
}

func New(modelName string) (*Client, error) {
	ctx := context.Background()

	c, err := genai.NewClient(
		ctx,
		option.WithAPIKey(os.Getenv("GEMINI_API_KEY")),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		model:     c.GenerativeModel(modelName),
		modelName: modelName,
	}, nil
}

// ModelName returns the configured model identifier (e.g. "gemini-2.5-flash").
func (c *Client) ModelName() string {
	return c.modelName
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	start := time.Now()
	resp, err := c.model.GenerateContent(
		ctx,
		genai.Text(prompt),
	)
	obs.GeminiDuration.WithLabelValues(c.modelName, "generate").Observe(time.Since(start).Seconds())
	if err != nil {
		obs.GeminiRequests.WithLabelValues(c.modelName, "generate", "error").Inc()
		return "", err
	}
	obs.GeminiRequests.WithLabelValues(c.modelName, "generate", "success").Inc()

	text := resp.Candidates[0].Content.Parts[0].(genai.Text)
	return string(text), nil
}
