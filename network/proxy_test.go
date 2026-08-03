package network

import "testing"

func TestParseConfiguredProxy(t *testing.T) {
	for _, test := range []struct {
		name  string
		raw   string
		want  string
		valid bool
	}{
		{name: "host and port", raw: "127.0.0.1:7897", want: "http://127.0.0.1:7897", valid: true},
		{name: "explicit scheme", raw: "http://127.0.0.1:7897", want: "http://127.0.0.1:7897", valid: true},
		{name: "windows proxy list", raw: "http=127.0.0.1:7897;https=127.0.0.1:7897", want: "http://127.0.0.1:7897", valid: true},
		{name: "empty", raw: "", valid: false},
		{name: "missing host", raw: "http://", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseConfiguredProxy(test.raw)
			if test.valid {
				if err != nil {
					t.Fatalf("parseConfiguredProxy() error: %v", err)
				}
				if got.String() != test.want {
					t.Fatalf("parseConfiguredProxy(%q) = %q, want %q", test.raw, got, test.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("parseConfiguredProxy(%q) returned %v, want error", test.raw, got)
			}
		})
	}
}
