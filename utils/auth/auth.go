package main

import (
	"fmt"
	"net/url"
	"os"

	"github.com/antgroup/hugescm/modules/tui"
)

func main() {
	base, err := url.Parse("https://zeta.example.io")
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad url: %v\n", err)
		return
	}
	var username, password string

	if err := tui.AskInput(&username, "Username for '%s': ", base.String()); err != nil {
		fmt.Fprintf(os.Stderr, "ask username error: %v\n", err)
		return
	}

	if err := tui.AskPassword(&password, "Password for '%s://%s@%s': ", base.Scheme, url.PathEscape(username), base.Host); err != nil {
		fmt.Fprintf(os.Stderr, "ask password error: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "Username: %v password: %v\n", username, password)
}
