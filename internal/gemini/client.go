package gemini

import (
	"context"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Client struct {
	model *genai.GenerativeModel
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
		model: c.GenerativeModel(modelName),
	}, nil
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := c.model.GenerateContent(
		ctx,
		genai.Text(prompt),
	)
	if err != nil {
		return "", err
	}

	text := resp.Candidates[0].Content.Parts[0].(genai.Text)
	return string(text), nil
}




