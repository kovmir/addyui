package views

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"

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

// intVal dereferences an optional integer field, defaulting to zero.
func intVal(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// describeError returns a short, human-readable messages matching API errors.
func describeError(err error) string {
	urlErr, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err.Error()
	}
	host := hostOf(urlErr.URL)

	switch {
	case errors.Is(urlErr.Err, context.DeadlineExceeded):
		return fmt.Sprintf("request timed out — is %s reachable?", host)
	case errors.Is(urlErr.Err, syscall.ECONNREFUSED):
		return fmt.Sprintf("connection refused by %s — is the instance URL correct?", host)
	}
	if dnsErr, ok := errors.AsType[*net.DNSError](urlErr.Err); ok {
		return fmt.Sprintf("cannot resolve %s — check the instance URL and your network", dnsErr.Name)
	}
	return err.Error()
}

// hostOf extracts the hostname from a request URL, falling back to a
// generic name when the URL cannot be parsed.
func hostOf(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return "the API"
}
