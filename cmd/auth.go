package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/kehao95/zenodo-cli/internal/config"
	"github.com/kehao95/zenodo-cli/internal/errs"
	"github.com/spf13/cobra"
)

func readInputFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 8<<20+1))
}

func (a *app) auth() *cobra.Command {
	group := &cobra.Command{Use: "auth", Short: "Endpoint-specific token configuration", Args: cobra.NoArgs}
	status := &cobra.Command{Use: "status", Short: "Show token source without revealing the token", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return a.print(cmd, map[string]any{"endpoint": a.client.Base.String(), "configured": a.client.Token != "", "source": a.source, "config": a.configPath, "read_only": a.client.Policy.ReadOnly})
	}}
	test := &cobra.Command{Use: "test", Short: "Verify token using GET of your depositions (no mutations)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := a.client.RequireToken(); err != nil {
			return err
		}
		_, err := a.client.JSON(cmd.Context(), "GET", "/deposit/depositions?size=1", nil)
		if err != nil {
			return err
		}
		return a.print(cmd, map[string]any{"ok": true, "endpoint": a.client.Base.String(), "source": a.source})
	}}
	var stdin, verify bool
	login := &cobra.Command{Use: "login", Short: "Save a token from stdin (0600 config; never echoes it)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !stdin {
			return errs.New(2, "invalid_input", "use --token-stdin to avoid credentials in shell history")
		}
		b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 16<<10+1))
		if err != nil {
			return err
		}
		token := strings.TrimSpace(string(b))
		if token == "" || len(b) > 16<<10 || strings.ContainsAny(token, "\r\n\t ") {
			return errs.New(2, "invalid_input", "expected one nonempty token on stdin")
		}
		a.client.Token = token
		if verify {
			if _, err = a.client.JSON(cmd.Context(), "GET", "/deposit/depositions?size=1", nil); err != nil {
				return err
			}
		}
		if err = config.Save(a.configPath, a.client.Base.String(), token); err != nil {
			return err
		}
		return a.print(cmd, map[string]any{"ok": true, "endpoint": a.client.Base.String(), "config": a.configPath, "verified": verify})
	}}
	login.Flags().BoolVar(&stdin, "token-stdin", false, "read token from stdin")
	login.Flags().BoolVar(&verify, "verify", false, "verify via GET before saving")
	logout := &cobra.Command{Use: "logout", Short: "Remove saved token for this endpoint (environment overrides remain)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := config.Save(a.configPath, a.client.Base.String(), ""); err != nil {
			return err
		}
		return a.print(cmd, map[string]any{"ok": true, "endpoint": a.client.Base.String(), "environment_token_remains": a.source != "config"})
	}}
	group.AddCommand(status, test, login, logout)
	return group
}
