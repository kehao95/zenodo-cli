package zenodo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kehao95/zenodo-cli/internal/errs"
)

type Policy struct {
	ReadOnly bool
	Confirm  string
}

type Client struct {
	Base    *url.URL
	Token   string
	Policy  Policy
	HTTP    *http.Client
	Retries int
}

func New(base, token string, policy Policy, timeout time.Duration, retries int) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	c := &Client{Base: u, Token: token, Policy: policy, Retries: retries}
	c.HTTP = &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		// Reject all redirects on mutations: 301/302/303 can silently change POST to GET.
		if len(via) > 0 && via[0].Method != http.MethodGet && via[0].Method != http.MethodHead {
			return errs.New(6, "unsafe_redirect", "mutation redirects are not followed")
		}
		_, err := c.URL(req.URL.String())
		return err
	}}
	return c, nil
}

func (c *Client) Trust(u *url.URL) error {
	if u.Scheme != c.Base.Scheme || u.Host != c.Base.Host || u.User != nil || u.Fragment != "" {
		return errs.New(6, "unsafe_url", "refusing a link outside the selected Zenodo origin")
	}
	for k := range u.Query() {
		if strings.EqualFold(k, "access_token") {
			return errs.New(6, "unsafe_url", "access tokens in URLs are forbidden")
		}
	}
	return nil
}

func (c *Client) URL(path string) (*url.URL, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, errs.New(2, "invalid_input", "invalid API path")
	}
	if !u.IsAbs() {
		if u.Host != "" || !strings.HasPrefix(u.Path, "/") {
			return nil, errs.New(2, "invalid_input", "API path must start with / (relative to /api)")
		}
		u.Path = c.Base.Path + u.Path
		if u.RawPath != "" {
			u.RawPath = c.Base.EscapedPath() + u.RawPath
		}
		u.Scheme = c.Base.Scheme
		u.Host = c.Base.Host
	}
	if err = c.Trust(u); err != nil {
		return nil, err
	}
	// Reject alternate spellings before policy classification. Percent-encoded
	// separators, dot segments and doubled slashes can route differently upstream.
	if u.RawPath != "" || strings.Contains(u.Path, "//") || strings.ContainsAny(u.Path, "\\\x00") {
		return nil, errs.New(2, "invalid_input", "non-canonical API path")
	}
	for _, s := range strings.Split(u.Path, "/") {
		if s == "." || s == ".." {
			return nil, errs.New(2, "invalid_input", "dot segments are forbidden")
		}
	}
	if u.Path != c.Base.Path && !strings.HasPrefix(u.Path, c.Base.Path+"/") {
		return nil, errs.New(6, "unsafe_url", "API link is outside /api")
	}
	return u, nil
}

var confirmAction = regexp.MustCompile(`^/deposit/depositions/([1-9][0-9]*)(?:/actions/(publish|discard)|/files/[^/]+)?/?$`)

func (c *Client) Check(method string, u *url.URL) error {
	if method == http.MethodGet || method == http.MethodHead {
		return nil
	}
	if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
		return errs.New(2, "invalid_input", "unsupported HTTP method")
	}
	if c.Policy.ReadOnly {
		return errs.New(6, "read_only_violation", "ZENODO_CLI_READ_ONLY or --read-only blocks all remote writes")
	}
	path := strings.TrimPrefix(u.Path, c.Base.Path)
	if strings.Contains(path, "/actions/") {
		// Do not let alternate methods or unreviewed action names bypass confirmation.
		allowed := regexp.MustCompile(`^/deposit/depositions/[1-9][0-9]*/actions/(publish|discard|edit|newversion)/?$`)
		if method != "POST" || !allowed.MatchString(path) {
			return errs.New(6, "unreviewed_action", "unreviewed deposition action or method")
		}
	}
	m := confirmAction.FindStringSubmatch(path)
	needsConfirm := m != nil && (method == "DELETE" || m[2] == "publish" || m[2] == "discard")
	if needsConfirm && c.Policy.Confirm != m[1] {
		return errs.New(6, "confirmation_required", "this action requires --confirm "+m[1])
	}
	return nil
}

func (c *Client) RequireToken() error {
	if c.Token == "" {
		return errs.New(2, "missing_token", "no token configured; set ZENODO_ACCESS_TOKEN")
	}
	return nil
}

func (c *Client) Redact(s string) string {
	if c.Token != "" {
		s = strings.ReplaceAll(s, c.Token, "[REDACTED]")
	}
	return s
}

func (c *Client) Do(ctx context.Context, method, path string, body io.Reader, contentType, accept string) (*http.Response, error) {
	u, err := c.URL(path)
	if err != nil {
		return nil, err
	}
	if err = c.Check(method, u); err != nil {
		return nil, err
	}
	if method != "GET" && method != "HEAD" {
		if err = c.RequireToken(); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if sized, ok := body.(interface{ Size() int64 }); ok {
		req.ContentLength = sized.Size()
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("User-Agent", "zenodo-cli")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	for attempt := 0; ; attempt++ {
		resp, err := c.HTTP.Do(req)
		if err != nil {
			var typed *errs.Error
			if errors.As(err, &typed) {
				return nil, typed
			}
			return nil, errs.New(5, "network", c.Redact(err.Error()))
		}
		if method == "GET" && attempt < c.Retries && (resp.StatusCode == 429 || resp.StatusCode == 503) {
			delay := time.Duration(1<<attempt) * time.Second
			if value := resp.Header.Get("Retry-After"); value != "" {
				if seconds, e := strconv.Atoi(value); e == nil && seconds >= 0 {
					if seconds > 60 {
						return nil, c.responseError(resp)
					}
					delay = time.Duration(seconds) * time.Second
				} else if date, e := http.ParseTime(value); e == nil {
					delay = time.Until(date)
				}
			}
			if delay < 0 {
				delay = 0
			}
			// Do not retry earlier than requested when the server asks for a long wait.
			if delay > 60*time.Second {
				return nil, c.responseError(resp)
			}
			resp.Body.Close()
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, errs.New(5, "network", ctx.Err().Error())
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, c.responseError(resp)
		}
		return resp, nil
	}
}

func (c *Client) responseError(resp *http.Response) error {
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var payload struct {
		Message string `json:"message"`
		Errors  any    `json:"errors"`
	}
	_ = json.Unmarshal([]byte(c.Redact(string(b))), &payload)
	message := payload.Message
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	code, kind := 1, "api_error"
	switch resp.StatusCode {
	case 400, 405, 415, 422:
		code, kind = 2, "invalid_input"
	case 401:
		code, kind = 3, "authentication"
	case 403:
		code, kind = 6, "permission"
	case 404:
		code, kind = 7, "not_found"
	case 429:
		code, kind = 4, "rate_limit"
	}
	return &errs.Error{Code: code, Kind: kind, Status: resp.StatusCode, Message: message, Details: payload.Errors}
}

func (c *Client) JSON(ctx context.Context, method, path string, body any) (any, error) {
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	u, err := c.URL(path)
	if err != nil {
		return nil, err
	}
	if err = c.Check(method, u); err != nil {
		return nil, err
	}
	ct := ""
	if body != nil {
		ct = "application/json"
	}
	resp, err := c.Do(ctx, method, path, bytes.NewReader(raw), ct, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 204 || resp.ContentLength == 0 {
		return map[string]any{"ok": true, "status": resp.StatusCode}, nil
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 32<<20))
	dec.UseNumber()
	var result any
	if err = dec.Decode(&result); err == io.EOF {
		return map[string]any{"ok": true, "status": resp.StatusCode}, nil
	} else if err != nil {
		return nil, errs.New(1, "invalid_response", "Zenodo returned invalid or oversized JSON")
	}
	return result, nil
}

func Object(v any) map[string]any   { m, _ := v.(map[string]any); return m }
func String(v any) string           { s, _ := v.(string); return s }
func Link(v any, key string) string { return String(Object(Object(v)["links"])[key]) }
