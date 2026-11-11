package views

import (
	"net/http/httptest"
	"testing"

	"github.com/kovmir/addyui/internal/api"
	"github.com/kovmir/addyui/internal/mockaddy"
)

// startMockServer runs a fake addy.io API for the duration of the test.
func startMockServer(t *testing.T, opts ...mockaddy.Option) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mockaddy.New(opts...))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, opts ...mockaddy.Option) *api.ClientWithResponses {
	t.Helper()
	srv := startMockServer(t, opts...)
	client, err := api.New(srv.URL, "test-key")
	if err != nil {
		t.Fatalf("api.New() error = %v", err)
	}
	return client
}

func TestFetchVerifiedRecipients(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, mockaddy.WithRecipients([]mockaddy.Recipient{
		{ID: "r1", Email: "a@example.com"},
		{ID: "r2", Email: "b@example.com"},
	}))

	msg := fetchVerifiedRecipients(client)() // run the tea.Cmd
	loaded, ok := msg.(recipientsLoadedMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientsLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("recipientsLoadedMsg error = %v", loaded.err)
	}
	if len(loaded.recipients) != 2 {
		t.Fatalf("recipients = %d entries, want 2", len(loaded.recipients))
	}
	if loaded.recipients[0].id != "r1" || loaded.recipients[0].email != "a@example.com" {
		t.Errorf("recipients[0] = %+v, want {r1 a@example.com}", loaded.recipients[0])
	}
}

func TestFetchDomains(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	msg := NewCreateModel(client).fetchDomains()() // run the tea.Cmd
	opts, ok := msg.(createOptionsMsg)
	if !ok {
		t.Fatalf("command produced %T, want createOptionsMsg", msg)
	}
	if opts.err != nil {
		t.Fatalf("createOptionsMsg error = %v", opts.err)
	}
	if len(opts.domains) == 0 {
		t.Fatal("createOptionsMsg has no domains")
	}
	if opts.defaultDomain != mockaddy.DefaultDomain {
		t.Errorf("defaultDomain = %q, want %q", opts.defaultDomain, mockaddy.DefaultDomain)
	}
	if opts.defaultFormat != mockaddy.DefaultAliasFormat {
		t.Errorf("defaultFormat = %q, want %q", opts.defaultFormat, mockaddy.DefaultAliasFormat)
	}
}

func TestToggleActiveDeactivates(t *testing.T) {
	t.Parallel()

	id := "50c9e585-e7f5-41c4-9016-9014c15454bc"
	client := newTestClient(t, mockaddy.WithAliases([]mockaddy.Alias{
		{ID: id, Email: "github@demo.anonaddy.com", Domain: mockaddy.DefaultDomain,
			Active: true, CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00"},
	}))

	m := NewDetailsModel(client)
	active := true
	m.aliases = []api.Alias{{Id: &id, Active: &active}}
	m.index = 0

	msg := m.toggleActive(&m.aliases[0])() // run the tea.Cmd
	done, ok := msg.(detailsActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want detailsActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("toggle active failed: %v", done.err)
	}
	if done.op != "toggled active state" {
		t.Errorf("op = %q, want \"toggled active state\"", done.op)
	}
	if done.alias == nil || done.alias.Active == nil || *done.alias.Active {
		t.Errorf("refreshed alias is still active: %+v", done.alias)
	}
}

func TestToggleActiveActivates(t *testing.T) {
	t.Parallel()

	id := "0e3f5228-c22a-4b7d-bc27-68c3d01e7b8d"
	client := newTestClient(t, mockaddy.WithAliases([]mockaddy.Alias{
		{ID: id, Email: "newsletter@demo.anonaddy.com", Domain: mockaddy.DefaultDomain,
			Active: false, CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00"},
	}))

	m := NewDetailsModel(client)
	m.aliases = []api.Alias{{Id: &id, Active: new(bool)}}
	m.index = 0

	msg := m.toggleActive(&m.aliases[0])() // run the tea.Cmd
	done, ok := msg.(detailsActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want detailsActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("toggle active failed: %v", done.err)
	}
	if done.alias == nil || done.alias.Active == nil || !*done.alias.Active {
		t.Errorf("refreshed alias is still inactive: %+v", done.alias)
	}
}

func TestToggleActiveErrorSurfaces(t *testing.T) {
	t.Parallel()

	// The alias does not exist on the fake server, so the toggle must
	// come back as an error message rather than a success.
	id := "does-not-exist"
	client := newTestClient(t)

	m := NewDetailsModel(client)
	m.aliases = []api.Alias{{Id: &id, Active: new(bool)}}
	m.index = 0

	msg := m.toggleActive(&m.aliases[0])() // run the tea.Cmd
	done, ok := msg.(detailsActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want detailsActionDoneMsg", msg)
	}
	if done.err == nil {
		t.Error("toggle of a missing alias must produce an error")
	}
	if done.alias != nil {
		t.Errorf("a failed toggle must not carry an alias: %+v", done.alias)
	}
}
