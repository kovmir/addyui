// This is addy.io terminal user interface.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kovmir/addyui/internal/api"
	"github.com/kovmir/addyui/internal/config"
	"github.com/kovmir/addyui/internal/tui"
	"github.com/kovmir/addyui/internal/tui/views"
)

var (
	flagNew     = flag.Bool("new", false, "Create a new alias")
	flagFind    = flag.String("find", "", "Search term to filter the alias list at startup")
	flagShowVer = flag.Bool("version", false, "Print the version and exit")
	version     = "dev" // set by goreleaser via ldflags
)

func main() {
	flag.Parse()

	if *flagShowVer {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client, err := api.New(cfg.BaseURL, cfg.APIKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not create API client:", err)
		os.Exit(1)
	}

	startupView := views.ViewList
	if *flagNew {
		startupView = views.ViewCreate
	}
	listFilter := views.StateActiveInactive
	if *flagFind != "" {
		listFilter = views.StateAll
	}
	p := tea.NewProgram(
		tui.NewAppModelWithView(client, startupView,
			tui.WithSearch(strings.TrimSpace(*flagFind)),
			tui.WithState(listFilter)))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}
