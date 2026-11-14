// SPDX-License-Identifier: MIT

package mockaddy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// getAliases issues GET /api/v1/aliases (with an optional query string)
// against the fake server and returns the decoded alias list.
func getAliases(t *testing.T, s *Server, query string) []Alias {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/aliases"+query, nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/aliases status = %d, want 200", rec.Code)
	}
	var resp listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode alias list: %v", err)
	}
	return resp.Data
}

// aliasIDs extracts the ids of the aliases in list order.
func aliasIDs(aliases []Alias) []string {
	ids := make([]string, len(aliases))
	for i, a := range aliases {
		ids[i] = a.ID
	}
	return ids
}

func assertIDs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v", got, want)
		}
	}
}

func TestListAliasesPinnedFirst(t *testing.T) {
	t.Parallel()

	// The pinned aliases must surface first while the relative order
	// of the rest is preserved.
	s := New(WithAliases([]Alias{
		{ID: "a", Email: "a@example.com", Active: true},
		{ID: "b", Email: "b@example.com", Active: true, Pinned: true},
		{ID: "c", Email: "c@example.com", Active: true},
		{ID: "d", Email: "d@example.com", Active: true, Pinned: true},
	}))

	assertIDs(t, aliasIDs(getAliases(t, s, "")), []string{"b", "d", "a", "c"})
}

func TestListAliasesFilterPinned(t *testing.T) {
	t.Parallel()

	s := New(WithAliases([]Alias{
		{ID: "a", Email: "a@example.com", Active: true},
		{ID: "b", Email: "b@example.com", Active: true, Pinned: true},
		{ID: "c", Email: "c@example.com", Active: true},
		{ID: "d", Email: "d@example.com", Active: true, Pinned: true},
	}))

	assertIDs(t, aliasIDs(getAliases(t, s, "?filter[pinned]=true")),
		[]string{"b", "d"})
	assertIDs(t, aliasIDs(getAliases(t, s, "?filter[pinned]=false")),
		[]string{"a", "c"})
	// Without a filter every alias is returned, pinned ones first.
	assertIDs(t, aliasIDs(getAliases(t, s, "")), []string{"b", "d", "a", "c"})
}

func TestPinAlias(t *testing.T) {
	t.Parallel()

	id := "a"
	s := New(WithAliases([]Alias{
		{ID: id, Email: "a@example.com", Active: true},
		{ID: "b", Email: "b@example.com", Active: true, Pinned: true},
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pinned-aliases",
		strings.NewReader(`{"id": "`+id+`"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/pinned-aliases status = %d, want 200", rec.Code)
	}
	var resp singleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode pin response: %v", err)
	}
	if !resp.Data.Pinned {
		t.Errorf("pinned alias in response has pinned = false")
	}

	// The newly pinned alias must now surface first, ahead of the
	// already-pinned one.
	assertIDs(t, aliasIDs(getAliases(t, s, "")), []string{"a", "b"})
}

func TestUnpinAlias(t *testing.T) {
	t.Parallel()

	id := "b"
	s := New(WithAliases([]Alias{
		{ID: "a", Email: "a@example.com", Active: true},
		{ID: id, Email: "b@example.com", Active: true, Pinned: true},
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pinned-aliases/"+id, nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/v1/pinned-aliases status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 response carries a body: %q", rec.Body.String())
	}

	// The unpinned alias must now sort after the unpinned ones.
	assertIDs(t, aliasIDs(getAliases(t, s, "")), []string{"a", "b"})
}

func TestPinAliasUnknownID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pinned-aliases",
		strings.NewReader(`{"id": "does-not-exist"}`))
	rec := httptest.NewRecorder()
	s := New()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("pin of a missing alias status = %d, want 404", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/pinned-aliases/does-not-exist", nil)
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unpin of a missing alias status = %d, want 404", rec.Code)
	}
}

func TestPinAliasMissingID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pinned-aliases",
		strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s := New()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("pin without id status = %d, want 422", rec.Code)
	}
}
