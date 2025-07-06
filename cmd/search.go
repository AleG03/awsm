package cmd

import (
	"fmt"
	"os"
	"strings"

	"awsm/internal/aws"
	"awsm/internal/tui"

	"github.com/spf13/cobra"
)

var (
	searchAccountID bool
	searchProfile   bool
	searchSSO       bool
	caseSensitive   bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search AWS profiles, account IDs, and SSO sessions",
	Long: `Search through AWS configuration files for profiles, account IDs, and SSO sessions.

By default, searches all fields (profiles, account IDs, SSO sessions).
Use specific flags to limit search scope.

Examples:
  awsm search 123456789012           # Search everything for account ID
  awsm search 1234                   # Search everything for partial account ID
  awsm search my-profile             # Search everything for profile name
  awsm search my-sso-session         # Search everything for SSO session
  awsm search --account 123456789    # Search only account IDs
  awsm search --profile prod         # Search only profile names
  awsm search --sso my-session       # Search only SSO sessions`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	profiles, err := aws.ListProfilesDetailed()
	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	sessions, err := aws.ListSSOSessions()
	if err != nil {
		return fmt.Errorf("failed to list SSO sessions: %w", err)
	}

	var results []SearchResult

	// If no specific flags are set, search everything
	searchEverything := !searchProfile && !searchAccountID && !searchSSO

	// Search profiles
	if searchProfile || searchEverything {
		for _, profile := range profiles {
			if matchesQuery(profile.Name, query) {
				results = append(results, SearchResult{
					Type:          "Profile",
					Name:          profile.Name,
					AccountID:     profile.SSOAccountID,
					Region:        profile.Region,
					ProfileType:   string(profile.Type),
					SSOSession:    profile.SSOSession,
					RoleARN:       profile.RoleARN,
					SourceProfile: profile.SourceProfile,
				})
			}
		}
	}

	// Search account IDs
	if searchAccountID || searchEverything {
		for _, profile := range profiles {
			if profile.SSOAccountID != "" && matchesQuery(profile.SSOAccountID, query) {
				results = append(results, SearchResult{
					Type:        "Account ID",
					Name:        profile.Name,
					AccountID:   profile.SSOAccountID,
					Region:      profile.Region,
					ProfileType: string(profile.Type),
					SSOSession:  profile.SSOSession,
				})
			}
		}
	}

	// Search SSO sessions
	if searchSSO || searchEverything {
		for _, session := range sessions {
			if matchesQuery(session.Name, query) {
				results = append(results, SearchResult{
					Type:     "SSO Session",
					Name:     session.Name,
					StartURL: session.StartURL,
					Region:   session.Region,
				})
			}
		}
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "No results found for query: %s\n", query)
		return nil
	}

	displayResults(results, query)
	return nil
}

type SearchResult struct {
	Type          string
	Name          string
	AccountID     string
	Region        string
	ProfileType   string
	SSOSession    string
	RoleARN       string
	SourceProfile string
	StartURL      string
	SSORoleName   string
}

func matchesQuery(text, query string) bool {
	if !caseSensitive {
		text = strings.ToLower(text)
		query = strings.ToLower(query)
	}
	return strings.Contains(text, query)
}

func displayResults(results []SearchResult, query string) {
	fmt.Fprintf(os.Stderr, "%s Found %d result(s) for '%s':\n\n",
		tui.SuccessStyle.Render("✓"), len(results), query)

	for i, result := range results {
		if i > 0 {
			fmt.Println()
		}

		// Display profile name with type-specific color
		var bullet, name string
		switch result.ProfileType {
		case "SSO":
			bullet = tui.ProfileSSO.Render("●")
			name = tui.ProfileSSO.Render(result.Name)
		case "IAM":
			bullet = tui.ProfileIAM.Render("●")
			name = tui.ProfileIAM.Render(result.Name)
		case "Key":
			bullet = tui.ProfileKey.Render("●")
			name = tui.ProfileKey.Render(result.Name)
		default:
			bullet = tui.InfoStyle.Render("●")
			name = tui.InfoStyle.Render(result.Name)
		}

		fmt.Printf("%s %s\n", bullet, name)

		// Display details with consistent formatting
		if result.AccountID != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("Account:"), result.AccountID)
		}
		if result.Region != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("Region:"), result.Region)
		}
		if result.ProfileType != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("Type:"), getTypeDisplay(result.ProfileType))
		}
		if result.SSOSession != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("SSO Session:"), result.SSOSession)
		}
		if result.SSORoleName != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("SSO Role:"), result.SSORoleName)
		}
		if result.RoleARN != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("Role ARN:"), result.RoleARN)
		}
		if result.SourceProfile != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("Source:"), result.SourceProfile)
		}
		if result.StartURL != "" {
			fmt.Printf("  %s %s\n",
				tui.MutedStyle.Render("Start URL:"), result.StartURL)
		}
	}
}

func getTypeDisplay(profileType string) string {
	switch profileType {
	case "SSO":
		return tui.ProfileSSO.Render("SSO")
	case "IAM":
		return tui.ProfileIAM.Render("IAM")
	case "Key":
		return tui.ProfileKey.Render("Key")
	default:
		return profileType
	}
}

func init() {
	searchCmd.Flags().BoolVarP(&searchAccountID, "account", "a", false, "Search only account IDs")
	searchCmd.Flags().BoolVarP(&searchProfile, "profile", "p", false, "Search only profile names")
	searchCmd.Flags().BoolVarP(&searchSSO, "sso", "s", false, "Search only SSO session names")
	searchCmd.Flags().BoolVarP(&caseSensitive, "case-sensitive", "c", false, "Case-sensitive search")

	rootCmd.AddCommand(searchCmd)
}
