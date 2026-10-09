package cmd

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadAndCollision(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(t.TempDir(), "paper file.txt")
	content := "research payload\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	collision := false
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			files := "[]"
			if collision {
				files = `[{"filename":"paper file.txt"}]`
			}
			fmt.Fprintf(w, `{"files":%s,"links":{"bucket":%q}}`, files, f.server.URL+"/api/files/bucket")
			return
		}
		if r.Method != "PUT" || r.URL.Path != "/api/files/bucket/paper file.txt" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != content || r.ContentLength != int64(len(content)) {
			t.Errorf("body=%q length=%d", b, r.ContentLength)
		}
		h := md5.Sum(b)
		fmt.Fprintf(w, `{"key":"paper file.txt","checksum":"md5:%s"}`, hex.EncodeToString(h[:]))
	}
	code, out, e := f.run("", "files", "upload", "42", path)
	if code != 0 || !strings.Contains(out, "checksum") {
		t.Fatalf("%d %s %s", code, out, e)
	}
	collision = true
	before := len(f.calls)
	code, _, _ = f.run("", "files", "upload", "42", path)
	if code != 2 || len(f.calls) != before+1 {
		t.Fatal("collision not blocked before PUT")
	}
	if code, _, e = f.run("", "files", "upload", "42", path, "--replace"); code != 0 {
		t.Fatal(e)
	}
	before = len(f.calls)
	t.Setenv("ZENODO_CLI_READ_ONLY", "true")
	if code, _, _ = f.run("", "files", "upload", "42", path); code != 6 || len(f.calls) != before {
		t.Fatal("readonly upload touched server")
	}
}

func TestDownloadChecksumAndNoClobber(t *testing.T) {
	f := newFixture(t)
	content := "abc"
	checksum := "md5:900150983cd24fb0d6963f7d28e17f72"
	dest := filepath.Join(t.TempDir(), "paper.txt")
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/records/42" {
			fmt.Fprintf(w, `{"files":[{"key":"paper.txt","checksum":%q,"links":{"self":%q}}]}`, checksum, f.server.URL+"/api/files/bucket/paper.txt")
			return
		}
		fmt.Fprint(w, content)
		if r.Header.Get("Accept") != "*/*" {
			t.Error("file download must allow Zenodo content negotiation")
		}
	}
	code, out, e := f.run("", "records", "download", "42", "paper.txt", "--output", dest)
	if code != 0 || !strings.Contains(out, `"checksum_verified":true`) {
		t.Fatalf("%d %s %s", code, out, e)
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "abc" {
		t.Fatal(string(b))
	}
	before := len(f.calls)
	code, _, _ = f.run("", "records", "download", "42", "paper.txt", "-o", dest)
	if code != 2 || len(f.calls) != before+1 {
		t.Fatal("existing file fetched")
	}
	content = "bad checksum"
	code, out, e = f.run("", "records", "download", "42", "paper.txt", "-o", dest, "--force")
	if code != 1 || out != "" || !strings.Contains(e, "checksum_mismatch") {
		t.Fatalf("%d %s %s", code, out, e)
	}
	b, _ = os.ReadFile(dest)
	if string(b) != "abc" {
		t.Fatal("checksum mismatch clobbered destination")
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".zenodo-download-*"))
	if len(matches) != 0 {
		t.Fatal(matches)
	}
}

func TestCurrentFileShapeAndDraftDownload(t *testing.T) {
	f := newFixture(t)
	dest := filepath.Join(t.TempDir(), "data.txt")
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/deposit/") {
			fmt.Fprintf(w, `{"files":{"entries":{"data.txt":{"checksum":"900150983cd24fb0d6963f7d28e17f72","links":{"content":%q}}}}}`, f.server.URL+"/api/files/bucket/data.txt")
			return
		}
		fmt.Fprint(w, "abc")
	}
	code, _, e := f.run("", "files", "download", "42", "data.txt", "-o", dest)
	if code != 0 {
		t.Fatal(e)
	}
}

func TestExportRawAndRecordFiles(t *testing.T) {
	f := newFixture(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/x-bibtex" {
			fmt.Fprint(w, "@dataset{example}\n")
			return
		}
		fmt.Fprint(w, `{"files":[{"key":"a.txt"}]}`)
	}
	code, out, e := f.run("", "records", "export", "42")
	if code != 0 || out != "@dataset{example}\n" || e != "" {
		t.Fatalf("%d %s %s", code, out, e)
	}
	code, out, e = f.run("", "records", "files", "42")
	if code != 0 || out != "[{\"key\":\"a.txt\"}]\n" {
		t.Fatalf("%d %s %s", code, out, e)
	}
}
