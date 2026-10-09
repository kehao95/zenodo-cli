package config

import "testing"

func TestEndpointValidation(t *testing.T) {
	t.Setenv("ZENODO_BASE_URL", "")
	tests := []struct {
		base, want string
		bad        bool
	}{
		{"", Production, false},
		{"http://127.0.0.1:5000/api/", "http://127.0.0.1:5000/api", false},
		{"https://ZENODO.ORG/api", Production, false},
		{"http://example.com/api", "", true}, {"https://token@example.com/api", "", true},
		{"https://zenodo.org/api?access_token=x", "", true}, {"https://zenodo.org/", "", true},
		{"https://zenodo.org/api#fragment", "", true},
	}
	for _, tt := range tests {
		got, err := Endpoint(tt.base)
		if (err != nil) != tt.bad || (!tt.bad && got != tt.want) {
			t.Errorf("%q: %q %v", tt.base, got, err)
		}
	}
	t.Setenv("ZENODO_BASE_URL", "https://custom.invalid/api")
	if got, _ := Endpoint(""); got != "https://custom.invalid/api" {
		t.Fatal(got)
	}
	if got, _ := Endpoint(Production); got != Production {
		t.Fatal("explicit endpoint did not override environment")
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
