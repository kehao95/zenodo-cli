package config

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kehao95/zenodo-cli/internal/errs"
)

const Production = "https://zenodo.org/api"
const Sandbox = "https://sandbox.zenodo.org/api"

type File struct {
	Tokens map[string]string `json:"tokens"`
}

func Path(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if p := os.Getenv("ZENODO_CLI_CONFIG"); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "zenodo-cli", "config.json"), nil
}

func Endpoint(base string, sandbox bool) (string, error) {
	if sandbox && base != "" {
		return "", errs.New(2, "configuration", "--sandbox and --base-url are mutually exclusive")
	}
	if sandbox {
		return Sandbox, nil
	}
	if base == "" {
		base = os.Getenv("ZENODO_BASE_URL")
	}
	if base == "" {
		base = Production
	}
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", errs.New(2, "configuration", "base URL must be an absolute API URL without credentials, query, or fragment")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", errs.New(2, "configuration", "HTTPS is required (HTTP is allowed only on loopback for tests)")
	}
	if u.Path != "/api" || u.RawPath != "" {
		return "", errs.New(2, "configuration", "base URL path must be /api")
	}
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func Read(path string) (File, error) {
	var f File
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return File{Tokens: map[string]string{}}, nil
	}
	if err != nil {
		return f, errs.New(2, "configuration", "cannot read config: "+err.Error())
	}
	if json.Unmarshal(b, &f) != nil {
		return f, errs.New(2, "configuration", "config must be a JSON object with a tokens map")
	}
	if f.Tokens == nil {
		f.Tokens = map[string]string{}
	}
	return f, nil
}

func Token(path, endpoint string) (string, string, error) {
	key := ""
	if endpoint == Production {
		key = "ZENODO_ACCESS_TOKEN"
	}
	if endpoint == Sandbox {
		key = "ZENODO_SANDBOX_ACCESS_TOKEN"
	}
	if key != "" && os.Getenv(key) != "" {
		return strings.TrimSpace(os.Getenv(key)), key, nil
	}
	f, err := Read(path)
	if err != nil {
		return "", "", err
	}
	return f.Tokens[endpoint], "config", nil
}

func Save(path, endpoint, token string) error {
	f, err := Read(path)
	if err != nil {
		return err
	}
	if token == "" {
		delete(f.Tokens, endpoint)
	} else {
		f.Tokens[endpoint] = token
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return nil
}

func ReadOnly() (bool, error) {
	s := os.Getenv("ZENODO_CLI_READ_ONLY")
	if s == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, errs.New(2, "configuration", "invalid ZENODO_CLI_READ_ONLY: expected true or false")
	}
	return b, nil
}
