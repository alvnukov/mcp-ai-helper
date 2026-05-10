// Package security provides secret masking for MCP responses.
package security

import (
	"strings"
	"sync"
)

// Mask holds sensitive strings that must be redacted from output.
type Mask struct {
	mu       sync.RWMutex
	secrets  []string
}

// NewMask creates a Mask from sensitive values.
func NewMask(secrets ...string) *Mask {
	m := &Mask{}
	for _, s := range secrets {
		if s != "" {
			m.secrets = append(m.secrets, s)
		}
	}
	return m
}

// Add adds a secret value to the mask.
func (m *Mask) Add(secret string) {
	if secret == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets = append(m.secrets, secret)
}

// Apply replaces all known secrets in s with ***.
func (m *Mask) Apply(s string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, secret := range m.secrets {
		if secret != "" && strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}
