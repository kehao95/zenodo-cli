package cmd

import (
	"io"
	"os"

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
	group := &cobra.Command{Use: "auth", Short: "Inspect and verify environment authentication", Args: cobra.NoArgs}
	group.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	status := &cobra.Command{Use: "status", Short: "Show token source without revealing the token", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return a.print(cmd, map[string]any{"endpoint": a.client.Base.String(), "configured": a.client.Token != "", "source": "ZENODO_ACCESS_TOKEN", "read_only": a.client.Policy.ReadOnly})
	}}
	test := &cobra.Command{Use: "test", Short: "Verify token using GET of your depositions (no mutations)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := a.client.RequireToken(); err != nil {
			return err
		}
		_, err := a.client.JSON(cmd.Context(), "GET", "/deposit/depositions?size=1", nil)
		if err != nil {
			return err
		}
		return a.print(cmd, map[string]any{"ok": true, "endpoint": a.client.Base.String(), "source": "ZENODO_ACCESS_TOKEN"})
	}}
	group.AddCommand(status, test)
	return group
}
