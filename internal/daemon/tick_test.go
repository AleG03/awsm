package daemon

import (
	"testing"
	"time"
)

func TestAlertMFARecordsAReasonThePromptCanRender(t *testing.T) {
	// The mistake this guards against type-checked: BlockedReason is a string,
	// so passing the notification title where the reason belonged compiled, and
	// Short() then fell through to its default. The prompt still decided
	// something was wrong and drew "⚠" with nothing next to it.
	t.Setenv("HOME", t.TempDir())

	s := Snapshot{
		Profile:           "work",
		Kind:              KindRoleMFA,
		CredentialsExpiry: time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC),
		HaveCredentials:   true,
	}
	// Seeded as already notified, so the assertions are about the recorded
	// state and no desktop notification fires during a test run.
	state := State{NotifiedFor: NotificationKey(s.Profile, s.CredentialsExpiry)}

	alertMFA(&state, s)

	if state.Blocked != BlockedMFA {
		t.Errorf("Blocked = %q, want %q", state.Blocked, BlockedMFA)
	}
	if state.Blocked.Short() == "" {
		t.Errorf("Blocked = %q renders as nothing in the prompt", state.Blocked)
	}
	if state.BlockedProfile != s.Profile {
		t.Errorf("BlockedProfile = %q, want %q", state.BlockedProfile, s.Profile)
	}
}

func TestAlertOnceKeepsMarkingTheProfileAfterTheFirstNotification(t *testing.T) {
	// The notification is sent once so it does not become sixty an hour, but
	// the prompt has to keep warning for as long as the situation lasts.
	t.Setenv("HOME", t.TempDir())

	s := Snapshot{Profile: "work", CredentialsExpiry: time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)}
	state := State{NotifiedFor: NotificationKey(s.Profile, s.CredentialsExpiry)}

	state.Blocked = ""
	state.BlockedProfile = ""
	alertMFA(&state, s)

	if state.Blocked != BlockedMFA || state.BlockedProfile != s.Profile {
		t.Errorf("a repeated cycle cleared the prompt warning: %+v", state)
	}
}
