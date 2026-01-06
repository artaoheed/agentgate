package gemini

import (
	"context"
	"fmt"

	aiplatform "cloud.google.com/go/aiplatform/apiv1"
	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
)

type Client struct {
	project string
	region  string
	model   string
}

func New(project, region, model string) *Client {
	return &Client{
		project: project,
		region:  region,
		model:   model,
	}
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	client, err := aiplatform.NewPredictionClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	endpoint := fmt.Sprintf(
		"projects/%s/locations/%s/publishers/google/models/%s",
		c.project,
		c.region,
		c.model,
	)

	req := &aiplatformpb.GenerateContentRequest{
		Model: endpoint,
		Contents: []*aiplatformpb.Content{
			{
				Role: "user",
				Parts: []*aiplatformpb.Part{
					{Data: &aiplatformpb.Part_Text{Text: prompt}},
				},
			},
		},
	}

	resp, err := client.GenerateContent(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Candidates[0].Content.Parts[0].GetText(), nil
}
