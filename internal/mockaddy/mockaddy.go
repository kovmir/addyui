// Package mockaddy serves a tiny in-memory fake of the addy.io API so
// the TUI can be demonstrated, screenshotted and tested without
// touching a real account.
//
// New() returns a server pre-loaded with the demo dataset used for the
// README GIF; the With* options build tailored instances. The demo
// binary runs it standalone, while tests run it through
// net/http/httptest:
//
//	srv := httptest.NewServer(mockaddy.New(mockaddy.WithAliases(seed)))
//	defer srv.Close()
package mockaddy

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultAddr is the listen address of the standalone demo binary.
const DefaultAddr = "127.0.0.1:18099"

// DefaultDomain is the domain of the seeded demo aliases.
const DefaultDomain = "demo.anonaddy.com"

// DefaultAliasFormat is the default alias format of the fake account.
const DefaultAliasFormat = "random_words"

const defaultUserID = "ca0a4e09-c266-4f6f-845c-958db5090f09"

// Recipient is a verified recipient mail can be forwarded to.
type Recipient struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Alias mirrors the subset of the addy.io Alias model the TUI reads.
type Alias struct {
	ID                string      `json:"id"`
	UserID            string      `json:"user_id"`
	Email             string      `json:"email"`
	LocalPart         string      `json:"local_part"`
	Domain            string      `json:"domain"`
	Description       *string     `json:"description"`
	Active            bool        `json:"active"`
	Pinned            bool        `json:"pinned"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
	DeletedAt         *string     `json:"deleted_at"`
	EmailsForwarded   int         `json:"emails_forwarded"`
	EmailsBlocked     int         `json:"emails_blocked"`
	EmailsSent        int         `json:"emails_sent"`
	EmailsReplied     int         `json:"emails_replied"`
	LastForwarded     *string     `json:"last_forwarded"`
	LastBlocked       *string     `json:"last_blocked"`
	LastSent          *string     `json:"last_sent"`
	LastReplied       *string     `json:"last_replied"`
	Extension         *string     `json:"extension"`
	FromName          *string     `json:"from_name"`
	AliasableID       *string     `json:"aliasable_id"`
	AliasableType     *string     `json:"aliasable_type"`
	AttachedRecipOnly bool        `json:"attached_recipients_only"`
	Recipients        []Recipient `json:"recipients"`
	Labels            []string    `json:"labels"`
	DomainID          *string     `json:"domain_id"`
}

// listResponse wraps a paginated alias list the way the real API does.
type listResponse struct {
	Data  []Alias   `json:"data"`
	Links listLinks `json:"links"`
	Meta  listMeta  `json:"meta"`
}

type listLinks struct {
	First string `json:"first"`
	Last  string `json:"last"`
}

type listMeta struct {
	CurrentPage int   `json:"current_page"`
	From        int   `json:"from"`
	LastPage    int   `json:"last_page"`
	PerPage     int   `json:"per_page"`
	To          int   `json:"to"`
	Total       int   `json:"total"`
	Links       []any `json:"links"`
}

// recipientsResponse wraps the verified recipients list.
type recipientsResponse struct {
	Data []Recipient `json:"data"`
}

// singleResponse wraps a single alias or object in a "data" envelope.
type singleResponse struct {
	Data Alias `json:"data"`
}

// domainOptionsResponse is the payload of GET /api/v1/domain-options.
type domainOptionsResponse struct {
	Data               []string `json:"data"`
	SharedDomains      []string `json:"sharedDomains"`
	DefaultAliasDomain string   `json:"defaultAliasDomain"`
	DefaultAliasFormat string   `json:"defaultAliasFormat"`
}

// Server is an in-memory fake of the addy.io API. Each instance owns
// its state, so tests can run several servers in parallel.
type Server struct {
	mux *http.ServeMux

	mu             sync.Mutex
	aliases        []Alias
	recipients     []Recipient
	apiKey         string // empty: accept any token (demo mode)
	statusOverride int    // when non-zero, every response gets it
}

// Option configures a Server.
type Option func(*Server)

// WithAliases replaces the seeded alias list with the given data.
func WithAliases(seed []Alias) Option {
	return func(s *Server) { s.aliases = append([]Alias(nil), seed...) }
}

// WithRecipients replaces the seeded recipient list with the given data.
func WithRecipients(seed []Recipient) Option {
	return func(s *Server) { s.recipients = append([]Recipient(nil), seed...) }
}

// WithAPIKey makes the server require Authorization: Bearer <key> on
// every request and answer 401 otherwise. By default any token is
// accepted.
func WithAPIKey(key string) Option {
	return func(s *Server) { s.apiKey = key }
}

// WithStatusOverride makes every endpoint answer with the given HTTP
// status instead of its normal response, so error paths can be tested.
func WithStatusOverride(code int) Option {
	return func(s *Server) { s.statusOverride = code }
}

// New returns a fake addy.io API server with the demo dataset loaded.
func New(opts ...Option) *Server {
	s := &Server{
		aliases:    demoAliases(),
		recipients: demoRecipients(),
	}
	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()

	// Alias list: GET /api/v1/aliases
	mux.HandleFunc("GET /api/v1/aliases", s.listAliases)

	// Alias details: GET /api/v1/aliases/{id}
	mux.HandleFunc("GET /api/v1/aliases/{id}", s.getAlias)

	// Activate: POST /api/v1/active-aliases (the id travels in the body)
	mux.HandleFunc("POST /api/v1/active-aliases", s.activateAlias)

	// Deactivate: DELETE /api/v1/active-aliases/{id}
	mux.HandleFunc("DELETE /api/v1/active-aliases/{id}", s.deactivateAlias)

	// Verified recipients: GET /api/v1/recipients
	mux.HandleFunc("GET /api/v1/recipients", s.listRecipients)

	// Domain options: GET /api/v1/domain-options
	mux.HandleFunc("GET /api/v1/domain-options", s.domainOptions)

	// Fall-through for any other /api call the TUI makes: 200 with an
	// empty envelope, so browse-only demos never error out.
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("unhandled %s %s -> empty 200", r.Method, r.URL.Path)
		writeJSON(w, http.StatusOK, map[string]any{"data": nil})
	})

	s.mux = mux
	return s
}

// ServeHTTP answers with the configured status override (if any),
// checks the bearer token and routes the request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.statusOverride != 0 {
		writeJSON(w, s.statusOverride, map[string]string{"message": "forced status override"})
		return
	}
	if s.apiKey != "" && r.Header.Get("Authorization") != "Bearer "+s.apiKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthenticated."})
		return
	}
	s.mux.ServeHTTP(w, r)
}

// ts returns a formatted timestamp n days ago.
func ts(daysAgo int) string {
	return time.Now().AddDate(0, 0, daysAgo).Format("2006-01-02 15:04:05")
}

// demoAliases is the dataset behind the README GIF.
func demoAliases() []Alias {
	recipients := demoRecipients()
	return []Alias{
		{
			ID:    "50c9e585-e7f5-41c4-9016-9014c15454bc",
			Email: "github@" + DefaultDomain, LocalPart: "github", Domain: DefaultDomain,
			Description: new("GitHub"), Active: true, Pinned: true,
			CreatedAt: ts(-30), UpdatedAt: ts(-1), EmailsForwarded: 42, EmailsBlocked: 3,
			LastForwarded: new("2026-11-09 08:14:00"), Recipients: recipients[:1],
		},
		{
			ID:    "c549db7d-5fac-4b09-9443-9e47f644d29f",
			Email: "gitlab@" + DefaultDomain, LocalPart: "gitlab", Domain: DefaultDomain,
			Description: new("GitLab"), Active: true,
			CreatedAt: ts(-28), UpdatedAt: ts(-2), EmailsForwarded: 11, EmailsBlocked: 0,
			LastForwarded: new("2026-11-07 19:32:00"), Recipients: recipients[:1],
		},
		{
			ID:    "7a1e0b64-9d02-4c41-a2f3-2b0e9d44f7aa",
			Email: "shopping@" + DefaultDomain, LocalPart: "shopping", Domain: DefaultDomain,
			Description: new("Online shopping"), Active: true,
			CreatedAt: ts(-20), UpdatedAt: ts(-3), EmailsForwarded: 87, EmailsBlocked: 14,
			LastForwarded: new("2026-11-08 12:00:00"), LastBlocked: new("2026-11-06 03:47:00"),
			Recipients: recipients[:2],
		},
		{
			ID:    "0e3f5228-c22a-4b7d-bc27-68c3d01e7b8d",
			Email: "newsletter@" + DefaultDomain, LocalPart: "newsletter", Domain: DefaultDomain,
			Description: new("Newsletters"), Active: false,
			CreatedAt: ts(-15), UpdatedAt: ts(-4), EmailsForwarded: 5, EmailsBlocked: 96,
			LastBlocked: new("2026-11-05 07:21:00"),
		},
		{
			ID:    "3d84ccbb-9032-49f4-8d1d-73e6e39f9c10",
			Email: "banking@" + DefaultDomain, LocalPart: "banking", Domain: DefaultDomain,
			Description: new("Bank statements"), Active: true, Pinned: true,
			CreatedAt: ts(-10), UpdatedAt: ts(-5), EmailsForwarded: 23, EmailsBlocked: 0,
			LastForwarded: new("2026-11-02 09:00:00"), Recipients: recipients[:1],
		},
		{
			ID:    "9f5b6f7e-83d1-4f08-9920-2ce77b6f4d3e",
			Email: "forums@" + DefaultDomain, LocalPart: "forums", Domain: DefaultDomain,
			Description: new("Forum accounts"), Active: true,
			CreatedAt: ts(-7), UpdatedAt: ts(-6), EmailsForwarded: 2, EmailsBlocked: 1,
			LastForwarded: new("2026-10-30 22:10:00"),
		},
		{
			ID:    "b2c4a9d0-3f7e-4d55-b0a1-8ea6f2c9e511",
			Email: "old@" + DefaultDomain, LocalPart: "old", Domain: DefaultDomain,
			Description: new("Deleted service"), Active: true,
			CreatedAt: ts(-90), UpdatedAt: ts(-7), DeletedAt: new("2026-10-12 00:00:00"),
			EmailsForwarded: 1,
		},
		{
			ID:    "e19d2b33-4a6c-4e89-9f3d-5f0b7d2a6c44",
			Email: "social@" + DefaultDomain, LocalPart: "social", Domain: DefaultDomain,
			Description: new("Social media"), Active: false,
			CreatedAt: ts(-5), UpdatedAt: ts(-8), EmailsBlocked: 7,
			LastBlocked: new("2026-11-04 18:05:00"), Recipients: recipients[:2],
		},
	}
}

func demoRecipients() []Recipient {
	return []Recipient{
		{ID: "11111111-1111-4111-8111-111111111111", Email: "me@example.com"},
		{ID: "22222222-2222-4222-8222-222222222222", Email: "backup@example.org"},
	}
}

// filterAliases applies the query-string filters the TUI sends: search
// term, deleted-state and active-state. Pagination is ignored (the demo
// set fits on one page) but meta still reports sane values.
func (s *Server) filterAliases(r *http.Request) []Alias {
	q := r.URL.Query()
	search := q.Get("filter[search]")
	deleted := q.Get("filter[deleted]") // "with", "only" or empty
	active := q.Get("filter[active]")   // "true", "false" or empty

	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Alias, 0, len(s.aliases))
	for _, a := range s.aliases {
		if search != "" &&
			!strings.Contains(a.Email, search) &&
			!strings.Contains(a.LocalPart, search) &&
			!strings.Contains(strVal(a.Description), search) {
			continue
		}
		if deleted == "only" && a.DeletedAt == nil {
			continue
		}
		if deleted == "" && a.DeletedAt != nil {
			continue
		}
		if active == "true" && !a.Active {
			continue
		}
		if active == "false" && a.Active {
			continue
		}
		out = append(out, a)
	}
	return out
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	if status == http.StatusNoContent {
		// A 204 must not carry a body. The generated client only
		// attempts to decode the body when Content-Type says JSON,
		// so the header is omitted as well.
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func (s *Server) listAliases(w http.ResponseWriter, r *http.Request) {
	data := s.filterAliases(r)
	total := len(data)
	writeJSON(w, http.StatusOK, listResponse{
		Data: data,
		Links: listLinks{
			First: "http://" + r.Host + r.URL.Path,
			Last:  "http://" + r.Host + r.URL.Path,
		},
		Meta: listMeta{
			CurrentPage: 1, From: 1, LastPage: 1,
			PerPage: total, To: total, Total: total,
			Links: []any{},
		},
	})
}

func (s *Server) getAlias(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.aliases {
		if a.ID == id {
			writeJSON(w, http.StatusOK, singleResponse{Data: a})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
}

// activateAlias activates the alias named in the JSON body.
func (s *Server) activateAlias(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"message": "missing alias id"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.aliases {
		if s.aliases[i].ID == body.ID {
			s.aliases[i].Active = true
			s.aliases[i].UpdatedAt = ts(0)
			writeJSON(w, http.StatusOK, singleResponse{Data: s.aliases[i]})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
}

// deactivateAlias deactivates the alias named in the path, answering
// with the empty 204 the API spec declares.
func (s *Server) deactivateAlias(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.aliases {
		if s.aliases[i].ID == id {
			s.aliases[i].Active = false
			s.aliases[i].UpdatedAt = ts(0)
			writeJSON(w, http.StatusNoContent, nil)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
}

func (s *Server) listRecipients(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, recipientsResponse{Data: s.recipients})
}

func (s *Server) domainOptions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, domainOptionsResponse{
		Data:               []string{DefaultDomain, "demo." + DefaultDomain},
		SharedDomains:      []string{},
		DefaultAliasDomain: DefaultDomain,
		DefaultAliasFormat: DefaultAliasFormat,
	})
}
