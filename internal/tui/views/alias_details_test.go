package views

import (
	"testing"

	"github.com/kovmir/addyui/internal/api"
)

// fullTestAlias covers every optional field detailsRows renders.
func fullTestAlias() api.Alias {
	id := "50c9e585-e7f5-41c4-9016-9014c15454bc"
	email := "web@johndoe.anonaddy.com"
	description := "web shop"
	domain := "johndoe.anonaddy.com"
	localPart := "web"
	active := true
	pinned := true
	forwarded := 5
	blocked := 1
	replied := 2
	sent := 0
	created := "2019-10-01 09:00:00"
	updated := "2020-11-11 11:11:11"
	return api.Alias{
		Id:              &id,
		Email:           &email,
		Description:     &description,
		Domain:          &domain,
		LocalPart:       &localPart,
		Active:          &active,
		Pinned:          &pinned,
		EmailsForwarded: &forwarded,
		EmailsBlocked:   &blocked,
		EmailsReplied:   &replied,
		EmailsSent:      &sent,
		CreatedAt:       &created,
		UpdatedAt:       &updated,
		Recipients: &[]any{
			map[string]any{"id": "r1", "email": "a@example.com"},
			map[string]any{"id": "r2", "email": "b@example.com"},
		},
	}
}

func TestDetailsRowCountMatchesConstants(t *testing.T) {
	t.Parallel()

	// detailRowCount backs the table height calculation; the row
	// indices (detailsRowXxx) must stay in sync with detailsRows.
	rows := detailsRows(&api.Alias{})
	if len(rows) != detailsRowDeleted+1 {
		t.Errorf("detailsRows has %d rows, but detailsRowDeleted = %d", len(rows), detailsRowDeleted)
	}
	if got := detailRowCount(); got != len(rows) {
		t.Errorf("detailRowCount() = %d, want %d", got, len(rows))
	}
}

func TestDetailsRowsLabels(t *testing.T) {
	t.Parallel()

	rows := detailsRows(&api.Alias{})
	want := []string{
		"Email", "Description", "Domain", "Local part",
		"Active", "Pinned", "Recipients",
		"Forwarded", "Blocked", "Replied", "Sent",
		"Created", "Updated", "Deleted",
	}
	if len(rows) != len(want) {
		t.Fatalf("detailsRows has %d rows, want %d", len(rows), len(want))
	}
	for i, label := range want {
		if rows[i][0] != label {
			t.Errorf("row %d label = %q, want %q", i, rows[i][0], label)
		}
	}
}

func TestDetailsRowsValues(t *testing.T) {
	t.Parallel()

	a := fullTestAlias()
	rows := detailsRows(&a)

	if rows[detailsRowEmail][1] != "web@johndoe.anonaddy.com" {
		t.Errorf("Email = %q", rows[detailsRowEmail][1])
	}
	if rows[detailsRowDescription][1] != "web shop" {
		t.Errorf("Description = %q", rows[detailsRowDescription][1])
	}
	if rows[detailsRowDomain][1] != "johndoe.anonaddy.com" {
		t.Errorf("Domain = %q", rows[detailsRowDomain][1])
	}
	if rows[detailsRowLocalPart][1] != "web" {
		t.Errorf("Local part = %q", rows[detailsRowLocalPart][1])
	}
	if rows[detailsRowActive][1] != "true" {
		t.Errorf("Active = %q, want \"true\"", rows[detailsRowActive][1])
	}
	if rows[detailsRowPinned][1] != "true" {
		t.Errorf("Pinned = %q, want \"true\"", rows[detailsRowPinned][1])
	}
	if rows[detailsRowRecipients][1] != "a@example.com, b@example.com" {
		t.Errorf("Recipients = %q", rows[detailsRowRecipients][1])
	}
	if rows[detailsRowForwarded][1] != "5" {
		t.Errorf("Forwarded = %q", rows[detailsRowForwarded][1])
	}
	if rows[detailsRowBlocked][1] != "1" {
		t.Errorf("Blocked = %q", rows[detailsRowBlocked][1])
	}
	if rows[detailsRowReplied][1] != "2" {
		t.Errorf("Replied = %q", rows[detailsRowReplied][1])
	}
	if rows[detailsRowSent][1] != "0" {
		t.Errorf("Sent = %q, want \"0\"", rows[detailsRowSent][1])
	}
	if rows[detailsRowCreated][1] != "2019-10-01 09:00:00" {
		t.Errorf("Created = %q", rows[detailsRowCreated][1])
	}
	if rows[detailsRowUpdated][1] != "2020-11-11 11:11:11" {
		t.Errorf("Updated = %q", rows[detailsRowUpdated][1])
	}
	if rows[detailsRowDeleted][1] != "-" {
		t.Errorf("Deleted = %q, want \"-\" for a live alias", rows[detailsRowDeleted][1])
	}
}

func TestDetailsRowsEmptyAlias(t *testing.T) {
	t.Parallel()

	// A bare alias (e.g. from detailRowCount) must render blanks and
	// placeholders, not panic.
	rows := detailsRows(&api.Alias{})

	checks := []struct {
		row  int
		want string
	}{
		{detailsRowEmail, ""},
		{detailsRowDescription, "-"},
		{detailsRowDomain, ""},
		{detailsRowLocalPart, ""},
		{detailsRowActive, "false"},
		{detailsRowPinned, "false"},
		{detailsRowRecipients, "-"},
		{detailsRowForwarded, ""},
		{detailsRowBlocked, ""},
		{detailsRowReplied, ""},
		{detailsRowSent, ""},
		{detailsRowCreated, ""},
		{detailsRowUpdated, ""},
		{detailsRowDeleted, "-"},
	}
	for _, c := range checks {
		if rows[c.row][1] != c.want {
			t.Errorf("row %d value = %q, want %q", c.row, rows[c.row][1], c.want)
		}
	}
}

func TestDetailsRowsDeletedAlias(t *testing.T) {
	t.Parallel()

	a := fullTestAlias()
	deletedAt := "2021-01-01 00:00:00"
	a.DeletedAt = &deletedAt
	rows := detailsRows(&a)
	if rows[detailsRowDeleted][1] != "2021-01-01 00:00:00" {
		t.Errorf("Deleted = %q, want the timestamp", rows[detailsRowDeleted][1])
	}
}

func TestDetailsRowsInactiveAlias(t *testing.T) {
	t.Parallel()

	// new(bool) is false: an explicitly inactive alias.
	a := fullTestAlias()
	a.Active = new(bool)
	rows := detailsRows(&a)
	if rows[detailsRowActive][1] != "false" {
		t.Errorf("Active = %q, want \"false\"", rows[detailsRowActive][1])
	}
}

func TestDetailsRowsUnpinnedAlias(t *testing.T) {
	t.Parallel()

	// new(bool) is false: an explicitly unpinned alias.
	a := fullTestAlias()
	a.Pinned = new(bool)
	rows := detailsRows(&a)
	if rows[detailsRowPinned][1] != "false" {
		t.Errorf("Pinned = %q, want \"false\"", rows[detailsRowPinned][1])
	}
}

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
