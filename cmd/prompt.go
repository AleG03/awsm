package cmd

import (
	"fmt"
	"strings"
	"time"

	"awsm/internal/aws"
	"awsm/internal/daemon"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	promptFormat      string
	promptNoColor     bool
	promptEmptyOnNone bool
)

const defaultPromptFormat = "{icon} {profile}:{region} {ttl}{blocked}"

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Print a compact one-line status string for use in shell prompts",
	Long: `Print a compact representation of the current AWS profile suitable for
embedding in a shell prompt (PS1, starship custom command, etc.).

The command is designed to be fast and offline: it never calls AWS APIs. It
reads the active profile from ~/.aws/credentials and the cached credentials
(if any) from ~/.awsm/cache.

The format string supports the following placeholders:
  {profile}  active profile name
  {region}   profile region
  {type}     profile type (SSO/IAM/Key)
  {ttl}      remaining TTL of cached credentials (e.g. "2h15m", "expired", "static")
  {account}  AWS account id (only available when present in profile config)
  {icon}     small icon based on profile type
  {blocked}  warning when the refresh daemon needs you (e.g. " ⚠ MFA", " ⚠ SSO")

Examples:
  awsm prompt
  awsm prompt --format '{profile}@{region}'
  awsm prompt --format '☁ {profile} ({ttl})'

Recommended starship.toml snippet:
  [custom.awsm]
  command = "awsm prompt --no-color"
  when = "awsm profile current"
  format = "[$output]($style) "`,
	RunE: runPrompt,
}

func init() {
	promptCmd.Flags().StringVarP(&promptFormat, "format", "f", defaultPromptFormat, "Format string")
	promptCmd.Flags().BoolVar(&promptNoColor, "no-color", false, "Disable ANSI color output")
	promptCmd.Flags().BoolVar(&promptEmptyOnNone, "empty-on-none", false, "Print nothing (exit 0) when no active profile")
	rootCmd.AddCommand(promptCmd)
}

func runPrompt(cmd *cobra.Command, args []string) error {
	profile := aws.GetCurrentProfileName()
	if profile == "" {
		if promptEmptyOnNone {
			return nil
		}
		fmt.Println("(no profile)")
		return nil
	}

	region, _ := aws.GetProfileRegion(profile)
	pType, accountID := lookupTypeAndAccount(profile)

	ttlText, ttlColor := computeTTL(profile, pType)

	repl := strings.NewReplacer(
		"{blocked}", blockedMarker(profile, promptNoColor),
		"{profile}", profile,
		"{region}", region,
		"{type}", string(pType),
		"{ttl}", colorize(ttlText, ttlColor, promptNoColor),
		"{account}", accountID,
		"{icon}", iconFor(pType),
	)
	out := repl.Replace(promptFormat)

	// Collapse double spaces left by empty placeholders.
	out = strings.Join(strings.Fields(out), " ")

	fmt.Println(out)
	return nil
}

// lookupTypeAndAccount returns the profile type and best-effort account id
// without making any AWS API call.
func lookupTypeAndAccount(name string) (aws.ProfileType, string) {
	profiles, err := aws.ListProfilesDetailed()
	if err != nil {
		return "", ""
	}
	for _, p := range profiles {
		if p.Name != name {
			continue
		}
		acc := p.SSOAccountID
		if acc == "" && p.RoleARN != "" {
			parts := strings.Split(p.RoleARN, ":")
			if len(parts) >= 5 {
				acc = parts[4]
			}
		}
		return p.Type, acc
	}
	return "", ""
}

// computeTTL returns a short string describing the credential lifetime and the
// color category to render it with. It only reads the on-disk cache.
func computeTTL(profile string, pType aws.ProfileType) (string, lipgloss.Color) {
	muted := lipgloss.Color("#6B7280")
	if pType == aws.ProfileTypeKey {
		return "static", muted
	}

	// 1. awsm cache (set for IAM assume-role / session-token flows).
	exp, ok := aws.ActiveCredentialsExpiry(profile)
	// 2. AWS CLI SSO token cache (covers SSO profiles).
	if !ok && pType == aws.ProfileTypeSSO {
		exp, ok = aws.SSOTokenExpiry(profile)
	}
	if !ok {
		return "?", muted
	}

	d := time.Until(exp)
	switch {
	case d <= 0:
		return "expired", lipgloss.Color("#EF4444")
	case d <= 5*time.Minute:
		return humanizeDuration(d), lipgloss.Color("#EF4444")
	case d <= 30*time.Minute:
		return humanizeDuration(d), lipgloss.Color("#F59E0B")
	default:
		return humanizeDuration(d), lipgloss.Color("#10B981")
	}
}

// blockedMarker reports what the refresh daemon is stuck on, if anything.
//
// The daemon's notification is a moment; this is the reminder that stays, and
// it appears where the credentials are about to be used rather than in the
// middle of the screen. Empty when nothing is wrong, so a healthy prompt is
// unchanged.
func blockedMarker(profile string, noColor bool) string {
	if profile == "" {
		return ""
	}
	reason, ok := daemon.BlockedFor(profile)
	if !ok {
		return ""
	}
	return " " + colorize("⚠ "+reason.Short(), lipgloss.Color("#EF4444"), noColor)
}

func iconFor(pType aws.ProfileType) string {
	switch pType {
	case aws.ProfileTypeSSO:
		return "☁"
	case aws.ProfileTypeIAM:
		return "🔐"
	case aws.ProfileTypeKey:
		return "🔑"
	default:
		return ""
	}
}

func colorize(text string, color lipgloss.Color, disable bool) string {
	if disable || text == "" {
		return text
	}
	// Force color output even when stdout is not a TTY (shells eat the codes).
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(text)
}
