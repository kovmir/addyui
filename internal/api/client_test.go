package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kovmir/addyui/internal/mockaddy"
)

// captureServer records the auth-relevant headers and path of every
// request, answering with the shape the client expects.
func captureServer(t *testing.T, path, auth, xrw *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*path = r.URL.Path
		*auth = r.Header.Get("Authorization")
		*xrw = r.Header.Get("X-Requested-With")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewSendsAuthHeaders(t *testing.T) {
	t.Parallel()

	var path, auth, xrw string
	srv := captureServer(t, &path, &auth, &xrw)

	client, err := New(srv.URL, "secret-token")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.GetAllRecipientsWithResponse(ctx, &GetAllRecipientsParams{})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatalf("expected 200, got %s", resp.Status())
	}
	if auth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want \"Bearer secret-token\"", auth)
	}
	if xrw != "XMLHttpRequest" {
		t.Errorf("X-Requested-With = %q, want \"XMLHttpRequest\"", xrw)
	}
}

func TestNewTrimsTrailingSlash(t *testing.T) {
	t.Parallel()

	var path, auth, xrw string
	srv := captureServer(t, &path, &auth, &xrw)

	// A user-supplied base URL with a trailing slash must not produce
	// //api/v1/... double-slash paths.
	client, err := New(srv.URL+"/", "secret-token")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.GetAllRecipientsWithResponse(ctx, &GetAllRecipientsParams{}); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if path != "/api/v1/recipients" {
		t.Errorf("request path = %q, want \"/api/v1/recipients\"", path)
	}
}

func TestClientDecodesResponses(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(mockaddy.New())
	t.Cleanup(srv.Close)

	client, err := New(srv.URL, "test-key")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Recipients decode into the generated model.
	recipients, err := client.GetAllRecipientsWithResponse(ctx, &GetAllRecipientsParams{})
	if err != nil {
		t.Fatalf("GetAllRecipients failed: %v", err)
	}
	if recipients.JSON200 == nil || recipients.JSON200.Data == nil {
		t.Fatalf("GetAllRecipients: expected 200 with data, got %s", recipients.Status())
	}
	if len(*recipients.JSON200.Data) != 2 {
		t.Errorf("recipients = %d entries, want 2", len(*recipients.JSON200.Data))
	}
	if got := strVal((*recipients.JSON200.Data)[0].Email); got != "me@example.com" {
		t.Errorf("first recipient email = %q, want \"me@example.com\"", got)
	}

	// A single alias decodes with its fields intact.
	const id = "50c9e585-e7f5-41c4-9016-9014c15454bc"
	alias, err := client.GetASpecificAliasWithResponse(ctx, id, &GetASpecificAliasParams{})
	if err != nil {
		t.Fatalf("GetASpecificAlias failed: %v", err)
	}
	if alias.JSON200 == nil || alias.JSON200.Data == nil {
		t.Fatalf("GetASpecificAlias: expected 200 with data, got %s", alias.Status())
	}
	data := alias.JSON200.Data
	if data.Email == nil || *data.Email != "github@demo.anonaddy.com" {
		t.Errorf("alias email = %v, want \"github@demo.anonaddy.com\"", data.Email)
	}
	if data.Pinned == nil || !*data.Pinned {
		t.Error("alias pinned = false, want true")
	}
}

func TestClientSurfacesErrorStatuses(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(mockaddy.New(mockaddy.WithStatusOverride(500)))
	t.Cleanup(srv.Close)

	client, err := New(srv.URL, "test-key")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetAllRecipientsWithResponse(ctx, &GetAllRecipientsParams{})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.JSON200 != nil {
		t.Error("JSON200 must be nil for a 500 response")
	}
	if resp.StatusCode() != 500 {
		t.Errorf("StatusCode() = %d, want 500", resp.StatusCode())
	}
}

func TestClientRejectedOnBadToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(mockaddy.New(mockaddy.WithAPIKey("the-real-key")))
	t.Cleanup(srv.Close)

	client, err := New(srv.URL, "wrong-key")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetAllRecipientsWithResponse(ctx, &GetAllRecipientsParams{})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode() != 401 {
		t.Errorf("StatusCode() = %d, want 401", resp.StatusCode())
	}
	if resp.JSON200 != nil {
		t.Error("JSON200 must be nil for a 401 response")
	}
}

// strVal dereferences an optional string; the package does not export
// a helper for tests.
func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
