package actions

import (
	"time"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/agent"
)

// Unlock decrypts the device key and caches it in the agent for ttl.
func Unlock(passphrase string, ttl time.Duration) error {
	return agent.Unlock(passphrase, ttl)
}

// Lock forgets the cached agent key immediately.
func Lock() error {
	return agent.Lock()
}

// IsUnlocked reports whether the agent currently holds an unlocked device key.
func IsUnlocked() bool {
	_, ok := agent.Cached()
	return ok
}
