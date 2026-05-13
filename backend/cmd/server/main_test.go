package main

import (
	"strings"
	"testing"
)

func TestRedactRemote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "tokenized https remote",
			in:   "https://user:secret-token@github.com/Aolda/aods-manifest.git",
			want: "https://user:%3Credacted%3E@github.com/Aolda/aods-manifest.git",
		},
		{
			name: "password without username",
			in:   "https://:secret-token@github.com/Aolda/aods-manifest.git",
			want: "https://redacted:%3Credacted%3E@github.com/Aolda/aods-manifest.git",
		},
		{
			name: "no password",
			in:   "https://github.com/Aolda/aods-manifest.git",
			want: "https://github.com/Aolda/aods-manifest.git",
		},
		{
			name: "invalid url passes through",
			in:   "://bad-url",
			want: "://bad-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := redactRemote(tt.in); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
			got := redactRemote(tt.in)
			if strings.Contains(got, "secret-token") {
				t.Fatalf("remote was not redacted: %q", got)
			}
		})
	}
}

func TestHostnameWorkerIDReturnsStableFallbackOrHostname(t *testing.T) {
	t.Parallel()

	if got := hostnameWorkerID(); strings.TrimSpace(got) == "" {
		t.Fatal("expected non-empty worker id")
	}
}
