package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	ssoListDetailed bool
	ssoFilterRegion string
	ssoNameFilter   string
	ssoSortBy       string
	ssoOutputJSON   bool
)

var ssoListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all SSO sessions",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := aws.ListSSOSessions()
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			if !ssoOutputJSON {
				util.WarnColor.Println("No SSO sessions found.")
			} else {
				fmt.Println("[]")
			}
			return nil
		}

		// Apply filters
		var filtered []aws.SSOSessionInfo
		for _, s := range sessions {
			if ssoFilterRegion != "" && !strings.EqualFold(s.Region, ssoFilterRegion) {
				continue
			}
			if ssoNameFilter != "" && !strings.Contains(strings.ToLower(s.Name), strings.ToLower(ssoNameFilter)) {
				continue
			}
			filtered = append(filtered, s)
		}

		if len(filtered) == 0 {
			if !ssoOutputJSON {
				util.WarnColor.Println("No SSO sessions match the specified filters.")
			} else {
				fmt.Println("[]")
			}
			return nil
		}

		// Sort sessions
		switch strings.ToLower(ssoSortBy) {
		case "region":
			util.SortBy(filtered, func(s1, s2 aws.SSOSessionInfo) bool {
				return s1.Region < s2.Region
			})
		default: // default sort by name
			util.SortBy(filtered, func(s1, s2 aws.SSOSessionInfo) bool {
				return s1.Name < s2.Name
			})
		}

		if ssoOutputJSON {
			return outputSSOSessionsJSON(filtered)
		}

		// Print sessions
		if ssoListDetailed {
			printDetailedSSOSessions(filtered)
		} else {
			printSimpleSSOSessions(filtered)
		}

		return nil
	},
}

func outputSSOSessionsJSON(sessions []aws.SSOSessionInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(sessions)
}

func printSimpleSSOSessions(sessions []aws.SSOSessionInfo) {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D9FF")).
		Bold(true)

	fmt.Println(headerStyle.Render("🔐 SSO Sessions"))
	fmt.Println(headerStyle.Render("═════════════"))
	fmt.Println()

	for _, s := range sessions {
		util.SuccessColor.Printf("● %s\n", s.Name)
		util.InfoColor.Printf("  Region: %s\n", s.Region)
		fmt.Println()
	}
}

func printDetailedSSOSessions(sessions []aws.SSOSessionInfo) {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D9FF")).
		Bold(true)

	fmt.Println(headerStyle.Render("🔐 SSO Sessions (Detailed)"))
	fmt.Println(headerStyle.Render("═══════════════════════"))
	fmt.Println()

	for i, s := range sessions {
		util.SuccessColor.Printf("● Session: %s\n", s.Name)
		fmt.Printf("  ├── Start URL: %s\n", s.StartURL)
		fmt.Printf("  ├── Region: %s\n", s.Region)
		fmt.Printf("  └── Scopes: %s\n", s.Scopes)
		if i < len(sessions)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func init() {
	ssoListCmd.Flags().BoolVarP(&ssoListDetailed, "detailed", "d", false, "Show detailed session information")
	ssoListCmd.Flags().StringVarP(&ssoFilterRegion, "region", "r", "", "Filter by region")
	ssoListCmd.Flags().StringVarP(&ssoNameFilter, "name", "n", "", "Filter by session name (case-insensitive)")
	ssoListCmd.Flags().StringVarP(&ssoSortBy, "sort", "s", "name", "Sort by field (name, region)")
	ssoListCmd.Flags().BoolVarP(&ssoOutputJSON, "json", "j", false, "Output sessions in JSON format")
	ssoCmd.AddCommand(ssoListCmd)
}
