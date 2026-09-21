// Package daemon renews the credentials of the active AWS profile on a
// schedule, without keeping a process resident.
//
// The work of one cycle is a single short-lived run: the operating system's
// own scheduler (launchd, a systemd user timer, Task Scheduler) invokes awsm,
// it decides what to do and exits. That costs nothing when idle and cannot
// drift into a bad state, which a sleeping process would have to be supervised
// against.
package daemon

import (
	"fmt"
	"time"
)

// CredentialsThreshold is how much remaining life triggers a renewal.
//
// It is not a safety margin, it is a retry budget: with checks every interval
// I and a threshold T, renewal happens while between T-I and T remain, so
// everything below T is time available to recover from a transient failure. At
// ten minutes and a one-minute interval that is at least nine further attempts
// before anything expires. A threshold close to the interval would leave a
// single attempt, and one network blip would be enough to expire.
const CredentialsThreshold = 10 * time.Minute

// SSOTokenThreshold is the same idea for the SSO access token, which lives for
// hours rather than an hour and so deserves a proportionally larger budget.
const SSOTokenThreshold = 30 * time.Minute

// Action is what a cycle decided to do.
type Action int

const (
	// ActionNone means the credentials are healthy. This is the overwhelming
	// majority of cycles and must cost nothing beyond reading a few files.
	ActionNone Action = iota
	// ActionRefresh renews the active credentials.
	ActionRefresh
	// ActionRefreshSSOToken renews the SSO token as well, keeping the session
	// alive so the browser is not needed later.
	ActionRefreshSSOToken
	// ActionNotifyMFA reports that a renewal needs an MFA code.
	ActionNotifyMFA
	// ActionUnsupported reports a profile awsm cannot renew at all.
	ActionUnsupported
)

func (a Action) String() string {
	switch a {
	case ActionNone:
		return "none"
	case ActionRefresh:
		return "refresh"
	case ActionRefreshSSOToken:
		return "refresh-sso-token"
	case ActionNotifyMFA:
		return "notify-mfa"
	case ActionUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// Snapshot is everything a cycle reads before deciding, all of it obtainable
// from local files. Keeping the decision a function of this struct is what
// makes it testable at arbitrary clock positions, expired ones included.
type Snapshot struct {
	Now time.Time

	// Profile is the active profile, empty when no profile is active.
	Profile string
	// CredentialsRevision identifies the file read before the renewal started.
	CredentialsRevision string
	Kind                KindInfo

	// CredentialsExpiry is when the active credentials run out.
	// HaveCredentials is false when nothing is cached for the profile.
	CredentialsExpiry time.Time
	HaveCredentials   bool

	// MFASessionValid reports a cached MFA session that removes the need for a
	// code.
	MFASessionValid bool

	// SSOTokenExpiry is when the SSO access token runs out.
	SSOTokenExpiry time.Time
	HaveSSOToken   bool
}

// KindInfo mirrors aws.ProfileKind without importing it, so this package stays
// free of AWS dependencies and the decision can be tested on its own.
type KindInfo string

const (
	KindStatic  KindInfo = "static"
	KindRole    KindInfo = "role"
	KindRoleMFA KindInfo = "role-mfa"
	KindSSO     KindInfo = "sso"
	KindProcess KindInfo = "credential-process"
	KindUnknown KindInfo = "unknown"
)

// Decision is an action together with the sentence explaining it, which goes
// straight into the log and, where relevant, the notification.
type Decision struct {
	Action Action
	Reason string
}

// Decide chooses what a cycle should do.
//
// Note that expiry is compared with "less than the threshold" rather than a
// window, so already-expired credentials -- the normal state after a laptop
// has been asleep -- are renewed rather than ignored for being past the point
// where a renewal was due.
func Decide(s Snapshot) Decision {
	if s.Profile == "" {
		return Decision{ActionNone, "no active profile"}
	}

	switch s.Kind {
	case KindStatic:
		return Decision{ActionNone, "static credentials do not expire"}
	case KindUnknown:
		return Decision{ActionUnsupported, fmt.Sprintf("cannot determine the type of profile %q", s.Profile)}
	}

	// Keeping the SSO session alive takes priority: letting it lapse is what
	// forces a browser login later, and renewing it also covers the
	// credentials derived from it.
	if s.Kind == KindSSO && s.HaveSSOToken && s.SSOTokenExpiry.Sub(s.Now) < SSOTokenThreshold {
		return Decision{ActionRefreshSSOToken, fmt.Sprintf(
			"SSO token for %s %s", s.Profile, expiryPhrase(s.SSOTokenExpiry.Sub(s.Now)))}
	}

	if !s.HaveCredentials {
		return Decision{ActionNone, fmt.Sprintf("no cached credentials for %s", s.Profile)}
	}

	remaining := s.CredentialsExpiry.Sub(s.Now)
	if remaining >= CredentialsThreshold {
		return Decision{ActionNone, fmt.Sprintf("credentials for %s %s", s.Profile, expiryPhrase(remaining))}
	}

	if s.Kind == KindRoleMFA && !s.MFASessionValid {
		return Decision{ActionNotifyMFA, fmt.Sprintf(
			"credentials for %s %s and need an MFA code", s.Profile, expiryPhrase(remaining))}
	}

	return Decision{ActionRefresh, fmt.Sprintf(
		"credentials for %s %s", s.Profile, expiryPhrase(remaining))}
}

// expiryPhrase renders how long is left as a clause that can follow a subject:
// "credentials for work " + expiryPhrase(d).
//
// The verb changes with the sign, because "expires 50 days ago" is not
// something anyone would write, and these strings are read by the user in
// notifications rather than only in a log.
func expiryPhrase(d time.Duration) string {
	if d < 0 {
		return fmt.Sprintf("expired %s ago", humanizeSpan(-d))
	}
	return fmt.Sprintf("expire in %s", humanizeSpan(d))
}

// humanizeSpan renders a duration at a precision that suits its size. A token
// that lapsed seven weeks ago was being reported as "1196h6m0s", which is
// accurate and unreadable.
func humanizeSpan(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d >= 2*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return d.Round(time.Minute).String()
	default:
		return d.Round(time.Second).String()
	}
}
