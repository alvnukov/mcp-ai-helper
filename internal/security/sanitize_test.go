package security

import "testing"

func TestMask_Apply(t *testing.T) {
	m := NewMask("secret-token-123", "another-secret")
	tests := []struct{ input, expected string }{
		{"error: secret-token-123 failed", "error: *** failed"},
		{"no secrets here", "no secrets here"},
		{"both secret-token-123 and another-secret", "both *** and ***"},
	}
	for _, tt := range tests {
		got := m.Apply(tt.input)
		if got != tt.expected {
			t.Errorf("Apply(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMask_Add(t *testing.T) {
	m := NewMask()
	m.Add("new-secret")
	got := m.Apply("error: new-secret")
	if got != "error: ***" {
		t.Errorf("got %q", got)
	}
}

func TestMask_NoSecrets(t *testing.T) {
	m := NewMask()
	got := m.Apply("clean text")
	if got != "clean text" {
		t.Errorf("got %q", got)
	}
}
