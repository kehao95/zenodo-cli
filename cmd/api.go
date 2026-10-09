package cmd

import (
	"strings"

	"github.com/kehao95/zenodo-cli/internal/errs"
	"github.com/spf13/cobra"
)

func (a *app) api() *cobra.Command {
	var data string
	var params []string
	c := &cobra.Command{Use: "api METHOD /PATH", Short: "Generic REST JSON request using the same auth and safety policy", Example: "  zenodo api GET /records --param q=climate --param size=5\n  zenodo api POST /deposit/depositions --data '{\"metadata\":{\"title\":\"Draft\"}}'", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		method := strings.ToUpper(args[0])
		path := args[1]
		u, err := a.client.URL(path)
		if err != nil {
			return err
		}
		q := u.Query()
		for _, p := range params {
			k, v, ok := strings.Cut(p, "=")
			if !ok || k == "" {
				return errs.New(2, "invalid_input", "--param requires key=value")
			}
			q.Add(k, v)
		}
		u.RawQuery = q.Encode()
		path = u.String()
		var body any
		if data != "" {
			if method == "GET" || method == "HEAD" {
				return errs.New(2, "invalid_input", "GET/HEAD requests do not accept --data")
			}
			body, err = input(cmd, data)
			if err != nil {
				return err
			}
		}
		if method == "HEAD" {
			return errs.New(2, "invalid_input", "api supports GET, POST, PUT, PATCH and DELETE JSON responses")
		}
		return a.request(cmd, method, path, body, method != "GET")
	}}
	c.Flags().StringVar(&data, "data", "", "JSON, @file or - for stdin")
	c.Flags().StringArrayVar(&params, "param", nil, "query key=value; repeat for multiple values")
	return c
}
