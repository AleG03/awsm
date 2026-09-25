package daemon

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// The bug: a warning raised when the daemon needed a person was only taken down
// by the daemon succeeding at something itself. Signing in from the panel, or
// from a terminal, fixed the account and left "blocked" in the state file --
// which is what both the panel and the shell prompt read. The person who had
// just signed in was told the session was still expired.
func TestABlockIsClearedOnceTheSessionIsBackGoodAgain(t *testing.T) {
	// clearResolvedBlock logs, and the log lives under $HOME: without this the
	// test writes lines about a profile called "work" into the real daemon log
	// of whoever runs it.
	t.Setenv("HOME", t.TempDir())

	now := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	state := State{
		Blocked:        BlockedSSO,
		BlockedProfile: "work",
		NotifiedFor:    "work@123",
	}
	// What the snapshot looks like after somebody signed in: a token that is
	// present and has not run out.
	snapshot := Snapshot{
		Now:            now,
		Profile:        "work",
		Kind:           KindSSO,
		HaveSSOToken:   true,
		SSOTokenExpiry: now.Add(57 * time.Minute),
	}

	clearResolvedBlock(&state, snapshot)

	if state.Blocked != "" {
		t.Errorf("Blocked = %q, want it cleared: the panel and the prompt both read this field", state.Blocked)
	}
	if state.BlockedProfile != "" {
		t.Errorf("BlockedProfile = %q, want it cleared alongside", state.BlockedProfile)
	}
	if state.NotifiedFor != "" {
		t.Error("NotifiedFor kept: the next real block would be swallowed as already reported")
	}
}

func TestABlockSurvivesWhatDoesNotResolveIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	now := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

	cases := map[string]struct {
		state    State
		snapshot Snapshot
	}{
		"no token at all": {
			State{Blocked: BlockedSSO, BlockedProfile: "work"},
			Snapshot{Now: now, Profile: "work", Kind: KindSSO, HaveSSOToken: false},
		},
		"token present but run out": {
			State{Blocked: BlockedSSO, BlockedProfile: "work"},
			Snapshot{Now: now, Profile: "work", Kind: KindSSO,
				HaveSSOToken: true, SSOTokenExpiry: now.Add(-time.Minute)},
		},
		"another profile is active": {
			State{Blocked: BlockedSSO, BlockedProfile: "work"},
			Snapshot{Now: now, Profile: "other", Kind: KindSSO,
				HaveSSOToken: true, SSOTokenExpiry: now.Add(time.Hour)},
		},
		"MFA session still absent": {
			State{Blocked: BlockedMFA, BlockedProfile: "work"},
			Snapshot{Now: now, Profile: "work", Kind: KindRoleMFA, MFASessionValid: false},
		},
		// Not a situation that passes: the profile cannot be renewed at all,
		// which is a fact about its configuration.
		"unrenewable profile": {
			State{Blocked: BlockedOther, BlockedProfile: "work"},
			Snapshot{Now: now, Profile: "work", Kind: KindSSO,
				HaveSSOToken: true, SSOTokenExpiry: now.Add(time.Hour)},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			state := c.state
			clearResolvedBlock(&state, c.snapshot)
			if state.Blocked == "" {
				t.Error("the warning was dropped while its cause was still there")
			}
		})
	}
}

func TestAnMFABlockIsClearedByAValidSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	now := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	state := State{Blocked: BlockedMFA, BlockedProfile: "work", NotifiedFor: "work@1"}
	snapshot := Snapshot{Now: now, Profile: "work", Kind: KindRoleMFA, MFASessionValid: true}

	clearResolvedBlock(&state, snapshot)

	if state.Blocked != "" {
		t.Errorf("Blocked = %q, want it cleared once a code has been typed", state.Blocked)
	}
}

// TestTickClearsAResolvedBlock drives Tick itself, over real files under a
// temporary home.
//
// The function above being right is not the property that matters: the same
// shape of bug -- correct logic nothing calls -- is what let the stale warning
// survive in the first place. This fails if Tick stops consulting it.
func TestTickClearsAResolvedBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	const profile = "work"
	const session = "corp"
	const startURL = "https://example.awsapps.com/start"

	if err := writeFile(filepath.Join(home, ".aws", "config"), `[sso-session `+session+`]
sso_start_url = `+startURL+`
sso_region = eu-west-1
sso_registration_scopes = sso:account:access

[profile `+profile+`]
sso_session = `+session+`
sso_account_id = 123456789012
sso_role_name = Admin
region = eu-west-1
`); err != nil {
		t.Fatal(err)
	}

	// The active profile is recorded as a comment in [default], which is where
	// GetCurrentProfileName looks.
	if err := writeFile(filepath.Join(home, ".aws", "credentials"), `[default]
# source_profile = `+profile+`
region = eu-west-1
`); err != nil {
		t.Fatal(err)
	}

	// A token that is present and has plenty of life: somebody signed in.
	// Named as the AWS CLI names it, since that is what SSOTokenExpiry finds.
	digest := sha1.Sum([]byte(session))
	token := map[string]any{
		"startUrl":    startURL,
		"accessToken": "fake-token",
		"region":      "eu-west-1",
		"expiresAt":   time.Now().UTC().Add(50 * time.Minute).Format("2006-01-02T15:04:05Z"),
	}
	encoded, err := json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(home, ".aws", "sso", "cache",
		hex.EncodeToString(digest[:])+".json"), string(encoded)); err != nil {
		t.Fatal(err)
	}

	// And the warning the daemon left behind before that sign-in.
	if err := SaveState(State{
		Blocked:        BlockedSSO,
		BlockedProfile: profile,
		NotifiedFor:    profile + "@1",
	}); err != nil {
		t.Fatal(err)
	}

	if blocked, ok := BlockedFor(profile); ok {
		t.Fatalf("status still reports %s before the next daemon tick", blocked)
	}
	if LoadState().Blocked != BlockedSSO {
		t.Fatal("read-only status modified daemon state")
	}

	Tick(Options{})

	after := LoadState()
	if after.Blocked != "" {
		t.Errorf("Blocked = %q after a tick, want it cleared.\n"+
			"The panel and the shell prompt read this field, so a stale value tells somebody "+
			"who has just signed in that their session is still expired.", after.Blocked)
	}
	if after.BlockedProfile != "" {
		t.Errorf("BlockedProfile = %q, want it cleared alongside", after.BlockedProfile)
	}
}
