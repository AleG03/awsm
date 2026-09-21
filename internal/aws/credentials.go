package aws

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"awsm/internal/awsini"
	"awsm/internal/filelock"
	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	oidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

// ErrSsoSessionExpired indicates SSO session has expired
var ErrSsoSessionExpired = errors.New("sso session is expired or invalid")

// TempCredentials holds a set of temporary AWS credentials.
type TempCredentials struct {
	AccessKeyId     string    `json:"access_key_id"`
	SecretAccessKey string    `json:"secret_access_key"`
	SessionToken    string    `json:"session_token"`
	Expires         time.Time `json:"expires"`
}

// credsCachePath returns the path for a profile's cached credentials.
func credsCachePath(profileName string) (string, error) {
	return credentialCachePath("roles", profileName)
}

func getCachedCreds(profileName string) *TempCredentials {
	path, err := credsCachePath(profileName)
	if err != nil {
		return nil
	}
	creds := readCredentialCache(path, profileName)
	if creds == nil || time.Until(creds.Expires) < time.Minute {
		return nil
	}
	return creds
}

func setCachedCreds(profileName string, creds *TempCredentials) {
	path, err := credsCachePath(profileName)
	if err == nil {
		writeCredentialCache(path, profileName, creds)
	}
}

// HasValidCachedCredentials checks if valid cached credentials exist for a profile.
func HasValidCachedCredentials(profileName string) bool {
	return getCachedCreds(profileName) != nil
}

// CachedCredentialsExpiry returns the expiration time of the cached credentials
// for a profile, if any. The second return value is false when no cache file
// exists or when it cannot be parsed.
//
// Unlike HasValidCachedCredentials, this does NOT enforce a minimum TTL — even
// already-expired entries are returned, so callers can render them as such in
// status displays.
func CachedCredentialsExpiry(profileName string) (time.Time, bool) {
	path, err := credsCachePath(profileName)
	if err != nil {
		return time.Time{}, false
	}
	creds := readCredentialCache(path, profileName)
	if creds == nil || creds.Expires.IsZero() {
		return time.Time{}, false
	}
	return creds.Expires, true
}

// profileConfig holds the relevant configuration details extracted from a profile.
type profileConfig struct {
	MfaSerial     string
	RoleArn       string
	SourceProfile string
	// ExternalId, RoleSessionName and DurationSeconds are honoured by the AWS
	// CLI and were previously ignored here: external_id made cross-account
	// roles that require it impossible to assume, and duration_seconds was
	// overridden by a hardcoded hour.
	ExternalId      string
	RoleSessionName string
	DurationSeconds int32
	// CredentialSource is read only to reject it explicitly. It replaces
	// source_profile with credentials taken from EC2/ECS/the environment,
	// which assumeRole below does not implement.
	CredentialSource string
}

// ProfileNeedsMFA checks if a profile requires MFA and returns the MFA serial.
func ProfileNeedsMFA(profileName string) (bool, string, error) {
	pConfig, profileType, err := inspectProfile(profileName)
	if err != nil {
		return false, "", err
	}
	if profileType == "iam" && pConfig.MfaSerial != "" {
		return true, pConfig.MfaSerial, nil
	}
	return false, "", nil
}

// GetCredentialsForProfile is the main entry point for getting credentials.
// It inspects the profile and dispatches to the correct handler.
// If mfaToken is non-empty, it will be used instead of prompting interactively.
func GetCredentialsForProfile(profileName string, mfaToken ...string) (creds *TempCredentials, isStatic bool, err error) {
	return getCredentials(profileName, true, mfaToken...)
}

// GetFreshCredentialsForProfile is GetCredentialsForProfile with the cache
// ignored.
//
// The cache is considered good until a minute before expiry, so anything trying
// to renew credentials ahead of time gets the old ones handed straight back.
// The cache is only replaced once the new credentials are in hand, so a failed
// renewal leaves the still-valid ones alone.
func GetFreshCredentialsForProfile(profileName string, mfaToken ...string) (creds *TempCredentials, isStatic bool, err error) {
	return getCredentials(profileName, false, mfaToken...)
}

func getCredentials(profileName string, useCache bool, mfaToken ...string) (creds *TempCredentials, isStatic bool, err error) {
	fingerprint, err := profileFingerprint(profileName)
	if err != nil {
		return nil, false, err
	}
	pConfig, profileType, err := inspectProfile(profileName)
	if err != nil {
		return nil, false, err
	}

	token := ""
	if len(mfaToken) > 0 {
		token = mfaToken[0]
	}

	switch profileType {
	case "iam":
		// Check credential cache before prompting for MFA
		if useCache {
			if cached := getCachedCreds(profileName); cached != nil {
				return cached, false, nil
			}
		}
		tempCreds, err := handleIamProfile(profileName, pConfig, token)
		if err != nil {
			return nil, false, err
		}
		result := &TempCredentials{
			AccessKeyId:     *tempCreds.AccessKeyId,
			SecretAccessKey: *tempCreds.SecretAccessKey,
			SessionToken:    *tempCreds.SessionToken,
			Expires:         *tempCreds.Expiration,
		}
		current, err := profileFingerprint(profileName)
		if err != nil {
			return nil, false, err
		}
		if current != fingerprint {
			return nil, false, fmt.Errorf("profile %q changed while resolving credentials; retry the command", profileName)
		}
		if path, err := credsCachePath(profileName); err == nil {
			writeCredentialCacheWithFingerprint(path, fingerprint, result)
		}
		return result, false, nil

	case "sso", "credential-process":
		awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profileName))
		if err != nil {
			return nil, false, fmt.Errorf("failed to load AWS config for profile: %w", err)
		}
		sdkCreds, err := awsCfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			if signingInWouldFix(profileType, err) {
				return nil, false, ErrSsoSessionExpired
			}
			return nil, false, err // Return the original error for other issues.
		}
		return &TempCredentials{
			AccessKeyId:     sdkCreds.AccessKeyID,
			SecretAccessKey: sdkCreds.SecretAccessKey,
			SessionToken:    sdkCreds.SessionToken,
			Expires:         sdkCreds.Expires,
		}, false, nil

	case "iam-user", "static":
		awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profileName))
		if err != nil {
			return nil, true, fmt.Errorf("failed to load AWS config for static profile: %w", err)
		}
		sdkCreds, err := awsCfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			return nil, true, fmt.Errorf("failed to retrieve static credentials: %w", err)
		}
		return &TempCredentials{
			AccessKeyId:     sdkCreds.AccessKeyID,
			SecretAccessKey: sdkCreds.SecretAccessKey,
			SessionToken:    sdkCreds.SessionToken,
			Expires:         sdkCreds.Expires,
		}, true, nil

	default:
		return nil, false, fmt.Errorf("unknown profile type for '%s'", profileName)
	}
}

// inspectProfile reads the config file to determine the profile type.
func inspectProfile(profileName string) (*profileConfig, string, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return nil, "", err
	}
	cfgFile, err := awsini.LoadOrEmpty(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read AWS config file: %w", err)
	}

	section, err := getProfileSection(cfgFile, profileName)
	if err != nil {
		// Profile not found in config, check credentials file for IAM user
		credentialsPath, credErr := GetAWSCredentialsPath()
		if credErr != nil {
			return nil, "", fmt.Errorf("could not find profile section for '%s'", profileName)
		}
		credFile, credErr := awsini.Load(credentialsPath)
		if credErr != nil {
			return nil, "", fmt.Errorf("could not find profile section for '%s'", profileName)
		}
		credSection, credErr := credFile.GetSection(profileName)
		if credErr != nil {
			return nil, "", fmt.Errorf("could not find profile section for '%s'", profileName)
		}
		// Check if it has static credentials
		if credSection.HasKey("aws_access_key_id") && credSection.HasKey("aws_secret_access_key") {
			return &profileConfig{}, "iam-user", nil
		}
		return nil, "", fmt.Errorf("could not find profile section for '%s'", profileName)
	}

	pConfig := &profileConfig{
		MfaSerial:        section.Key("mfa_serial").String(),
		RoleArn:          section.Key("role_arn").String(),
		SourceProfile:    section.Key("source_profile").String(),
		ExternalId:       section.Key("external_id").String(),
		RoleSessionName:  section.Key("role_session_name").String(),
		CredentialSource: section.Key("credential_source").String(),
	}

	// A duration we cannot parse is a mistake in the file, not a reason to
	// silently fall back to an hour and leave the user wondering.
	if raw := strings.TrimSpace(section.Key("duration_seconds").String()); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return nil, "", fmt.Errorf("profile '%s' has an invalid duration_seconds %q: want a positive number of seconds", profileName, raw)
		}
		pConfig.DurationSeconds = int32(seconds)
	}

	if pConfig.RoleArn != "" || pConfig.MfaSerial != "" {
		return pConfig, "iam", nil
	}
	if section.HasKey("sso_session") {
		return pConfig, "sso", nil
	}
	if section.HasKey("credential_process") {
		return pConfig, "credential-process", nil
	}
	if section.HasKey("aws_access_key_id") {
		return pConfig, "iam-user", nil
	}

	// Profile found in config but no special keys, check credentials file for static keys
	credentialsPath, credErr := GetAWSCredentialsPath()
	if credErr == nil {
		credFile, credErr := awsini.Load(credentialsPath)
		if credErr == nil {
			credSection, credErr := credFile.GetSection(profileName)
			if credErr == nil && credSection.HasKey("aws_access_key_id") && credSection.HasKey("aws_secret_access_key") {
				return &profileConfig{}, "iam-user", nil
			}
		}
	}

	return nil, "unknown", fmt.Errorf("could not determine type of profile '%s'", profileName)
}

// signingInWouldFix reports whether a failure to resolve credentials is one
// that a fresh SSO login would clear.
//
// An SSO session can be unusable in several ways that look nothing alike to the
// SDK and identical to the person in front of it, and each one that is not
// recognised here becomes a dead end: a raw error, no login offered, and a
// profile that cannot be entered at all. Three of them reached users that way
// before being listed:
//
//   - no cached token, because the session was never signed in to, or the cache
//     was cleared, or this is a new machine;
//   - a token the service rejects, because the session was revoked, signed out
//     elsewhere, or ended by the identity provider's own policy -- its recorded
//     expiry is still in the future, so nothing about it looks expired;
//   - a refresh token no longer accepted, which is the same policy running out
//     between renewals.
//
// The service errors are matched by type. errors.As reaches them through the
// credential provider's wrapping -- verified against a real rejection -- and a
// type cannot be reworded out from under this the way a message can. The string
// tests below are a backstop for the failures the SDK reports as prose rather
// than as a modelled error.
func signingInWouldFix(profileType string, err error) bool {
	// Only for SSO. The same branch serves credential_process profiles, where
	// a file that does not exist is the configured command itself, and signing
	// in to anything would not produce it.
	if profileType == "sso" && errors.Is(err, fs.ErrNotExist) {
		return true
	}

	var unauthorized *ssotypes.UnauthorizedException
	var expiredToken *oidctypes.ExpiredTokenException
	var invalidGrant *oidctypes.InvalidGrantException
	if errors.As(err, &unauthorized) ||
		errors.As(err, &expiredToken) ||
		errors.As(err, &invalidGrant) {
		return true
	}

	// The backstop, for failures the SDK words rather than models. "expired"
	// on its own subsumes the longer phrasings it has used, and matching the
	// exception name covers a rejection that arrives as text rather than as
	// the type above.
	message := err.Error()
	return strings.Contains(message, "expired") ||
		strings.Contains(message, "InvalidGrantException")
}

// handleIamProfile contains the logic for IAM-based profiles (MFA/role assumption).
func handleIamProfile(profileName string, pConfig *profileConfig, mfaToken string) (*types.Credentials, error) {
	if pConfig.RoleArn != "" {
		return assumeRole(profileName, pConfig, mfaToken)
	}
	return getSessionToken(profileName, pConfig, mfaToken)
}

// assumeRole handles the specific logic for calling sts:AssumeRole.
func assumeRole(profileName string, pConfig *profileConfig, mfaToken string) (*types.Credentials, error) {
	util.InfoColor.Fprintf(os.Stderr, "Assuming role %s...\n", util.BoldColor.Sprint(pConfig.RoleArn))

	// Without a source_profile the STS client would be built from this very
	// profile, which is the one holding role_arn: the SDK would resolve it by
	// assuming the role itself and awsm would then assume it a second time.
	// credential_source is the supported way to express that, and awsm does not
	// implement it, so say so rather than producing that double assumption.
	if pConfig.SourceProfile == "" && pConfig.CredentialSource != "" {
		return nil, fmt.Errorf("profile '%s' uses credential_source = %s, which awsm does not support yet; use source_profile, or run the command through the AWS CLI", profileName, pConfig.CredentialSource)
	}

	stsClientProfile := profileName
	if pConfig.SourceProfile != "" {
		stsClientProfile = pConfig.SourceProfile

		// Check if source profile is SSO and ensure it's logged in
		if ssoSession, err := GetSsoSessionForProfile(stsClientProfile); err == nil {
			// It's an SSO profile, check if login is needed
			if needsLogin, checkErr := checkSSOLoginNeeded(stsClientProfile); checkErr != nil {
				// If there's an error checking SSO status, it's likely expired or has connectivity issues
				if strings.Contains(checkErr.Error(), "certificate") || strings.Contains(checkErr.Error(), "SSL") {
					return nil, fmt.Errorf("SSL certificate issue with SSO session for source profile '%s'. Please check your network configuration or run: awsm sso login %s", stsClientProfile, ssoSession)
				}
				return nil, fmt.Errorf("SSO session for source profile '%s' has expired or is invalid. Please run: awsm sso login %s", stsClientProfile, ssoSession)
			} else if needsLogin {
				return nil, fmt.Errorf("SSO session for source profile '%s' has expired. Please run: awsm sso login %s", stsClientProfile, ssoSession)
			}
		}
	}

	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(stsClientProfile))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for source profile '%s': %w", stsClientProfile, err)
	}

	// With MFA, prefer authenticating the AssumeRole call with a cached MFA
	// session over sending a fresh code every time. Those credentials carry
	// aws:MultiFactorAuthPresent, so trust policies that demand MFA are still
	// satisfied, and the code is typed once per session instead of hourly --
	// which is also what makes unattended refresh possible at all.
	var tokenCode *string
	usedMFASession := false
	if pConfig.MfaSerial != "" {
		if canUseMFASession(stsClientProfile) {
			session, err := mfaSessionCredentials(stsClientProfile, pConfig.MfaSerial, mfaToken)
			if err != nil {
				return nil, err
			}
			awsCfg, err = staticCredentialsConfig(session, awsCfg.Region)
			if err != nil {
				return nil, fmt.Errorf("failed to build AWS config from the MFA session: %w", err)
			}
			usedMFASession = true
		} else {
			// The source credentials are already temporary, so GetSessionToken
			// would refuse them: send the code with the AssumeRole call.
			code := mfaToken
			if code == "" {
				prompt := fmt.Sprintf("Enter MFA token for %s: ", util.BoldColor.Sprint(pConfig.MfaSerial))
				var err error
				code, err = util.PromptForInput(prompt)
				if err != nil {
					return nil, fmt.Errorf("failed to read MFA token: %w", err)
				}
			}
			tokenCode = aws.String(code)
		}
	}

	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(pConfig.RoleArn),
		RoleSessionName: aws.String(BuildRoleSessionName(pConfig.RoleSessionName, profileName)),
	}

	// Left unset, STS applies its own default of one hour. Sending a value the
	// user did not ask for is what capped every session at an hour even when
	// the role allowed twelve.
	if pConfig.DurationSeconds > 0 {
		input.DurationSeconds = aws.Int32(pConfig.DurationSeconds)
	}
	if pConfig.ExternalId != "" {
		input.ExternalId = aws.String(pConfig.ExternalId)
	}
	// Sending the code again alongside session credentials that already carry
	// the MFA claim is both redundant and rejected by STS.
	if pConfig.MfaSerial != "" && !usedMFASession {
		input.SerialNumber = aws.String(pConfig.MfaSerial)
		input.TokenCode = tokenCode
	}

	stsClient := sts.NewFromConfig(awsCfg)
	result, err := stsClient.AssumeRole(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}
	return result.Credentials, nil
}

// getSessionToken handles the specific logic for calling sts:GetSessionToken.
func getSessionToken(profileName string, pConfig *profileConfig, mfaToken string) (*types.Credentials, error) {
	util.InfoColor.Fprintf(os.Stderr, "Getting session token for profile %s...\n", util.BoldColor.Sprint(profileName))

	// This is exactly the MFA session, so it goes through the same cache: an
	// hour used to mean retyping the code every hour for no reason.
	session, err := mfaSessionCredentials(profileName, pConfig.MfaSerial, mfaToken)
	if err != nil {
		return nil, err
	}
	return &types.Credentials{
		AccessKeyId:     aws.String(session.AccessKeyId),
		SecretAccessKey: aws.String(session.SecretAccessKey),
		SessionToken:    aws.String(session.SessionToken),
		Expiration:      aws.Time(session.Expires),
	}, nil
}

// GetAWSCredentialsPath returns the path to the AWS credentials file.
func GetAWSCredentialsPath() (string, error) {
	credentialsPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credentialsPath != "" {
		return credentialsPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "credentials"), nil
}

// UpdateCredentialsFile updates the default profile in the AWS credentials file
func UpdateCredentialsFile(creds *TempCredentials, region, profileName string) error {
	return withCredentialsLock(func() error { return updateCredentialsFile(creds, region, profileName) })
}

func updateCredentialsFile(creds *TempCredentials, region, profileName string) error {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	// Create .aws directory if it doesn't exist
	awsDir := filepath.Dir(credentialsPath)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	// Load or create credentials file
	cfg, err := loadOrCreateIni(credentialsPath)
	if err != nil {
		return err
	}

	// Get or create default section
	section, err := cfg.GetSection("default")
	if err != nil {
		section, err = cfg.NewSection("default")
		if err != nil {
			return fmt.Errorf("failed to create default section: %w", err)
		}
	}

	// Update credentials
	section.Key("aws_access_key_id").SetValue(creds.AccessKeyId)
	section.Key("aws_secret_access_key").SetValue(creds.SecretAccessKey)
	section.Key("aws_session_token").SetValue(creds.SessionToken)

	// Update region if provided
	if region != "" {
		section.Key("region").SetValue(region)
	}

	// Track the source profile name
	section.Key("# source_profile").SetValue(profileName)

	// And when what was just written stops working.
	//
	// Nothing else records it. The credentials themselves carry no expiry once
	// they are in this file, and awsm's own cache only ever held the profiles
	// it resolves through STS -- never the SSO ones -- so the refresh daemon
	// had no way to tell that an SSO profile's credentials were running out and
	// left them to lapse.
	//
	// A key whose name begins with '#' is the same trick as the line above:
	// ini.v1 reads it, and the AWS CLI's parser treats the line as a comment
	// and never sees it. Static credentials have no expiry, so the key is
	// removed rather than written empty -- a leftover from the profile before
	// would read as credentials that expired long ago.
	if creds.Expires.IsZero() {
		section.DeleteKey(defaultExpiresKey)
	} else {
		section.Key(defaultExpiresKey).SetValue(creds.Expires.UTC().Format(time.RFC3339))
	}

	// Save the file
	return awsini.Save(cfg, credentialsPath)
}

// defaultExpiresKey records, in the default profile, when the credentials
// written there stop working.
const defaultExpiresKey = "# expires"

// ActiveCredentialsExpiry reports when the credentials in use run out.
//
// "In use" means the ones in the default profile, which is what every tool
// reading ~/.aws/credentials actually gets, and what the refresh daemon exists
// to keep alive. It is only meaningful for the active profile: asked about any
// other, it falls back to what awsm resolved for that one.
//
// The fallback also covers credentials written before this was recorded, so an
// upgrade does not leave the daemon blind until the next switch.
func ActiveCredentialsExpiry(profileName string) (time.Time, bool) {
	if profileName == "" || profileName != GetCurrentProfileName() {
		return CachedCredentialsExpiry(profileName)
	}

	raw := readDefaultComment(defaultExpiresKey)
	if raw == "" {
		return CachedCredentialsExpiry(profileName)
	}
	expiry, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return CachedCredentialsExpiry(profileName)
	}
	return expiry, true
}

// readDefaultComment reads one of the '#'-prefixed keys out of the default
// profile, by hand.
//
// It has to be by hand. ini.v1 writes such a key happily and then, reading the
// same file back, takes the line for a comment and drops it -- which is exactly
// why the AWS CLI never sees it, and exactly why nothing can read it through
// the parser either. GetCurrentProfileName has scanned for '# source_profile'
// this way all along; this is the same scan, parameterised.
func readDefaultComment(key string) string {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return ""
	}
	file, err := os.Open(credentialsPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inDefault := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inDefault = line == "[default]"
			continue
		}
		if !inDefault || !strings.HasPrefix(line, key) {
			continue
		}
		if _, value, found := strings.Cut(line, "="); found {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// GetCurrentProfileName returns the name of the profile currently set in default
func GetCurrentProfileName() string {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return ""
	}

	// Try to load with ini first (faster)
	cfg, err := awsini.Load(credentialsPath)
	if err == nil {
		section, err := cfg.GetSection("default")
		if err == nil {
			if section.HasKey("# source_profile") {
				return section.Key("# source_profile").String()
			}
		}
	}

	// Fallback to manual parsing for edge cases
	file, err := os.Open(credentialsPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inDefaultSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[default]" {
			inDefaultSection = true
			continue
		}

		if strings.HasPrefix(line, "[") && line != "[default]" {
			inDefaultSection = false
			continue
		}

		if inDefaultSection && strings.HasPrefix(line, "# source_profile") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

// UpdateStaticProfile updates the default profile to use a static profile's credentials
func UpdateStaticProfile(profileName string) error {
	return withCredentialsLock(func() error { return updateStaticProfile(profileName) })
}

func updateStaticProfile(profileName string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	// Static keys can live in either file: awsm writes them into the profile in
	// the config, but the credentials file remains valid and is where anything
	// written before, or by "aws configure", still keeps them.
	var region, accessKey, secretKey, sessionToken string
	var haveSessionToken bool

	cfgFile, err := awsini.Load(configPath)
	if err == nil {
		if configSection, err := getProfileSection(cfgFile, profileName); err == nil {
			region = configSection.Key("region").String()
			accessKey = configSection.Key("aws_access_key_id").String()
			secretKey = configSection.Key("aws_secret_access_key").String()
			if configSection.HasKey("aws_session_token") {
				sessionToken = configSection.Key("aws_session_token").String()
				haveSessionToken = true
			}
		}
	}

	// The credentials file has to be loaded regardless: it is where the default
	// profile being written lives.
	credFile, err := awsini.LoadOrEmpty(credentialsPath)
	if err != nil {
		return fmt.Errorf("failed to read AWS credentials file: %w", err)
	}

	if credSection, err := credFile.GetSection(profileName); err == nil {
		if region == "" {
			region = credSection.Key("region").String()
		}
		// The credentials file wins when a profile is defined in both, which is
		// what the AWS CLI itself does.
		if credSection.Key("aws_access_key_id").String() != "" {
			accessKey = credSection.Key("aws_access_key_id").String()
			secretKey = credSection.Key("aws_secret_access_key").String()
			sessionToken = credSection.Key("aws_session_token").String()
			haveSessionToken = credSection.HasKey("aws_session_token")
		}
	}

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("profile '%s' does not have static credentials in %s or %s", profileName, configPath, credentialsPath)
	}

	// Update default section
	defaultSection, err := credFile.GetSection("default")
	if err != nil {
		defaultSection, err = credFile.NewSection("default")
		if err != nil {
			return fmt.Errorf("failed to create default section: %w", err)
		}
	}

	defaultSection.Key("aws_access_key_id").SetValue(accessKey)
	defaultSection.Key("aws_secret_access_key").SetValue(secretKey)

	// A stale token left on the default profile would be sent with the new
	// keys and rejected, so it goes unless the source actually carries one.
	if haveSessionToken && sessionToken != "" {
		defaultSection.Key("aws_session_token").SetValue(sessionToken)
	} else {
		defaultSection.DeleteKey("aws_session_token")
	}

	if region != "" {
		defaultSection.Key("region").SetValue(region)
	}

	// Track the source profile name
	defaultSection.Key("# source_profile").SetValue(profileName)

	return awsini.Save(credFile, credentialsPath)
}

// SetRegion updates the region in the default profile
func SetRegion(region string) error {
	return withCredentialsLock(func() error { return setRegion(region) })
}

func setRegion(region string) error {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	// Get current source profile name to preserve it
	currentSourceProfile := GetCurrentProfileName()

	// And the expiry, for the same reason. Changing a region leaves the
	// credentials exactly as they were, so the record of when they run out has
	// to survive too. It would not survive on its own: ini.v1 drops a
	// '#'-prefixed key when it reads the file, so every key of that kind is
	// lost unless the write puts it back.
	currentExpiry := readDefaultComment(defaultExpiresKey)

	// Create .aws directory if it doesn't exist
	awsDir := filepath.Dir(credentialsPath)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	cfg, err := loadOrCreateIni(credentialsPath)
	if err != nil {
		return err
	}

	// Get or create default section
	section, err := cfg.GetSection("default")
	if err != nil {
		section, err = cfg.NewSection("default")
		if err != nil {
			return fmt.Errorf("failed to create default section: %w", err)
		}
	}

	// Update region
	section.Key("region").SetValue(region)

	// Preserve the source profile comment if it exists
	if currentSourceProfile != "" {
		section.Key("# source_profile").SetValue(currentSourceProfile)
	}
	if currentExpiry != "" {
		section.Key(defaultExpiresKey).SetValue(currentExpiry)
	}

	// Save the file
	return awsini.Save(cfg, credentialsPath)
}

// ClearDefaultProfile removes all credentials and region from the default profile
func ClearDefaultProfile() error {
	return withCredentialsLock(func() error { return clearDefaultProfile() })
}

func clearDefaultProfile() error {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	cfg, err := awsini.Load(credentialsPath)
	if err != nil {
		return fmt.Errorf("failed to load credentials file: %w", err)
	}

	section, err := cfg.GetSection("default")
	if err != nil {
		return nil // No default section exists, nothing to clear
	}

	// Remove all keys from default section
	section.DeleteKey("aws_access_key_id")
	section.DeleteKey("aws_secret_access_key")
	section.DeleteKey("aws_session_token")
	section.DeleteKey("region")
	section.DeleteKey("# source_profile")
	section.DeleteKey(defaultExpiresKey)

	return awsini.Save(cfg, credentialsPath)
}

// checkSSOLoginNeeded checks if an SSO profile needs login
func checkSSOLoginNeeded(profileName string) (bool, error) {
	// Try to get credentials to see if SSO session is valid
	_, _, err := GetCredentialsForProfile(profileName)
	if err != nil && errors.Is(err, ErrSsoSessionExpired) {
		return true, nil
	}
	return false, err
}

// ErrActiveProfileChanged means a renewal lost its snapshot to another writer.
var ErrActiveProfileChanged = errors.New("active credentials changed during renewal")

func withCredentialsLock(fn func() error) error {
	path, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}
	return filelock.With(path, fn)
}

// UpdateCredentialsFileIfCurrent commits a renewal only while its profile and
// file revision still match. Interactive switches and clears use the same lock.
func UpdateCredentialsFileIfCurrent(creds *TempCredentials, region, profileName, revision string) error {
	return withCredentialsLock(func() error {
		current, err := ActiveCredentialsRevision()
		if err != nil {
			return err
		}
		if revision == "" || current != revision || GetCurrentProfileName() != profileName {
			return ErrActiveProfileChanged
		}
		return updateCredentialsFile(creds, region, profileName)
	})
}

// ClearDefaultProfileIfCurrent compares and clears under the same lock used by
// switches and renewals. A caller's earlier status read cannot authorize a clear.
func ClearDefaultProfileIfCurrent(profile string) error {
	return withCredentialsLock(func() error {
		if profile == "" || GetCurrentProfileName() != profile {
			return ErrActiveProfileChanged
		}
		return clearDefaultProfile()
	})
}
