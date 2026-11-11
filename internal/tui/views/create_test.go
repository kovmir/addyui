package views

import (
	"testing"
)

// newTestCreateModel returns a create screen with domain options
// loaded, detached from any API client: the functions under test are
// pure state transitions.
func newTestCreateModel(domains []string) *CreateModel {
	m := NewCreateModel(nil)
	m.loading = false
	m.domains = domains
	return &m
}

func TestCurrentDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		domains   []string
		domainIdx int
		want      string
	}{
		{"nothing loaded", nil, 0, ""},
		{"nothing loaded, zero index", []string{}, 0, ""},
		{"first domain", []string{"a.com", "b.com"}, 0, "a.com"},
		{"second domain", []string{"a.com", "b.com"}, 1, "b.com"},
		{"index past the end", []string{"a.com"}, 1, ""},
		{"negative index", []string{"a.com"}, -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newTestCreateModel(tt.domains)
			m.domainIdx = tt.domainIdx
			if got := m.currentDomain(); got != tt.want {
				t.Errorf("currentDomain() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCycleDomain(t *testing.T) {
	t.Parallel()

	m := newTestCreateModel([]string{"a.com", "b.com", "c.com"})

	// Forward from the first: 0 -> 1.
	m.cycle(createRowDomain, +1)
	if got := m.currentDomain(); got != "b.com" {
		t.Errorf("after +1 from first: domain = %q, want \"b.com\"", got)
	}

	// Forward from the last wraps to the first.
	m.cycle(createRowDomain, +1)
	m.cycle(createRowDomain, +1)
	if got := m.currentDomain(); got != "a.com" {
		t.Errorf("after wrapping forward: domain = %q, want \"a.com\"", got)
	}

	// Backward from the first wraps to the last.
	m.cycle(createRowDomain, -1)
	if got := m.currentDomain(); got != "c.com" {
		t.Errorf("after wrapping backward: domain = %q, want \"c.com\"", got)
	}
}

func TestCycleDomainWithoutOptions(t *testing.T) {
	t.Parallel()

	// Cycling the domain row before the options arrive must be a no-op.
	m := newTestCreateModel(nil)
	m.cycle(createRowDomain, +1)
	if got := m.currentDomain(); got != "" {
		t.Errorf("cycling with no domains changed the index to %q", got)
	}
}

func TestCycleFormat(t *testing.T) {
	t.Parallel()

	m := newTestCreateModel([]string{"a.com"})

	// Forward from the first.
	m.cycle(createRowFormat, +1)
	if got := aliasFormats[m.formatIdx].value; got != "random_characters" {
		t.Errorf("after +1: format = %q, want \"random_characters\"", got)
	}

	// Backward from the first wraps to the last option.
	m2 := newTestCreateModel([]string{"a.com"})
	m2.cycle(createRowFormat, -1)
	if got := aliasFormats[m2.formatIdx].value; got != "custom" {
		t.Errorf("after -1 from first: format = %q, want \"custom\"", got)
	}

	// Forward from the last wraps to the first.
	m2.cycle(createRowFormat, +1)
	if m2.formatIdx != 0 {
		t.Errorf("after +1 from last: formatIdx = %d, want 0", m2.formatIdx)
	}
}

func TestCycleUnknownRow(t *testing.T) {
	t.Parallel()

	m := newTestCreateModel([]string{"a.com"})
	before := m.domainIdx
	m.cycle(99, +1)
	if m.domainIdx != before {
		t.Errorf("cycling an unknown row changed domainIdx from %d to %d", before, m.domainIdx)
	}
}

func TestCreateRecipientsSummary(t *testing.T) {
	t.Parallel()

	m := newTestCreateModel([]string{"a.com"})
	m.recipients = []recipient{
		{id: "r1", email: "a@example.com"},
		{id: "r2", email: "b@example.com"},
		{id: "r3", email: "c@example.com"},
	}

	if got := m.recipientsSummary(); got != "-" {
		t.Errorf("summary with nothing selected = %q, want \"-\"", got)
	}

	m.selected = map[string]bool{"r2": true}
	if got := m.recipientsSummary(); got != "b@example.com" {
		t.Errorf("summary with one selected = %q, want \"b@example.com\"", got)
	}

	m.selected = map[string]bool{"r1": true, "r3": true}
	if got := m.recipientsSummary(); got != "a@example.com, c@example.com" {
		t.Errorf("summary with two selected = %q, want \"a@example.com, c@example.com\"", got)
	}
}

func TestCreateRecipientsSummaryKeepsRecipientOrder(t *testing.T) {
	t.Parallel()

	// The summary follows the recipient list order, not the order the
	// recipient was picked in (map iteration order is random).
	m := newTestCreateModel(nil)
	m.recipients = []recipient{
		{id: "r1", email: "first@example.com"},
		{id: "r2", email: "second@example.com"},
	}
	m.selected = map[string]bool{"r2": true, "r1": true}
	want := "first@example.com, second@example.com"
	if got := m.recipientsSummary(); got != want {
		t.Errorf("recipientsSummary() = %q, want %q (stable list order)", got, want)
	}
}
