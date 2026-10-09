package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEndpointValidation(t *testing.T) {
	t.Setenv("ZENODO_BASE_URL", "")
	tests := []struct {
		base    string
		sandbox bool
		want    string
		bad     bool
	}{
		{"", false, Production, false}, {"", true, Sandbox, false},
		{"http://127.0.0.1:5000/api/", false, "http://127.0.0.1:5000/api", false},
		{"https://ZENODO.ORG/api", false, Production, false},
		{"http://example.com/api", false, "", true}, {"https://token@example.com/api", false, "", true},
		{"https://zenodo.org/api?access_token=x", false, "", true}, {"https://zenodo.org/", false, "", true},
		{Production, true, "", true}, {"https://zenodo.org/api#fragment", false, "", true},
	}
	for _, tt := range tests {
		got, err := Endpoint(tt.base, tt.sandbox)
		if (err != nil) != tt.bad || (!tt.bad && got != tt.want) {
			t.Errorf("%q sandbox=%v: %q %v", tt.base, tt.sandbox, got, err)
		}
	}
	t.Setenv("ZENODO_BASE_URL", Sandbox)
	if got, _ := Endpoint("", false); got != Sandbox {
		t.Fatal(got)
	}
}

func TestCredentialIsolationAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := Save(path, Production, "saved-production"); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, Sandbox, "saved-sandbox"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("permissions %v %v", info, err)
	}
	t.Setenv("ZENODO_ACCESS_TOKEN", "env-production")
	t.Setenv("ZENODO_SANDBOX_ACCESS_TOKEN", "")
	for endpoint, want := range map[string]string{Production: "env-production", Sandbox: "saved-sandbox", "https://custom.invalid/api": ""} {
		token, _, err := Token(path, endpoint)
		if err != nil || token != want {
			t.Errorf("%s token selection failed", endpoint)
		}
	}
	t.Setenv("ZENODO_SANDBOX_ACCESS_TOKEN", "env-sandbox")
	if token, _, _ := Token(path, Sandbox); token != "env-sandbox" {
		t.Error("sandbox override failed")
	}
	if err = Save(path, Production, ""); err != nil {
		t.Fatal(err)
	}
	f, _ := Read(path)
	if _, ok := f.Tokens[Production]; ok {
		t.Fatal("logout failed")
	}
	if f.Tokens[Sandbox] != "saved-sandbox" {
		t.Fatal("logout changed other endpoint")
	}
}

func TestConfigPathsAndBadInput(t *testing.T) {
	t.Setenv("ZENODO_CLI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, _ := Path("")
	if filepath.Base(p) != "config.json" {
		t.Fatal(p)
	}
	t.Setenv("ZENODO_CLI_CONFIG", "override.json")
	if got, _ := Path(""); got != "override.json" {
		t.Fatal(got)
	}
	if got, _ := Path("explicit.json"); got != "explicit.json" {
		t.Fatal(got)
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("not json"), 0600)
	if _, err := Read(path); err == nil {
		t.Fatal("malformed config accepted")
	}
	t.Setenv("ZENODO_CLI_READ_ONLY", "nonsense")
	if _, err := ReadOnly(); err == nil {
		t.Fatal("malformed boolean accepted")
	}
}
