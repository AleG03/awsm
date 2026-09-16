package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State is what one cycle remembers for the next one.
//
// It exists mostly for one reason: a cycle runs every minute, and a profile
// that needs a human stays in that condition until the human acts. Without
// memory the same notification would arrive sixty times an hour, which is the
// difference between a useful daemon and one that gets turned off.
type State struct {
	LastRun    time.Time `json:"last_run"`
	LastAction string    `json:"last_action"`
	LastReason string    `json:"last_reason"`
	LastError  string    `json:"last_error,omitempty"`

	// NotifiedFor identifies the situation already reported: the profile plus
	// the expiry that prompted it. A new expiry is a new situation and is
	// allowed to notify again.
	NotifiedFor string `json:"notified_for,omitempty"`

	// Blocked names what the daemon cannot do on its own, empty when nothing
	// is wrong. The shell prompt reads it, so the reminder keeps showing where
	// the user is about to use AWS, long after a notification has gone.
	Blocked BlockedReason `json:"blocked,omitempty"`
	// BlockedProfile is the profile Blocked refers to, so a prompt showing a
	// different profile does not inherit someone else's warning.
	BlockedProfile string `json:"blocked_profile,omitempty"`
}

// BlockedReason is why an unattended renewal cannot proceed.
type BlockedReason string

const (
	// BlockedMFA means a code has to be typed.
	BlockedMFA BlockedReason = "mfa"
	// BlockedSSO means the SSO session needs a browser login.
	BlockedSSO BlockedReason = "sso"
	// BlockedOther covers a profile awsm cannot renew at all.
	BlockedOther BlockedReason = "other"
)

// Short renders the reason for a shell prompt, where every character costs.
func (b BlockedReason) Short() string {
	switch b {
	case BlockedMFA:
		return "MFA"
	case BlockedSSO:
		return "SSO"
	case BlockedOther:
		return "!"
	default:
		return ""
	}
}

// BlockedFor reports what is blocking the given profile, if anything.
//
// Reading the daemon's own state rather than recomputing keeps the prompt as
// cheap as it promises to be, and guarantees it says the same thing the
// notification did.
func BlockedFor(profile string) (BlockedReason, bool) {
	s := LoadState()
	if s.Blocked == "" || s.BlockedProfile != profile {
		return "", false
	}
	return s.Blocked, true
}

// Dir is where the daemon keeps its own files.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".awsm"), nil
}

func statePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon-state.json"), nil
}

// LoadState reads the stored state, returning an empty one when there is none.
// A corrupt file is treated as absent: losing this costs one duplicate
// notification, which is not worth failing a cycle over.
func LoadState() State {
	path, err := statePath()
	if err != nil {
		return State{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}
	}
	return s
}

// SaveState writes the state, best effort.
func SaveState(s State) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// NotificationKey identifies one situation worth notifying about exactly once.
func NotificationKey(profile string, expiry time.Time) string {
	return fmt.Sprintf("%s@%d", profile, expiry.Unix())
}
