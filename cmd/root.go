package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kehao95/zenodo-cli/internal/config"
	"github.com/kehao95/zenodo-cli/internal/errs"
	"github.com/kehao95/zenodo-cli/internal/zenodo"
	"github.com/spf13/cobra"
)

type app struct {
	client                   *zenodo.Client
	source                   string
	base                     string
	sandbox, human, readOnly bool
	confirm                  string
	timeout                  time.Duration
	retries                  int
}

func New(in io.Reader, out, errOut io.Writer, version string) *cobra.Command {
	a := &app{}
	root := &cobra.Command{Use: "zenodo", Short: "Zenodo for agents: JSON-first, explicit writes", Version: version, SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs}
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.RunE = func(cmd *cobra.Command, args []string) error { return cmd.Help() }
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return errs.New(2, "invalid_input", err.Error()) })
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		endpoint, err := config.Endpoint(a.base, a.sandbox)
		if err != nil {
			return err
		}
		token, source := config.Token(endpoint)
		a.source = source
		envRO, err := config.ReadOnly()
		if err != nil {
			return err
		}
		if a.timeout <= 0 || a.retries < 0 || a.retries > 5 {
			return errs.New(2, "invalid_input", "timeout must be positive and retries must be 0..5")
		}
		a.client, err = zenodo.New(endpoint, token, zenodo.Policy{ReadOnly: envRO || a.readOnly, Confirm: a.confirm}, a.timeout, a.retries)
		return err
	}
	f := root.PersistentFlags()
	f.StringVar(&a.base, "base-url", "", "API URL (default: ZENODO_BASE_URL or https://zenodo.org/api)")
	f.BoolVar(&a.sandbox, "sandbox", false, "use sandbox and its separate ZENODO_SANDBOX_ACCESS_TOKEN")
	f.BoolVarP(&a.human, "human", "H", false, "pretty JSON for human inspection")
	f.BoolVar(&a.readOnly, "read-only", false, "block every remote mutation")
	f.StringVar(&a.confirm, "confirm", "", "target deposition ID for publish, delete or discard")
	f.DurationVar(&a.timeout, "timeout", 2*time.Minute, "HTTP request timeout (increase for large transfers)")
	f.IntVar(&a.retries, "retries", 2, "GET retries on 429/503; writes are never retried")
	root.AddCommand(a.auth(), a.records(), a.depositions(), a.files(), a.licenses(), a.api(), a.metadata())
	return root
}

func Execute(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, version string) int {
	root := New(in, out, errOut, version)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		e := errs.As(err)
		// Cobra usage errors are not typed, but still belong to input/configuration.
		if e.Code == 1 && (strings.HasPrefix(e.Message, "unknown command") || strings.HasPrefix(e.Message, "accepts ") || strings.HasPrefix(e.Message, "requires ") || strings.Contains(e.Message, "arg(s)")) {
			e.Code = 2
			e.Kind = "invalid_input"
		}
		_ = json.NewEncoder(errOut).Encode(map[string]any{"error": e})
		return e.Code
	}
	return 0
}

func (a *app) print(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	if a.human {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

func (a *app) request(cmd *cobra.Command, method, path string, body any, auth bool) error {
	u, err := a.client.URL(path)
	if err != nil {
		return err
	}
	if err = a.client.Check(method, u); err != nil {
		return err
	}
	if auth {
		if err = a.client.RequireToken(); err != nil {
			return err
		}
	}
	result, err := a.client.JSON(cmd.Context(), method, path, body)
	if err != nil {
		return err
	}
	return a.print(cmd, result)
}

func input(cmd *cobra.Command, spec string) (any, error) {
	var b []byte
	var err error
	if spec == "-" {
		b, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), 8<<20+1))
	} else if strings.HasPrefix(spec, "@") {
		b, err = readInputFile(spec[1:])
	} else {
		b = []byte(spec)
	}
	if err != nil {
		return nil, errs.New(2, "invalid_input", err.Error())
	}
	if len(b) > 8<<20 {
		return nil, errs.New(2, "invalid_input", "JSON input exceeds 8 MiB")
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	var v any
	if err = dec.Decode(&v); err != nil {
		return nil, errs.New(2, "invalid_input", "invalid JSON input")
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return nil, errs.New(2, "invalid_input", "expected exactly one JSON value")
	}
	return v, nil
}

func id(s string) (string, error) {
	if s == "" || s[0] == '0' {
		return "", errs.New(2, "invalid_input", "record/deposition ID must be a positive integer")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", errs.New(2, "invalid_input", "record/deposition ID must be a positive integer")
		}
	}
	return s, nil
}

func depositPath(s string) (string, error) { s, err := id(s); return "/deposit/depositions/" + s, err }

func metadataInput(cmd *cobra.Command, spec string) (map[string]any, error) {
	v, err := input(cmd, spec)
	if err != nil {
		return nil, err
	}
	m := zenodo.Object(v)
	if m == nil {
		return nil, errs.New(2, "invalid_input", "metadata must be a JSON object")
	}
	if nested, ok := m["metadata"]; ok {
		m = zenodo.Object(nested)
		if m == nil {
			return nil, errs.New(2, "invalid_input", "metadata field must be an object")
		}
	}
	return m, nil
}

func require(s, label string) error {
	if strings.TrimSpace(s) == "" {
		return errs.New(2, "invalid_input", fmt.Sprintf("%s is required", label))
	}
	return nil
}
