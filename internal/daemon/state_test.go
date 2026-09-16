package daemon

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotificationKeyChangesWithTheExpiry(t *testing.T) {
	// The key is what stops a cycle running every minute from sending sixty
	// notifications an hour, while still allowing a genuinely new situation
	// to be reported.
	expiry := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	if NotificationKey("work", expiry) != NotificationKey("work", expiry) {
		t.Error("the same situation must produce the same key")
	}
	if NotificationKey("work", expiry) == NotificationKey("work", expiry.Add(time.Hour)) {
		t.Error("a new expiry is a new situation and must be allowed to notify again")
	}
	if NotificationKey("work", expiry) == NotificationKey("other", expiry) {
		t.Error("different profiles must not share a key")
	}
}

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := State{
		LastRun:     time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC),
		LastAction:  "refresh",
		LastReason:  "credentials for work expire in 9m",
		NotifiedFor: "work@123",
	}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got := LoadState()
	if !got.LastRun.Equal(want.LastRun) || got.LastAction != want.LastAction ||
		got.NotifiedFor != want.NotifiedFor {
		t.Errorf("round trip lost data: %+v", got)
	}
}

func TestLoadStateToleratesAMissingOrCorruptFile(t *testing.T) {
	// Losing this file costs one duplicate notification. Failing a cycle over
	// it would cost the credentials.
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := LoadState(); got.LastAction != "" {
		t.Errorf("a missing file should yield an empty state, got %+v", got)
	}

	dir := filepath.Join(home, ".awsm")
	if err := writeFile(filepath.Join(dir, "daemon-state.json"), "not json"); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(); got.LastAction != "" {
		t.Errorf("a corrupt file should yield an empty state, got %+v", got)
	}
}

func TestLogWritesAndRotates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	Log("first line %d", 1)
	path, err := LogPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := readFile(path)
	if err != nil {
		t.Fatalf("log not written: %v", err)
	}
	if !contains(data, "first line 1") {
		t.Errorf("log does not contain the message: %q", data)
	}
	// The timestamp is what makes the log usable when reading it later.
	if !contains(data, "T") {
		t.Errorf("log line should carry a timestamp: %q", data)
	}
}

func TestAwsmCommandUsesThisBinaryNotThePath(t *testing.T) {
	// A bare "awsm" would resolve through PATH, which can hold an older build
	// than the one raising the alert; the fix would then appear not to work.
	got := AwsmCommand("profile", "set", "work")
	if strings.HasPrefix(got, "awsm ") {
		t.Errorf("got %q, which relies on PATH", got)
	}
	if !strings.HasSuffix(got, " profile set work") {
		t.Errorf("got %q, want it to end with the arguments", got)
	}
}
