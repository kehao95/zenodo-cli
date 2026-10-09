package config

import "testing"

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

func TestEnvironmentCredentialIsolation(t *testing.T) {
	t.Setenv("ZENODO_ACCESS_TOKEN", " production-token ")
	t.Setenv("ZENODO_SANDBOX_ACCESS_TOKEN", "")
	t.Setenv("ZENODO_CUSTOM_ACCESS_TOKEN", "")
	for _, tt := range []struct{ endpoint, token, source string }{
		{Production, "production-token", "ZENODO_ACCESS_TOKEN"},
		{Sandbox, "", "ZENODO_SANDBOX_ACCESS_TOKEN"},
		{"https://custom.invalid/api", "", "ZENODO_CUSTOM_ACCESS_TOKEN"},
	} {
		token, source := Token(tt.endpoint)
		if token != tt.token || source != tt.source {
			t.Errorf("%s: incorrect credential selection", tt.endpoint)
		}
	}
	t.Setenv("ZENODO_SANDBOX_ACCESS_TOKEN", "sandbox-token")
	t.Setenv("ZENODO_CUSTOM_ACCESS_TOKEN", "custom-token")
	if token, _ := Token(Sandbox); token != "sandbox-token" {
		t.Fatal("sandbox credential not selected")
	}
	if token, _ := Token("https://custom.invalid/api"); token != "custom-token" {
		t.Fatal("custom credential not selected")
	}
}

func TestReadOnlyEnvironment(t *testing.T) {
	for _, value := range []string{"", "false", "true", "nonsense"} {
		t.Setenv("ZENODO_CLI_READ_ONLY", value)
		got, err := ReadOnly()
		if (err != nil) != (value == "nonsense") || got != (value == "true") {
			t.Errorf("%q: %v %v", value, got, err)
		}
	}
}
