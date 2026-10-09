package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kehao95/zenodo-cli/internal/config"
)

type fixture struct {
	t       *testing.T
	server  *httptest.Server
	path    string
	calls   []string
	handler http.HandlerFunc
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Setenv("ZENODO_CLI_READ_ONLY", "")
	t.Setenv("ZENODO_BASE_URL", "")
	t.Setenv("ZENODO_ACCESS_TOKEN", "")
	t.Setenv("ZENODO_SANDBOX_ACCESS_TOKEN", "")
	f := &fixture{t: t, path: filepath.Join(t.TempDir(), "config.json")}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls = append(f.calls, r.Method+" "+r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("incorrect Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		if f.handler != nil {
			f.handler(w, r)
		} else {
			fmt.Fprint(w, `{"ok":true}`)
		}
	}))
	t.Cleanup(f.server.Close)
	if err := config.Save(f.path, f.server.URL+"/api", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) run(stdin string, args ...string) (int, string, string) {
	f.t.Helper()
	var out, errOut bytes.Buffer
	argv := append([]string{"--config", f.path, "--base-url", f.server.URL + "/api", "--retries", "0"}, args...)
	code := Execute(context.Background(), argv, strings.NewReader(stdin), &out, &errOut, "test")
	return code, out.String(), errOut.String()
}

func TestCommandRoutes(t *testing.T) {
	tests := []struct {
		args               []string
		method, path, body string
	}{
		{[]string{"auth", "test"}, "GET", "/api/deposit/depositions?size=1", ""},
		{[]string{"records", "get", "42"}, "GET", "/api/records/42", ""},
		{[]string{"depositions", "get", "42"}, "GET", "/api/deposit/depositions/42", ""},
		{[]string{"depositions", "create"}, "POST", "/api/deposit/depositions", "{}"},
		{[]string{"drafts", "create", "--metadata", `{"title":"Example","unknown":{"x":9007199254740993}}`}, "POST", "/api/deposit/depositions", `{"metadata":{"title":"Example","unknown":{"x":9007199254740993}}}`},
		{[]string{"depositions", "update", "42", "--metadata", "-"}, "PUT", "/api/deposit/depositions/42", `{"metadata":{"title":"From stdin"}}`},
		{[]string{"depositions", "publish", "42", "--confirm", "42"}, "POST", "/api/deposit/depositions/42/actions/publish", ""},
		{[]string{"depositions", "edit", "42"}, "POST", "/api/deposit/depositions/42/actions/edit", ""},
		{[]string{"depositions", "discard", "42", "--confirm", "42"}, "POST", "/api/deposit/depositions/42/actions/discard", ""},
		{[]string{"depositions", "delete", "42", "--confirm", "42"}, "DELETE", "/api/deposit/depositions/42", ""},
		{[]string{"files", "list", "42"}, "GET", "/api/deposit/depositions/42/files", ""},
		{[]string{"files", "get", "42", "uuid"}, "GET", "/api/deposit/depositions/42/files/uuid", ""},
		{[]string{"files", "delete", "42", "uuid", "--confirm", "42"}, "DELETE", "/api/deposit/depositions/42/files/uuid", ""},
		{[]string{"files", "rename", "42", "uuid", "renamed.csv"}, "PUT", "/api/deposit/depositions/42/files/uuid", `{"name":"renamed.csv"}`},
		{[]string{"files", "sort", "42", "b", "a"}, "PUT", "/api/deposit/depositions/42/files", `[{"id":"b"},{"id":"a"}]`},
		{[]string{"licenses", "get", "cc-by-4.0"}, "GET", "/api/vocabularies/licenses/cc-by-4.0", ""},
		{[]string{"api", "GET", "/records", "--param", "q=a b", "--param", "size=1"}, "GET", "/api/records?q=a+b&size=1", ""},
		{[]string{"api", "POST", "/deposit/depositions", "--data", "-"}, "POST", "/api/deposit/depositions", `{"metadata":{"title":"From stdin"}}`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			f := newFixture(t)
			f.handler = func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.RequestURI() != tt.path {
					t.Errorf("got %s %s", r.Method, r.URL.RequestURI())
				}
				var b bytes.Buffer
				b.ReadFrom(r.Body)
				if b.String() != tt.body {
					t.Errorf("body %q != %q", b.String(), tt.body)
				}
				if tt.body != "" && r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing content type")
				}
				if r.Method == "DELETE" {
					w.WriteHeader(204)
					return
				}
				fmt.Fprint(w, `{"id":42}`)
			}
			code, out, errOut := f.run(`{"metadata":{"title":"From stdin"}}`, tt.args...)
			if code != 0 || errOut != "" || !json.Valid([]byte(out)) {
				t.Fatalf("code=%d out=%s err=%s", code, out, errOut)
			}
			if len(f.calls) != 1 {
				t.Fatalf("calls %v", f.calls)
			}
		})
	}
}

func TestWriteSafetyBeforeDispatch(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  string
	}{
		{"publish confirmation", []string{"depositions", "publish", "42"}, ""},
		{"wrong confirmation", []string{"depositions", "publish", "42", "--confirm", "41"}, ""},
		{"generic confirmation", []string{"api", "POST", "/deposit/depositions/42/actions/publish/"}, ""},
		{"delete confirmation", []string{"files", "delete", "42", "uuid"}, ""},
		{"readonly create", []string{"depositions", "create"}, "true"},
		{"readonly reserve", []string{"depositions", "reserve-doi", "42"}, "true"},
		{"readonly newversion", []string{"depositions", "newversion", "42"}, "true"},
		{"readonly publish", []string{"depositions", "publish", "42", "--confirm", "42", "--read-only=false"}, "true"},
		{"readonly generic", []string{"api", "PUT", "/deposit/depositions/42", "--data", "{}"}, "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			t.Setenv("ZENODO_CLI_READ_ONLY", tt.env)
			code, out, _ := f.run("", tt.args...)
			if code != 6 || out != "" || len(f.calls) != 0 {
				t.Fatalf("code=%d out=%s calls=%v", code, out, f.calls)
			}
		})
	}
}

func TestRejectedInput(t *testing.T) {
	tests := [][]string{
		{"records", "get", "../42"}, {"records", "get", "0"}, {"records", "get"},
		{"records", "search", "--size", "101"}, {"records", "search", "--page", "0"},
		{"depositions", "update", "42"}, {"depositions", "create", "--metadata", "[]"},
		{"depositions", "create", "--metadata", `{"metadata":null}`}, {"depositions", "create", "--metadata", "{} {}"},
		{"files", "rename", "42", "id", "../name"}, {"files", "sort", "42", "a", "a"},
		{"api", "GET", "https://other.invalid/api/records"}, {"api", "GET", "/records?access_token=bad"},
		{"api", "POST", "/deposit/depositions/42/actions/%70ublish", "--confirm", "42"},
		{"api", "GET", "/../records"}, {"api", "GET", "/records", "--data", "{}"}, {"api", "GET", "/records", "--param", "noequal"},
		{"--dry-run", "depositions", "create"}, {"--timeout", "0s", "auth", "status"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f := newFixture(t)
			code, out, _ := f.run("", args...)
			if code == 0 || out != "" || len(f.calls) > 0 {
				t.Fatalf("code=%d out=%s calls=%v", code, out, f.calls)
			}
		})
	}
}

func TestPagination(t *testing.T) {
	for _, mode := range []string{"depositions", "records", "loop", "limit"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			f.handler = func(w http.ResponseWriter, r *http.Request) {
				if mode == "depositions" {
					if r.URL.Query().Get("page") == "1" {
						fmt.Fprint(w, `[{"id":1}]`)
					} else {
						fmt.Fprint(w, `[]`)
					}
					return
				}
				if r.URL.Query().Get("page") == "1" || mode == "loop" {
					fmt.Fprintf(w, `{"hits":{"hits":[{"id":1}]},"links":{"next":%q}}`, f.server.URL+"/api/records?page=2&size=1")
				} else {
					fmt.Fprint(w, `{"hits":{"hits":[{"id":2}]},"links":{}}`)
				}
			}
			args := []string{"records", "search", "--size", "1", "--all"}
			if mode == "depositions" {
				args[0] = "depositions"
				args[1] = "list"
			}
			if mode == "limit" {
				args = append(args, "--max-pages", "1")
			}
			code, out, errOut := f.run("", args...)
			if mode == "loop" {
				if code != 1 || out != "" {
					t.Fatalf("%d %s %s", code, out, errOut)
				}
				return
			}
			if code != 0 {
				t.Fatal(errOut)
			}
			var result struct {
				Pages    []any
				Complete bool
				Next     string
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			want := 2
			if mode == "limit" {
				want = 1
				if result.Complete || result.Next == "" {
					t.Fatal(out)
				}
			} else if !result.Complete {
				t.Fatal(out)
			}
			if len(result.Pages) != want {
				t.Fatal(out)
			}
		})
	}
}

func TestReserveAndNewVersion(t *testing.T) {
	f := newFixture(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/newversion"):
			fmt.Fprintf(w, `{"id":42,"links":{"latest_draft":%q}}`, f.server.URL+"/api/deposit/depositions/43")
		case r.URL.Path == "/api/deposit/depositions/43":
			fmt.Fprint(w, `{"id":43,"submitted":false}`)
		case r.Method == "GET":
			fmt.Fprint(w, `{"id":42,"metadata":{"title":"Keep","custom":{"nested":9007199254740993}}}`)
		case r.Method == "PUT":
			var body map[string]any
			dec := json.NewDecoder(r.Body)
			dec.UseNumber()
			dec.Decode(&body)
			m := body["metadata"].(map[string]any)
			if m["title"] != "Keep" || m["prereserve_doi"] != true || m["custom"].(map[string]any)["nested"] != json.Number("9007199254740993") {
				t.Fatal(body)
			}
			fmt.Fprint(w, `{"id":42}`)
		}
	}
	if code, _, e := f.run("", "depositions", "reserve-doi", "42"); code != 0 {
		t.Fatal(e)
	}
	code, out, e := f.run("", "depositions", "newversion", "42")
	if code != 0 || !strings.Contains(out, `"id":43`) {
		t.Fatalf("%d %s %s", code, out, e)
	}
}

func TestNewVersionReadFailureRetainsRecoveryContext(t *testing.T) {
	f := newFixture(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			fmt.Fprintf(w, `{"id":42,"links":{"latest_draft":%q}}`, f.server.URL+"/api/deposit/depositions/43")
			return
		}
		w.WriteHeader(404)
		fmt.Fprint(w, `{"message":"Draft not yet available"}`)
	}
	code, out, e := f.run("", "depositions", "newversion", "42")
	if code != 7 || out != "" || !strings.Contains(e, "new version created") || len(f.calls) != 2 {
		t.Fatalf("%d %s %s calls=%v", code, out, e, f.calls)
	}
}

func TestPreviewAndMetadata(t *testing.T) {
	f := newFixture(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":42,"metadata":{"title":"Draft","upload_type":"dataset","description":"Description","creators":[{"name":"Doe, Jane"}]}}`)
	}
	code, out, e := f.run("", "depositions", "preview", "42")
	if code != 0 || !strings.Contains(out, `"metadata_valid":true`) || !strings.Contains(out, "preview=1") {
		t.Fatalf("%d %s %s", code, out, e)
	}
	if len(f.calls) != 1 || !strings.HasPrefix(f.calls[0], "GET ") {
		t.Fatal(f.calls)
	}
	code, out, e = f.run("", "metadata", "template")
	if code != 0 {
		t.Fatal(e)
	}
	code, _, e = f.run(out, "metadata", "validate", "--metadata", "-")
	if code != 0 {
		t.Fatal(e)
	}
	code, out, e = f.run("", "metadata", "validate", "--metadata", `{"upload_type":"publication","publication_date":"wrong"}`)
	if code != 2 || out != "" || !strings.Contains(e, "publication_type") {
		t.Fatalf("%d %s %s", code, out, e)
	}
}

func TestAuthLoginStatusLogoutAndFileInput(t *testing.T) {
	f := newFixture(t)
	code, out, e := f.run("new-token\n", "auth", "login", "--token-stdin")
	if code != 0 || strings.Contains(out, "new-token") {
		t.Fatalf("%d %s %s", code, out, e)
	}
	saved, _ := config.Read(f.path)
	if saved.Tokens[f.server.URL+"/api"] != "new-token" {
		t.Fatal("token was not saved")
	}
	code, out, e = f.run("", "auth", "status", "--human")
	if code != 0 || !strings.Contains(out, `"configured": true`) || strings.Contains(out, "new-token") {
		t.Fatalf("%d %s %s", code, out, e)
	}
	if code, _, e = f.run("", "auth", "logout"); code != 0 {
		t.Fatal(e)
	}
	if code, _, _ = f.run("", "auth", "test"); code != 2 {
		t.Fatal("missing token accepted")
	}
	if code, _, _ = f.run("two tokens", "auth", "login", "--token-stdin"); code != 2 {
		t.Fatal("bad token accepted")
	}
	if code, _, _ = f.run("token", "auth", "login"); code != 2 {
		t.Fatal("missing token-stdin accepted")
	}
	if err := config.Save(f.path, f.server.URL+"/api", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if code, _, e = f.run("fixture-token\n", "auth", "login", "--token-stdin", "--verify"); code != 0 {
		t.Fatal(e)
	}
	metadata := filepath.Join(t.TempDir(), "metadata.json")
	os.WriteFile(metadata, []byte(`{"title":"File input"}`), 0600)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["metadata"].(map[string]any)["title"] != "File input" {
			t.Error(body)
		}
		fmt.Fprint(w, `{"id":42}`)
	}
	if code, _, e = f.run("", "depositions", "create", "--metadata", "@"+metadata); code != 0 {
		t.Fatal(e)
	}
}
