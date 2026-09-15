package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"awsm/internal/awsini"

	"gopkg.in/ini.v1"
)

// OrganizeConfigFile reads the AWS config file, reorganizes it into logical groups,
// and writes it back. The resulting file has:
//  1. [default] section (if present)
//  2. Each SSO session followed by its related profiles (grouped by account)
//  3. IAM role profiles sorted alphabetically
//  4. Static key profiles sorted alphabetically
//  5. Other profiles
func OrganizeConfigFile(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	content := buildOrganizedConfig(cfg)
	return awsini.WriteFile(configPath, []byte(content))
}

// buildOrganizedConfig creates a well-organized config string from an ini file
func buildOrganizedConfig(cfg *ini.File) string {
	var b strings.Builder

	// Collect sections by type
	ssoSessionMap := make(map[string]*ini.Section)          // session name -> section
	ssoProfilesBySession := make(map[string][]*ini.Section) // session name -> profiles
	var iamProfiles []*ini.Section
	var keyProfiles []*ini.Section
	var otherProfiles []*ini.Section

	for _, section := range cfg.Sections() {
		name := section.Name()
		switch {
		case name == "DEFAULT" || name == "default":
			// Write the default section first if it has keys
			if len(section.Keys()) > 0 {
				writeSectionAs(&b, section, "default")
			}

		case strings.HasPrefix(name, "sso-session "):
			sessionName := strings.TrimPrefix(name, "sso-session ")
			ssoSessionMap[sessionName] = section

		case strings.HasPrefix(name, "profile "):
			// Categorize profiles
			if section.HasKey("sso_session") || section.HasKey("sso_start_url") {
				sessionName := section.Key("sso_session").String()
				if sessionName == "" {
					sessionName = "_direct_sso"
				}
				ssoProfilesBySession[sessionName] = append(ssoProfilesBySession[sessionName], section)
			} else if section.HasKey("role_arn") {
				iamProfiles = append(iamProfiles, section)
			} else if section.HasKey("credential_process") {
				otherProfiles = append(otherProfiles, section)
			} else {
				keyProfiles = append(keyProfiles, section)
			}

		default:
			// Bare profile names (without "profile " prefix)
			if name != "DEFAULT" {
				otherProfiles = append(otherProfiles, section)
			}
		}
	}

	// --- SSO: each session block with its profiles ---
	// Collect all session names (from sso-session declarations and from profiles referencing them)
	allSessionNames := make(map[string]bool)
	for name := range ssoSessionMap {
		allSessionNames[name] = true
	}
	for name := range ssoProfilesBySession {
		if name != "_direct_sso" {
			allSessionNames[name] = true
		}
	}

	var sortedSessionNames []string
	for name := range allSessionNames {
		sortedSessionNames = append(sortedSessionNames, name)
	}
	sort.Strings(sortedSessionNames)

	// Also handle profiles with direct SSO (no named session)
	if profiles, ok := ssoProfilesBySession["_direct_sso"]; ok && len(profiles) > 0 {
		sortedSessionNames = append(sortedSessionNames, "_direct_sso")
	}

	for _, sessionName := range sortedSessionNames {
		// Write separator
		if sessionName == "_direct_sso" {
			b.WriteString("# ─── SSO Profiles (direct) ──────────────────────────────────────────────────\n\n")
		} else {
			b.WriteString(fmt.Sprintf("# ─── SSO: %s ────────────────────────────────────────────────────────────────\n\n", sessionName))
		}

		// Write the sso-session section first
		if session, exists := ssoSessionMap[sessionName]; exists {
			writeSection(&b, session)
		}

		// Write profiles grouped by account
		profiles := ssoProfilesBySession[sessionName]
		if len(profiles) > 0 {
			profilesByAccount := make(map[string][]*ini.Section)
			for _, p := range profiles {
				accountID := p.Key("sso_account_id").String()
				if accountID == "" {
					accountID = "_unknown"
				}
				profilesByAccount[accountID] = append(profilesByAccount[accountID], p)
			}

			var accountIDs []string
			for id := range profilesByAccount {
				accountIDs = append(accountIDs, id)
			}
			sort.Strings(accountIDs)

			for _, accountID := range accountIDs {
				accountProfiles := profilesByAccount[accountID]
				sort.Slice(accountProfiles, func(i, j int) bool {
					return accountProfiles[i].Name() < accountProfiles[j].Name()
				})

				if accountID != "_unknown" {
					b.WriteString(fmt.Sprintf("# Account: %s\n", accountID))
				}

				for _, section := range accountProfiles {
					writeSection(&b, section)
				}
			}
		}
	}

	// --- IAM Role Profiles ---
	if len(iamProfiles) > 0 {
		sort.Slice(iamProfiles, func(i, j int) bool {
			return iamProfiles[i].Name() < iamProfiles[j].Name()
		})

		b.WriteString("# ─── IAM Role Profiles ──────────────────────────────────────────────────────\n\n")
		for _, section := range iamProfiles {
			writeSection(&b, section)
		}
	}

	// --- Static Key Profiles ---
	if len(keyProfiles) > 0 {
		sort.Slice(keyProfiles, func(i, j int) bool {
			return keyProfiles[i].Name() < keyProfiles[j].Name()
		})

		b.WriteString("# ─── Static Key Profiles ────────────────────────────────────────────────────\n\n")
		for _, section := range keyProfiles {
			writeSection(&b, section)
		}
	}

	// --- Other profiles ---
	if len(otherProfiles) > 0 {
		sort.Slice(otherProfiles, func(i, j int) bool {
			return otherProfiles[i].Name() < otherProfiles[j].Name()
		})

		b.WriteString("# ─── Other Profiles ─────────────────────────────────────────────────────────\n\n")
		for _, section := range otherProfiles {
			writeSection(&b, section)
		}
	}

	return b.String()
}

// writeSection writes a single ini section to the builder with consistent
// formatting.
func writeSection(b *strings.Builder, section *ini.Section) {
	writeSectionAs(b, section, section.Name())
}

// writeSectionAs writes a section under an explicit header name.
//
// Comments the user put in the file are re-emitted, and keys carrying a nested
// sub-section (an empty value followed by indented lines, such as "s3 =") keep
// their children. Dropping either used to silently rewrite the user's file:
// losing the children left "s3 =" with an empty string value, which makes
// botocore abort with "'str' object has no attribute 'get'".
func writeSectionAs(b *strings.Builder, section *ini.Section, name string) {
	writeComment(b, section.Comment)
	b.WriteString(fmt.Sprintf("[%s]\n", name))

	// Write keys in a deterministic order for common AWS keys
	keyOrder := []string{
		"sso_start_url", "sso_region", "sso_registration_scopes",
		"sso_session", "sso_account_id", "sso_role_name",
		"role_arn", "source_profile", "mfa_serial", "external_id",
		"credential_process",
		"aws_access_key_id", "aws_secret_access_key", "aws_session_token",
		"region", "output",
	}

	written := make(map[string]bool)

	// Write known keys in preferred order
	for _, keyName := range keyOrder {
		if section.HasKey(keyName) {
			key := section.Key(keyName)
			if key.Value() != "" {
				writeKey(b, key)
				written[keyName] = true
			}
		}
	}

	// Write any remaining keys not in the preferred order
	for _, key := range section.Keys() {
		if !written[key.Name()] {
			writeKey(b, key)
		}
	}

	b.WriteString("\n")
}

// writeKey writes one key with its comment and any nested sub-section.
func writeKey(b *strings.Builder, key *ini.Key) {
	writeComment(b, key.Comment)
	nested := key.NestedValues()
	if key.Value() == "" && len(nested) > 0 {
		b.WriteString(fmt.Sprintf("%-24s=\n", key.Name()))
	} else {
		b.WriteString(fmt.Sprintf("%-24s= %s\n", key.Name(), key.Value()))
	}
	for _, child := range nested {
		b.WriteString(fmt.Sprintf("  %s\n", child))
	}
}

// writeComment re-emits a comment block captured by go-ini, skipping the
// banners this file generates itself. On the next read those banners come back
// attached to the section that follows them, so without the filter every run
// of OrganizeConfigFile would stack another copy.
func writeComment(b *strings.Builder, comment string) {
	comment = strings.TrimRight(comment, "\n")
	if comment == "" {
		return
	}
	for _, line := range strings.Split(comment, "\n") {
		if isGeneratedComment(line) {
			continue
		}
		b.WriteString(line + "\n")
	}
}

// generatedAccountComment matches the "# Account: <id>" headings written by
// buildOrganizedConfig.
var generatedAccountComment = regexp.MustCompile(`^[#;]\s*Account:\s*[0-9]+\s*$`)

// isGeneratedComment reports whether a comment line was written by
// buildOrganizedConfig rather than by the user. Every banner it emits contains
// the box drawing character U+2500, which nothing else in an AWS config uses.
func isGeneratedComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
		return false
	}
	if strings.ContainsRune(trimmed, '\u2500') {
		return true
	}
	return generatedAccountComment.MatchString(trimmed)
}
