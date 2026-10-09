package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/kehao95/zenodo-cli/internal/errs"
)

const Production = "https://zenodo.org/api"

func Endpoint(base string) (string, error) {
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
