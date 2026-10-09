package zenodo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kehao95/zenodo-cli/internal/errs"
)

func TestAPIErrorCodesAndRedaction(t *testing.T) {
	for status, code := range map[int]int{400: 2, 401: 3, 403: 6, 404: 7, 409: 1, 415: 2, 422: 2, 429: 4, 500: 1} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"message":"bad secret-token","errors":[{"field":"title","message":"secret-token"}]}`)
			}))
			defer server.Close()
			c, _ := New(server.URL+"/api", "secret-token", Policy{}, time.Second, 0)
			_, err := c.JSON(context.Background(), "GET", "/records", nil)
			e := errs.As(err)
			if e.Code != code || e.Status != status || strings.Contains(fmt.Sprint(e.Details), "secret-token") || strings.Contains(e.Message, "secret-token") {
				t.Fatalf("%+v", e)
			}
		})
	}
}

func TestRetriesOnlyGET(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(429)
					fmt.Fprint(w, `{"message":"rate limit"}`)
				} else {
					fmt.Fprint(w, `{"ok":true}`)
				}
			}))
			defer server.Close()
			c, _ := New(server.URL+"/api", "test-token", Policy{Confirm: "42"}, time.Second, 2)
			_, err := c.JSON(context.Background(), method, "/deposit/depositions/42", nil)
			if method == "GET" {
				if err != nil || calls != 2 {
					t.Fatalf("calls=%d err=%v", calls, err)
				}
			} else if errs.As(err).Code != 4 || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestRedirectsNeverLeakOrReplayWrites(t *testing.T) {
	received := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received++; fmt.Fprint(w, `{}`) }))
	defer other.Close()
	for _, method := range []string{"GET", "POST"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				http.Redirect(w, r, other.URL+"/api/records", 307)
			}))
			defer server.Close()
			c, _ := New(server.URL+"/api", "test-token", Policy{}, time.Second, 0)
			_, err := c.JSON(context.Background(), method, "/deposit/depositions", nil)
			if err == nil || received != 0 || calls != 1 {
				t.Fatalf("calls=%d received=%d err=%v", calls, received, err)
			}
		})
	}
}

func TestMutationSameOriginRedirectBlocked(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "/api/deposit/depositions/42", 302)
	}))
	defer server.Close()
	c, _ := New(server.URL+"/api", "test-token", Policy{}, time.Second, 0)
	_, err := c.JSON(context.Background(), "POST", "/deposit/depositions", nil)
	if err == nil || calls != 1 {
		t.Fatal(calls, err)
	}
}

func TestCancellationDuringBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "5"); w.WriteHeader(429) }))
	defer server.Close()
	c, _ := New(server.URL+"/api", "", Policy{}, time.Second, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.JSON(ctx, "GET", "/records", nil)
	if err == nil || time.Since(start) > time.Second {
		t.Fatal(err)
	}
}

func TestCanonicalURLsAndPolicy(t *testing.T) {
	c, _ := New("https://zenodo.org/api", "", Policy{}, time.Second, 0)
	for _, path := range []string{"//other.invalid/api", "/../records", "/deposit//depositions", "/deposit/depositions/42/actions/%70ublish", "https://zenodo.org/api/records?access_token=x", "https://zenodo.org/records/42", "http://zenodo.org/api/records"} {
		if _, err := c.URL(path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	for _, path := range []string{"/records?q=a%2Fb", "https://zenodo.org/api/records/42", "/files/uuid/paper%20file.txt"} {
		if _, err := c.URL(path); err != nil {
			t.Errorf("rejected %q: %v", path, err)
		}
	}
	for _, path := range []string{"/deposit/depositions/42/actions/publish", "/deposit/depositions/42/actions/publish/", "/deposit/depositions/42/actions/discard"} {
		u, _ := c.URL(path)
		if err := c.Check("POST", u); errs.As(err).Code != 6 {
			t.Error(path, err)
		}
	}
}
