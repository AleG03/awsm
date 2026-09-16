package daemon

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

func base(kind KindInfo, ttl time.Duration) Snapshot {
	return Snapshot{
		Now:               now,
		Profile:           "work",
		Kind:              kind,
		CredentialsExpiry: now.Add(ttl),
		HaveCredentials:   true,
	}
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name string
		in   Snapshot
		want Action
	}{
		{
			name: "no active profile",
			in:   Snapshot{Now: now},
			want: ActionNone,
		},
		{
			name: "static credentials never need renewing",
			in:   base(KindStatic, -time.Hour),
			want: ActionNone,
		},
		{
			name: "plenty of life left",
			in:   base(KindRole, 55*time.Minute),
			want: ActionNone,
		},
		{
			name: "just above the threshold",
			in:   base(KindRole, CredentialsThreshold+time.Second),
			want: ActionNone,
		},
		{
			name: "at the threshold",
			in:   base(KindRole, CredentialsThreshold),
			want: ActionNone,
		},
		{
			name: "just below the threshold",
			in:   base(KindRole, CredentialsThreshold-time.Second),
			want: ActionRefresh,
		},
		{
			name: "already expired, which is the normal state after sleep",
			in:   base(KindRole, -3*time.Hour),
			want: ActionRefresh,
		},
		{
			name: "MFA profile with a live session renews unattended",
			in: func() Snapshot {
				s := base(KindRoleMFA, time.Minute)
				s.MFASessionValid = true
				return s
			}(),
			want: ActionRefresh,
		},
		{
			name: "MFA profile without a session needs a human",
			in:   base(KindRoleMFA, time.Minute),
			want: ActionNotifyMFA,
		},
		{
			name: "MFA profile far from expiry is left alone",
			in:   base(KindRoleMFA, time.Hour),
			want: ActionNone,
		},
		{
			name: "unknown profile is reported, not retried",
			in:   base(KindUnknown, time.Minute),
			want: ActionUnsupported,
		},
		{
			name: "nothing cached yet means nothing to renew",
			in: func() Snapshot {
				s := base(KindRole, 0)
				s.HaveCredentials = false
				return s
			}(),
			want: ActionNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(tc.in)
			if got.Action != tc.want {
				t.Errorf("Decide() = %v (%s), want %v", got.Action, got.Reason, tc.want)
			}
			if got.Reason == "" {
				t.Error("every decision must carry a reason: it goes into the log")
			}
		})
	}
}

func TestDecideKeepsTheSSOSessionAlive(t *testing.T) {
	// The SSO token is renewed ahead of the credentials derived from it,
	// because letting it lapse is what forces a browser login later.
	s := base(KindSSO, 55*time.Minute)
	s.HaveSSOToken = true
	s.SSOTokenExpiry = now.Add(SSOTokenThreshold - time.Minute)

	got := Decide(s)
	if got.Action != ActionRefreshSSOToken {
		t.Fatalf("Decide() = %v (%s), want %v", got.Action, got.Reason, ActionRefreshSSOToken)
	}
	if !strings.Contains(got.Reason, "SSO token") {
		t.Errorf("reason should name the SSO token, got %q", got.Reason)
	}
}

func TestDecideLeavesAHealthySSOTokenAlone(t *testing.T) {
	s := base(KindSSO, 55*time.Minute)
	s.HaveSSOToken = true
	s.SSOTokenExpiry = now.Add(6 * time.Hour)

	if got := Decide(s); got.Action != ActionNone {
		t.Errorf("Decide() = %v (%s), want none", got.Action, got.Reason)
	}
}

func TestDecideRenewsSSOCredentialsEvenWithAHealthyToken(t *testing.T) {
	// The token lasts hours, the credentials an hour: the shorter one still
	// has to be renewed on its own schedule.
	s := base(KindSSO, time.Minute)
	s.HaveSSOToken = true
	s.SSOTokenExpiry = now.Add(6 * time.Hour)

	if got := Decide(s); got.Action != ActionRefresh {
		t.Errorf("Decide() = %v (%s), want refresh", got.Action, got.Reason)
	}
}

func TestThresholdLeavesRoomForRetries(t *testing.T) {
	// The property the threshold exists for: with checks every interval, a
	// renewal is decided while at least (threshold - interval) remains, and
	// that remainder is how many further attempts are possible before the
	// credentials actually expire.
	const interval = DefaultInterval
	budget := CredentialsThreshold - interval
	attempts := int(budget / interval)

	if attempts < 5 {
		t.Errorf("threshold %s with interval %s leaves only %d retries; one failed call would be enough to expire",
			CredentialsThreshold, interval, attempts)
	}
	if interval >= CredentialsThreshold {
		t.Fatalf("interval %s must be shorter than the threshold %s", interval, CredentialsThreshold)
	}
}

func TestReasonsReadAsSentences(t *testing.T) {
	// The reason ends up in the log and in notifications, so it has to read
	// like English rather than like concatenated fragments.
	for _, s := range []Snapshot{base(KindRole, 55*time.Minute), base(KindRole, time.Minute)} {
		reason := Decide(s).Reason
		if strings.Contains(reason, "for in ") || strings.Contains(reason, "  ") {
			t.Errorf("malformed reason: %q", reason)
		}
	}
}

func TestHumanizeReadsCorrectlyOnBothSidesOfNow(t *testing.T) {
	if got := humanize(90 * time.Second); got != "in 2m0s" {
		t.Errorf("humanize(90s) = %q", got)
	}
	if got := humanize(-90 * time.Second); got != "2m0s ago" {
		t.Errorf("humanize(-90s) = %q", got)
	}
}

func TestCredentialsNearExpiry(t *testing.T) {
	// Renewing the SSO token does not invalidate credentials already issued,
	// so a cycle that rotates the token only renews credentials that are
	// themselves due. Otherwise every token rotation would drag an
	// unnecessary STS call along with it.
	if credentialsNearExpiry(base(KindSSO, 50*time.Minute)) {
		t.Error("credentials with 50 minutes left are not due for renewal")
	}
	if !credentialsNearExpiry(base(KindSSO, time.Minute)) {
		t.Error("credentials with a minute left are due")
	}
	if !credentialsNearExpiry(base(KindSSO, -time.Hour)) {
		t.Error("expired credentials are due")
	}

	none := base(KindSSO, time.Minute)
	none.HaveCredentials = false
	if credentialsNearExpiry(none) {
		t.Error("credentials that do not exist cannot be due")
	}
}
