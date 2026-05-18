// Package secrets fetches values from Google Secret Manager. Used by
// the server to resolve credentials when they are not provided directly
// as environment variables (e.g. local dev with a long-lived gcloud
// session, instead of pasting the key into the shell).
package secrets

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// Fetch returns the payload of the given secret version. resourceName
// must be a full resource path of the form
//
//	projects/{project}/secrets/{secret}/versions/{version}
//
// e.g. "projects/agent-gate/secrets/gemini-api-key/versions/latest".
//
// Authentication uses Application Default Credentials. The returned
// value is the raw secret bytes; callers convert to string if needed.
func Fetch(ctx context.Context, resourceName string) ([]byte, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager client: %w", err)
	}
	defer client.Close()

	res, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: resourceName,
	})
	if err != nil {
		return nil, fmt.Errorf("access secret %q: %w", resourceName, err)
	}
	return res.Payload.Data, nil
}
