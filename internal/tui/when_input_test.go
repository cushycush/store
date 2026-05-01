package tui

import (
	"reflect"
	"testing"

	"github.com/cushycush/store/v2/internal/config"
)

func TestParseWhenExpression(t *testing.T) {
	tr := true
	fa := false
	tests := []struct {
		name    string
		in      string
		want    *config.WhenClause
		wantErr string
	}{
		{name: "empty clears", in: "", want: nil},
		{name: "whitespace clears", in: "   ", want: nil},
		{name: "single os", in: "os=linux", want: &config.WhenClause{OS: config.Strings{"linux"}}},
		{name: "comma list", in: "os=linux,darwin", want: &config.WhenClause{OS: config.Strings{"linux", "darwin"}}},
		{
			name: "two fields",
			in:   "os=linux shell=zsh",
			want: &config.WhenClause{OS: config.Strings{"linux"}, Shell: config.Strings{"zsh"}},
		},
		{name: "wsl true", in: "wsl=true", want: &config.WhenClause{WSL: &tr}},
		{name: "wsl false via 0", in: "wsl=0", want: &config.WhenClause{WSL: &fa}},
		{name: "all known keys", in: "os=linux arch=amd64 distro=arch distro_version=rolling hostname=box shell=zsh wsl=false",
			want: &config.WhenClause{
				OS:            config.Strings{"linux"},
				Arch:          config.Strings{"amd64"},
				Distro:        config.Strings{"arch"},
				DistroVersion: config.Strings{"rolling"},
				Hostname:      config.Strings{"box"},
				Shell:         config.Strings{"zsh"},
				WSL:           &fa,
			}},
		{name: "missing equals", in: "linux", wantErr: "expected key=value"},
		{name: "empty value", in: "os=", wantErr: "expected key=value"},
		{name: "unknown key", in: "platform=linux", wantErr: "unknown when key"},
		{name: "bad wsl", in: "wsl=maybe", wantErr: "wsl="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWhenExpression(tt.in)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseWhenExpression(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatWhenExpressionRoundTrip(t *testing.T) {
	tr := true
	cases := []*config.WhenClause{
		nil,
		{OS: config.Strings{"linux"}},
		{OS: config.Strings{"linux", "darwin"}, Shell: config.Strings{"zsh"}},
		{Hostname: config.Strings{"work-laptop"}, WSL: &tr},
	}
	for _, want := range cases {
		s := formatWhenExpression(want)
		got, err := parseWhenExpression(s)
		if err != nil {
			t.Fatalf("round trip parse(%q) error = %v", s, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round trip: got %+v from %q, want %+v", got, s, want)
		}
	}
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
