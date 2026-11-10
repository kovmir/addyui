package views

import (
	"fmt"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

// tableStyles returns the app-wide table style: bubbles' defaults, but
// with the selected row rendered as reversed fg/bg instead of pink.
func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Selected = lipgloss.NewStyle().Bold(true).Reverse(true)
	return s
}

// str safely dereferences an optional string field.
func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// apiError merges a transport error with a non-2xx API response into a
// single error, so a 4xx/5xx is not silently reported as success. All
// oapi-codegen generated response types implement this interface.
func apiError(err error, resp interface {
	Status() string
	StatusCode() int
}) error {
	if err != nil {
		return err
	}
	if code := resp.StatusCode(); code < 200 || code >= 300 {
		return fmt.Errorf("unexpected response: %s", resp.Status())
	}
	return nil
}

// count renders an optional integer field as a display string.
func count(n *int) string {
	if n == nil {
		return ""
	}
	return fmt.Sprintf("%d", *n)
}
