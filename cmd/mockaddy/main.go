// Command mockaddy serves a tiny in-memory fake of the addy.io API so the
// TUI can be demonstrated and screenshotted without touching a real account.
//
// Usage:
//
//	mockaddy -addr 127.0.0.1:18099
//
// Then run the TUI against it:
//
//	ADDY_BASE_URL=http://127.0.0.1:18099 ADDY_API_KEY=demo ./addyui
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAddr   = "127.0.0.1:18099"
	defaultDomain = "demo.anonaddy.com"
	userID        = "ca0a4e09-c266-4f6f-845c-958db5090f09"
)

// recipient is a verified recipient mail can be forwarded to.
type recipient struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// alias mirrors the subset of the addy.io Alias model the TUI reads.
type alias struct {
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
	Recipients        []recipient `json:"recipients"`
	Labels            []string    `json:"labels"`
	DomainID          *string     `json:"domain_id"`
}

// listResponse wraps a paginated alias list the way the real API does.
type listResponse struct {
	Data  []alias   `json:"data"`
	Links listLinks `json:"links"`
	Meta  listMeta  `json:"meta"`
}

type listLinks struct {
	First string `json:"first"`
	Last  string `json:"last"`
}

type listMeta struct {
	CurrentPage int           `json:"current_page"`
	From        int           `json:"from"`
	LastPage    int           `json:"last_page"`
	PerPage     int           `json:"per_page"`
	To          int           `json:"to"`
	Total       int           `json:"total"`
	Links       []interface{} `json:"links"`
}

// recipientsResponse wraps the verified recipients list.
type recipientsResponse struct {
	Data []recipient `json:"data"`
}

// singleResponse wraps a single alias or object in a "data" envelope.
type singleResponse struct {
	Data alias `json:"data"`
}

var (
	recipients = []recipient{
		{ID: "11111111-1111-4111-8111-111111111111", Email: "me@example.com"},
		{ID: "22222222-2222-4222-8222-222222222222", Email: "backup@example.org"},
	}

	aliases = []alias{
		{
			ID:    "50c9e585-e7f5-41c4-9016-9014c15454bc",
			Email: "github@" + defaultDomain, LocalPart: "github", Domain: defaultDomain,
			Description: new("GitHub"), Active: true, Pinned: true,
			CreatedAt: ts(-30), UpdatedAt: ts(-1), EmailsForwarded: 42, EmailsBlocked: 3,
			LastForwarded: new("2026-11-09 08:14:00"), Recipients: recipients[:1],
		},
		{
			ID:    "c549db7d-5fac-4b09-9443-9e47f644d29f",
			Email: "gitlab@" + defaultDomain, LocalPart: "gitlab", Domain: defaultDomain,
			Description: new("GitLab"), Active: true,
			CreatedAt: ts(-28), UpdatedAt: ts(-2), EmailsForwarded: 11, EmailsBlocked: 0,
			LastForwarded: new("2026-11-07 19:32:00"), Recipients: recipients[:1],
		},
		{
			ID:    "7a1e0b64-9d02-4c41-a2f3-2b0e9d44f7aa",
			Email: "shopping@" + defaultDomain, LocalPart: "shopping", Domain: defaultDomain,
			Description: new("Online shopping"), Active: true,
			CreatedAt: ts(-20), UpdatedAt: ts(-3), EmailsForwarded: 87, EmailsBlocked: 14,
			LastForwarded: new("2026-11-08 12:00:00"), LastBlocked: new("2026-11-06 03:47:00"),
			Recipients: recipients[:2],
		},
		{
			ID:    "0e3f5228-c22a-4b7d-bc27-68c3d01e7b8d",
			Email: "newsletter@" + defaultDomain, LocalPart: "newsletter", Domain: defaultDomain,
			Description: new("Newsletters"), Active: false,
			CreatedAt: ts(-15), UpdatedAt: ts(-4), EmailsForwarded: 5, EmailsBlocked: 96,
			LastBlocked: new("2026-11-05 07:21:00"),
		},
		{
			ID:    "3d84ccbb-9032-49f4-8d1d-73e6e39f9c10",
			Email: "banking@" + defaultDomain, LocalPart: "banking", Domain: defaultDomain,
			Description: new("Bank statements"), Active: true, Pinned: true,
			CreatedAt: ts(-10), UpdatedAt: ts(-5), EmailsForwarded: 23, EmailsBlocked: 0,
			LastForwarded: new("2026-11-02 09:00:00"), Recipients: recipients[:1],
		},
		{
			ID:    "9f5b6f7e-83d1-4f08-9920-2ce77b6f4d3e",
			Email: "forums@" + defaultDomain, LocalPart: "forums", Domain: defaultDomain,
			Description: new("Forum accounts"), Active: true,
			CreatedAt: ts(-7), UpdatedAt: ts(-6), EmailsForwarded: 2, EmailsBlocked: 1,
			LastForwarded: new("2026-10-30 22:10:00"),
		},
		{
			ID:    "b2c4a9d0-3f7e-4d55-b0a1-8ea6f2c9e511",
			Email: "old@" + defaultDomain, LocalPart: "old", Domain: defaultDomain,
			Description: new("Deleted service"), Active: true,
			CreatedAt: ts(-90), UpdatedAt: ts(-7), DeletedAt: new("2026-10-12 00:00:00"),
			EmailsForwarded: 1,
		},
		{
			ID:    "e19d2b33-4a6c-4e89-9f3d-5f0b7d2a6c44",
			Email: "social@" + defaultDomain, LocalPart: "social", Domain: defaultDomain,
			Description: new("Social media"), Active: false,
			CreatedAt: ts(-5), UpdatedAt: ts(-8), EmailsBlocked: 7,
			LastBlocked: new("2026-11-04 18:05:00"), Recipients: recipients[:2],
		},
	}
)

// ts returns a formatted timestamp n days ago.
func ts(daysAgo int) string {
	return time.Now().AddDate(0, 0, daysAgo).Format("2006-01-02 15:04:05")
}

// filterAliases applies the query-string filters the TUI sends: search term,
// deleted-state and active-state. Pagination is ignored (the demo set fits on
// one page) but meta still reports sane values.
func filterAliases(r *http.Request) []alias {
	q := r.URL.Query()
	search := q.Get("filter[search]")
	deleted := q.Get("filter[deleted]") // "with", "only" or empty
	active := q.Get("filter[active]")   // "true", "false" or empty

	out := make([]alias, 0, len(aliases))
	for _, a := range aliases {
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func main() {
	addr := flag.String("addr", defaultAddr, "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	// Alias list: GET /api/v1/aliases
	mux.HandleFunc("GET /api/v1/aliases", func(w http.ResponseWriter, r *http.Request) {
		data := filterAliases(r)
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
				Links: []interface{}{},
			},
		})
	})

	// Alias details: GET /api/v1/aliases/{id}
	mux.HandleFunc("GET /api/v1/aliases/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		for _, a := range aliases {
			if a.ID == id {
				writeJSON(w, http.StatusOK, singleResponse{Data: a})
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
	})

	// Verified recipients: GET /api/v1/recipients
	mux.HandleFunc("GET /api/v1/recipients", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, recipientsResponse{Data: recipients})
	})

	// Fall-through for any other /api call the TUI makes: 200 with an empty
	// envelope, so browse-only demos never error out.
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("unhandled %s %s -> empty 200", r.Method, r.URL.Path)
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": nil})
	})

	fmt.Printf("mockaddy listening on http://%s (API key accepted: any)\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
