// Command devtoken prints a sign-in token for a local dev/demo install that
// has no Portal. It signs with the jwt.secret in the given config, exactly
// like the Portal would. Only members of the demo team "dev-team" (created
// by docker/dev-portal-tables.sql) can be signed in, so the tool is useless
// against a real Portal database.
//
//	go run ./cmd/devtoken -c configs/config.yaml -user dev-admin
package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/middleware"
)

func main() {
	cfgPath := flag.String("c", "configs/config.yaml", "config file (same as the server's -c)")
	user := flag.String("user", "dev-admin", "users.id to sign in as")
	base := flag.String("base", "http://localhost:8900", "where the web app is served")
	flag.Parse()

	if err := config.Load(*cfgPath); err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}
	if err := database.Init(config.C.Database); err != nil {
		fmt.Fprintln(os.Stderr, "connect database:", err)
		os.Exit(1)
	}
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM team_members WHERE team_id='dev-team' AND user_id=?`, *user).Scan(&n)
	if n == 0 {
		fmt.Fprintf(os.Stderr, "refusing: %q is not a member of the demo team dev-team (dev/demo installs only; see docker/dev-portal-tables.sql)\n", *user)
		os.Exit(1)
	}
	tok, err := middleware.GenerateToken(*user, *user, "", "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(tok)
	fmt.Fprintf(os.Stderr, "\nOpen: %s/auth/callback?token=%s\n(valid 24h)\n", *base, url.QueryEscape(tok))
}
