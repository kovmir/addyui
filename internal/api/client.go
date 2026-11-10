package api

import (
	"context"
	"net/http"
	"strings"
)

// New returns a pre-configured client for the addy.io REST API.
//
// It wires the bearer token and the X-Requested-With header that every
// addy.io endpoint expects, so callers never have to set them again.
func New(baseURL, apiKey string) (*ClientWithResponses, error) {
	return NewClientWithResponses(
		strings.TrimSuffix(baseURL, "/"),
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			return nil
		}),
	)
}
