package views

import (
	"testing"

	"github.com/kovmir/addyui/internal/api"
)

func TestAliasRecipients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		recipients *[]any
		want       []recipient
	}{
		{
			name:       "nil recipients",
			recipients: nil,
			want:       nil,
		},
		{
			name: "well formed entries",
			recipients: &[]any{
				map[string]any{"id": "r1", "email": "a@example.com"},
				map[string]any{"id": "r2", "email": "b@example.com"},
			},
			want: []recipient{
				{id: "r1", email: "a@example.com"},
				{id: "r2", email: "b@example.com"},
			},
		},
		{
			name: "non-map entries are skipped",
			recipients: &[]any{
				"not a map",
				42,
				map[string]any{"id": "r1", "email": "a@example.com"},
			},
			want: []recipient{{id: "r1", email: "a@example.com"}},
		},
		{
			name: "entries without id and email are skipped",
			recipients: &[]any{
				map[string]any{"id": "r1", "email": "a@example.com"},
				map[string]any{"other": "field"},
				map[string]any{"id": 42, "email": true},
			},
			want: []recipient{{id: "r1", email: "a@example.com"}},
		},
		{
			name: "id-only entry is kept",
			recipients: &[]any{
				map[string]any{"id": "r1"},
			},
			want: []recipient{{id: "r1"}},
		},
		{
			name: "email-only entry is kept",
			recipients: &[]any{
				map[string]any{"email": "a@example.com"},
			},
			want: []recipient{{email: "a@example.com"}},
		},
		{
			name:       "empty list",
			recipients: &[]any{},
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &api.Alias{Recipients: tt.recipients}
			got := aliasRecipients(a)
			if len(got) != len(tt.want) {
				t.Fatalf("aliasRecipients() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("aliasRecipients()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRecipientSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		emails []string
		want   string
	}{
		{"no recipients", nil, "-"},
		{"empty slice", []string{}, "-"},
		{"single", []string{"a@example.com"}, "a@example.com"},
		{
			"several short ones fit",
			[]string{"a@example.com", "b@example.com"},
			"a@example.com, b@example.com",
		},
		{
			"long list is truncated with ellipsis",
			// Each email adds len+2 to the width; the third 16-char
			// email pushes the width to 54 > 40, so the output stops
			// there.
			[]string{"aaaa@xxxxxxxxxxx", "bbbb@xxxxxxxxxxx", "cccc@xxxxxxxxxxx", "dddd@xxxxxxxxxxx"},
			"aaaa@xxxxxxxxxxx, bbbb@xxxxxxxxxxx, cccc@xxxxxxxxxxx...",
		},
		{
			"truncation stops at the email that crosses the limit",
			// 20+2=22, 22+22=44 > 40: stops after the second email.
			[]string{"aaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbb", "ccc@example.com"},
			"aaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbb...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := recipientSummary(tt.emails); got != tt.want {
				t.Errorf("recipientSummary(%v) = %q, want %q", tt.emails, got, tt.want)
			}
		})
	}
}
