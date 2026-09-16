package daemon

import (
	"errors"
	"fmt"
	"time"

	"awsm/internal/aws"
	"awsm/internal/util"
)

// Options configures one cycle.
type Options struct {
	// AutoLogin opens the SSO login flow when the session can no longer be
	// refreshed. Off by default: a scheduled task that opens a browser window
	// on its own can do it mid-presentation or at three in the morning.
	AutoLogin bool
}

// Tick runs one cycle and returns what it decided.
//
// It declares itself unattended first, so that any path which would have asked
// for an MFA code fails immediately and explicably instead of hanging on a
// stdin nobody is attached to.
func Tick(opts Options) Decision {
	util.SetNonInteractive(true)

	snapshot := takeSnapshot(time.Now())
	decision := Decide(snapshot)

	state := LoadState()
	state.LastRun = snapshot.Now
	state.LastAction = decision.Action.String()
	state.LastReason = decision.Reason
	state.LastError = ""

	switch decision.Action {
	case ActionNone:
		// Not logged: a quiet cycle every minute would bury the ones that
		// matter, and `awsm daemon status` reports the same thing on demand.

	case ActionRefresh, ActionRefreshSSOToken:
		Log("%s: %s", decision.Action, decision.Reason)
		// Rotating the SSO token is an explicit step: resolving credentials
		// does not do it, because the SDK only refreshes a token that has
		// already expired.
		if decision.Action == ActionRefreshSSOToken {
			if err := aws.RefreshSSOToken(snapshot.Profile); err != nil {
				Log("SSO token rotation failed: %v", err)
				state.LastError = err.Error()
				// Only a dead refresh token needs a person. Anything else is
				// worth retrying on the next cycle rather than announcing.
				if errors.Is(err, aws.ErrSsoSessionExpired) {
					handleExpiredSSO(snapshot, opts, &state)
				}
				break
			}
			Log("rotated the SSO token for %s", snapshot.Profile)
			state.NotifiedFor = ""

			// Rotating the token does not invalidate credentials already
			// issued, so renewing them now would be an STS call for nothing.
			// The SSO token lives an hour and the credentials do too, on
			// offset schedules, so this is a real saving rather than a
			// theoretical one.
			if !credentialsNearExpiry(snapshot) {
				break
			}
		}
		if err := refresh(snapshot, opts, &state); err != nil {
			state.LastError = err.Error()
			Log("refresh failed: %v", err)
		} else {
			Log("renewed credentials for %s", snapshot.Profile)
			// A successful renewal makes any earlier warning obsolete.
			state.NotifiedFor = ""
		}

	case ActionNotifyMFA:
		notifyOnce(&state, snapshot, "AWS credentials need MFA",
			fmt.Sprintf("Profile %s cannot be renewed unattended. Run: awsm profile set %s",
				snapshot.Profile, snapshot.Profile))

	case ActionUnsupported:
		notifyOnce(&state, snapshot, "AWS credentials not renewable", decision.Reason)
	}

	if err := SaveState(state); err != nil {
		Log("could not save state: %v", err)
	}
	return decision
}

// credentialsNearExpiry reports whether the active credentials are themselves due
// for renewal, independently of the SSO token's schedule.
func credentialsNearExpiry(s Snapshot) bool {
	return s.HaveCredentials && s.CredentialsExpiry.Sub(s.Now) < CredentialsThreshold
}

// takeSnapshot reads everything the decision needs, all of it from local files.
func takeSnapshot(now time.Time) Snapshot {
	s := Snapshot{Now: now, Profile: aws.GetCurrentProfileName(), Kind: KindUnknown}
	if s.Profile == "" {
		return s
	}

	kind, err := aws.ClassifyProfile(s.Profile)
	if err == nil {
		s.Kind = KindInfo(kind)
	}

	if expiry, ok := aws.CachedCredentialsExpiry(s.Profile); ok {
		s.CredentialsExpiry, s.HaveCredentials = expiry, true
	}

	if s.Kind == KindRoleMFA {
		if _, sessionProfile := aws.MFASerialForProfile(s.Profile); sessionProfile != "" {
			s.MFASessionValid = aws.HasValidMFASession(sessionProfile)
		}
	}

	if s.Kind == KindSSO {
		if expiry, ok := aws.SSOTokenExpiry(s.Profile); ok {
			s.SSOTokenExpiry, s.HaveSSOToken = expiry, true
		}
	}
	return s
}

// refresh renews the active credentials and writes them to the default profile.
//
// The cache is bypassed on purpose: it is considered good until a minute before
// expiry, so the ordinary path would hand back the very credentials this is
// trying to replace and the cycle would silently do nothing.
func refresh(s Snapshot, opts Options, state *State) error {
	creds, isStatic, err := aws.GetFreshCredentialsForProfile(s.Profile)
	if err != nil {
		if errors.Is(err, aws.ErrSsoSessionExpired) {
			handleExpiredSSO(s, opts, state)
			return fmt.Errorf("SSO session for %s needs a new login", s.Profile)
		}
		if errors.Is(err, util.ErrNonInteractive) {
			notifyOnce(state, s, "AWS credentials need MFA",
				fmt.Sprintf("Profile %s needs an MFA code. Run: awsm profile set %s",
					s.Profile, s.Profile))
			return fmt.Errorf("profile %s needs an MFA code", s.Profile)
		}
		return err
	}
	if isStatic {
		return nil
	}

	region, err := aws.GetProfileRegion(s.Profile)
	if err != nil {
		region = ""
	}
	return aws.UpdateCredentialsFile(creds, region, s.Profile)
}

// handleExpiredSSO deals with the one case that genuinely needs a browser.
func handleExpiredSSO(s Snapshot, opts Options, state *State) {
	session, err := aws.GetSsoSessionForProfile(s.Profile)
	if err != nil {
		session = ""
	}

	if opts.AutoLogin && session != "" {
		Log("SSO session %s expired, starting login", session)
		if err := aws.PerformSSOLogin(session); err != nil {
			Log("automatic SSO login failed: %v", err)
		} else {
			state.NotifiedFor = ""
			return
		}
	}

	command := "awsm sso login"
	if session != "" {
		command = fmt.Sprintf("aws sso login --sso-session %s", session)
	}
	notifyOnce(state, s, "AWS SSO session expired",
		fmt.Sprintf("Profile %s needs a new login. Run: %s", s.Profile, command))
}

// notifyOnce reports a situation the user has to resolve, and remembers it so
// the next cycle a minute later stays quiet about the same thing.
func notifyOnce(state *State, s Snapshot, title, message string) {
	key := NotificationKey(s.Profile, s.CredentialsExpiry)
	if state.NotifiedFor == key {
		return
	}
	Log("%s: %s", title, message)
	Notify(title, message)
	state.NotifiedFor = key
}
