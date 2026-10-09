package zenodo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadSHA256(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "abc") }))
	defer server.Close()
	c, _ := New(server.URL+"/api", "", Policy{}, time.Second, 0)
	h := sha256.Sum256([]byte("abc"))
	dest := filepath.Join(t.TempDir(), "file.txt")
	if _, err := c.Download(context.Background(), "/files/file.txt", dest, "sha256:"+hex.EncodeToString(h[:]), false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "abc" {
		t.Fatal(string(b))
	}
	if _, err := c.Download(context.Background(), "/files/file.txt", dest, "sha512:unsupported", true); err == nil {
		t.Fatal("unsupported checksum accepted")
	}
	if b, _ := os.ReadFile(dest); string(b) != "abc" {
		t.Fatal("existing destination changed")
	}
}
