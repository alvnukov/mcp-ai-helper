package config

import (
	"strings"
	"testing"
)

func TestResolveValuesMergesHandlesVarsAndEnv(t *testing.T) {
	cfg := Config{Secrets: map[string]SecretConfig{
		"GH_TOKEN": {Value: "fake-gh-token-42a7b9c1d3e5f", Enabled: true},
	}}
	resolved, err := cfg.ResolveValues([]string{"GH_TOKEN"}, map[string]string{"BUILD_ID": "b-17"}, map[string]string{"WHO": "env"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Vars["GH_TOKEN"] != "fake-gh-token-42a7b9c1d3e5f" || resolved.Vars["BUILD_ID"] != "b-17" {
		t.Fatalf("vars = %#v", resolved.Vars)
	}
	joined := strings.Join(resolved.Env, "\n")
	for _, want := range []string{"HELPER_SECRET_GH_TOKEN=fake-gh-token-42a7b9c1d3e5f", "GH_TOKEN=fake-gh-token-42a7b9c1d3e5f", "WHO=env"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("env missing %q in %#v", want, resolved.Env)
		}
	}
}

func TestResolveValuesFailsClosedOnCollisions(t *testing.T) {
	cfg := Config{Secrets: map[string]SecretConfig{
		"GH_TOKEN": {Value: "fake-gh-token-42a7b9c1d3e5f", Enabled: true},
	}}
	cases := []struct {
		name    string
		vars    map[string]string
		env     map[string]string
		handles []string
		want    string
	}{
		{name: "var collides with handle", vars: map[string]string{"GH_TOKEN": "x"}, handles: []string{"GH_TOKEN"}, want: "collides with secret handle"},
		{name: "env collides with handle", env: map[string]string{"GH_TOKEN": "x"}, handles: []string{"GH_TOKEN"}, want: "collides with secret handle"},
		{name: "reserved handle name", handles: []string{"PATH"}, want: "reserved environment name"},
		{name: "invalid var name", vars: map[string]string{"1bad": "x"}, want: "invalid vars name"},
		{name: "invalid env name", env: map[string]string{"BAD=KEY": "x"}, want: "invalid env name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cfg.ResolveValues(tc.handles, tc.vars, tc.env)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
