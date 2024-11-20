package main

import (
	"fmt"
	"net/url"
	"os"

	"github.com/antgroup/hugescm/modules/survey"
)

func main() {
	base, err := url.Parse("https://zeta.example.io")
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad url: %v\n", err)
		return
	}
	var username, password string
	pu := &survey.Input{
		Message: fmt.Sprintf("Username for '%s':", base.String()),
	}
	if err := survey.AskOne(pu, &username); err != nil {
		fmt.Fprintf(os.Stderr, "AskOne error: %v\n", err)
		return
	}
	prompt := &survey.Password{
		Message: fmt.Sprintf("Password for '%s://%s@%s':", base.Scheme, url.PathEscape(username), base.Host),
	}
	if err := survey.AskOne(prompt, &password); err != nil {
		fmt.Fprintf(os.Stderr, "AskOne error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Username: %v password: %v\n", username, password)
}
