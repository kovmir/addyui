package views

import (
	"errors"
	"testing"
)

func TestStr(t *testing.T) {
	t.Parallel()

	value := "johndoe@anonaddy.com"
	tests := []struct {
		name string
		in   *string
		want string
	}{
		{"nil", nil, ""},
		{"empty", new(string), ""},
		{"value", &value, "johndoe@anonaddy.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := str(tt.in); got != tt.want {
				t.Errorf("str(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	t.Parallel()

	zero := 0
	answer := 42
	tests := []struct {
		name string
		in   *int
		want string
	}{
		{"nil", nil, ""},
		{"zero", &zero, "0"},
		{"value", &answer, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := count(tt.in); got != tt.want {
				t.Errorf("count(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// fakeResponse is a minimal stand-in for the oapi-codegen generated
// response types, which satisfy the interface apiError expects.
type fakeResponse struct {
	status string
	code   int
}

func (r fakeResponse) Status() string  { return r.status }
func (r fakeResponse) StatusCode() int { return r.code }

func TestApiError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("connection refused")

	tests := []struct {
		name      string
		err       error
		resp      fakeResponse
		wantErr   bool
		wantMsg   string // empty: don't check the message
		sameError bool   // the transport error must be returned as-is
	}{
		{
			name:      "transport error passthrough",
			err:       transportErr,
			resp:      fakeResponse{"500 Internal Server Error", 500},
			wantErr:   true,
			sameError: true,
		},
		{
			name:    "200 OK",
			resp:    fakeResponse{"200 OK", 200},
			wantErr: false,
		},
		{
			name:    "201 Created",
			resp:    fakeResponse{"201 Created", 201},
			wantErr: false,
		},
		{
			name:    "299 still success",
			resp:    fakeResponse{"299 Unknown", 299},
			wantErr: false,
		},
		{
			name:    "404 Not Found",
			resp:    fakeResponse{"404 Not Found", 404},
			wantErr: true,
			wantMsg: "unexpected response: 404 Not Found",
		},
		{
			name:    "500 Internal Server Error",
			resp:    fakeResponse{"500 Internal Server Error", 500},
			wantErr: true,
			wantMsg: "unexpected response: 500 Internal Server Error",
		},
		{
			name:    "100 below range",
			resp:    fakeResponse{"100 Continue", 100},
			wantErr: true,
			wantMsg: "unexpected response: 100 Continue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := apiError(tt.err, tt.resp)
			if (err != nil) != tt.wantErr {
				t.Fatalf("apiError() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				return
			}
			if tt.sameError && err != transportErr {
				t.Errorf("apiError() = %v, want the original error %v", err, transportErr)
			}
			if tt.wantMsg != "" && err.Error() != tt.wantMsg {
				t.Errorf("apiError() = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestFilterLabels(t *testing.T) {
	t.Parallel()

	pin := map[pinFilter]string{
		pinAll:      "all",
		pinPinned:   "pinned",
		pinUnpinned: "unpinned",
	}
	for f, want := range pin {
		if got := f.label(); got != want {
			t.Errorf("pinFilter(%d).label() = %q, want %q", f, got, want)
		}
	}
	if got := pinFilter(99).label(); got == "" {
		t.Error("unknown pinFilter must not render an empty label")
	}

	state := map[StateFilter]string{
		StateActiveInactive: "active + inactive",
		StateAll:            "all",
		StateActive:         "active",
		StateDeleted:        "deleted",
	}
	for f, want := range state {
		if got := f.label(); got != want {
			t.Errorf("StateFilter(%d).label() = %q, want %q", f, got, want)
		}
	}
	if got := StateFilter(99).label(); got == "" {
		t.Error("unknown StateFilter must not render an empty label")
	}
}

func TestFormatLabelsCoverCycleOrder(t *testing.T) {
	t.Parallel()

	// Every format option in the cycle order must have a non-empty
	// display label and API value; they come from the API spec.
	for i, f := range aliasFormats {
		if f.value == "" || f.label == "" {
			t.Errorf("aliasFormats[%d] has an empty value or label: %+v", i, f)
		}
	}
}
