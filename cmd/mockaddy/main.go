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
//
// The server itself lives in internal/mockaddy, where the test suite
// also uses it through net/http/httptest.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/kovmir/addyui/internal/mockaddy"
)

func main() {
	addr := flag.String("addr", mockaddy.DefaultAddr, "listen address")
	flag.Parse()

	fmt.Printf("mockaddy listening on http://%s (API key accepted: any)\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mockaddy.New()))
}
