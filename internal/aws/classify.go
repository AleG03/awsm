package aws

// ProfileKind is what a profile needs in order to produce credentials.
//
// It exists because an unattended caller has to know, before touching the
// network, whether refreshing a profile is even possible: two of these kinds
// cannot be refreshed without a human.
type ProfileKind string

const (
	// KindStatic is long-term IAM user keys. Nothing expires, nothing to do.
	KindStatic ProfileKind = "static"
	// KindRole assumes a role from credentials that need no interaction.
	KindRole ProfileKind = "role"
	// KindRoleMFA assumes a role and needs an MFA code, unless a cached MFA
	// session covers it.
	KindRoleMFA ProfileKind = "role-mfa"
	// KindSSO resolves through IAM Identity Center. Refreshable while the SSO
	// token can still be refreshed.
	KindSSO ProfileKind = "sso"
	// KindProcess delegates to an external credential_process command.
	KindProcess ProfileKind = "credential-process"
	// KindUnknown is a profile awsm cannot categorise.
	KindUnknown ProfileKind = "unknown"
)

// Refreshable reports whether credentials of this kind can be renewed without
// a person present. KindRoleMFA depends on a cached MFA session, so it is
// answered by the caller rather than here.
func (k ProfileKind) Refreshable() bool {
	return k == KindRole || k == KindSSO || k == KindProcess
}

// ClassifyProfile determines a profile's kind without making any network call.
//
// Reading the config is all it takes, which is what allows a scheduled run to
// skip profiles it could not handle instead of discovering that half way
// through an API call, or worse, at a prompt with no terminal attached.
func ClassifyProfile(profileName string) (ProfileKind, error) {
	pConfig, profileType, err := inspectProfile(profileName)
	if err != nil {
		return KindUnknown, err
	}

	switch profileType {
	case "iam":
		if pConfig.MfaSerial != "" {
			return KindRoleMFA, nil
		}
		return KindRole, nil
	case "sso":
		return KindSSO, nil
	case "credential-process":
		return KindProcess, nil
	case "iam-user", "static":
		return KindStatic, nil
	default:
		return KindUnknown, nil
	}
}

// MFASerialForProfile returns the profile's mfa_serial, and the profile whose
// credentials an MFA session would be cached under.
//
// The session belongs to the source profile, not to the role profile, because
// that is whose long-term keys sts:GetSessionToken is called with.
func MFASerialForProfile(profileName string) (serial, sessionProfile string) {
	pConfig, _, err := inspectProfile(profileName)
	if err != nil || pConfig == nil {
		return "", ""
	}
	sessionProfile = profileName
	if pConfig.SourceProfile != "" {
		sessionProfile = pConfig.SourceProfile
	}
	return pConfig.MfaSerial, sessionProfile
}
