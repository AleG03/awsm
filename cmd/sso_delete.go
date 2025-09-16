package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	ssoForceDelete bool
)

var ssoDeleteCmd = &cobra.Command{
	Use:               "delete <sso-session>",
	Short:             "Delete an SSO session and optionally its associated profiles",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSSOSessions,
	RunE: func(cmd *cobra.Command, args []string) error {
		ssoSession := args[0]

		// Check if SSO session exists
		sessions, err := aws.ListSSOSessions()
		if err != nil {
			return err
		}

		var sessionExists bool
		for _, session := range sessions {
			if session.Name == ssoSession {
				sessionExists = true
				break
			}
		}

		if !sessionExists {
			util.WarnColor.Printf("SSO session '%s' does not exist\n", ssoSession)
			return nil
		}

		// Get associated profiles
		profiles, err := aws.GetProfilesBySSO(ssoSession)
		if err != nil {
			return err
		}

		// Show what will be deleted
		util.InfoColor.Printf("SSO session '%s' will be deleted\n", ssoSession)
		if len(profiles) > 0 {
			util.InfoColor.Printf("This will also delete %d associated profiles:\n", len(profiles))
			for _, profile := range profiles {
				fmt.Printf("  - %s\n", profile)
			}
		}

		// Confirm deletion unless forced
		if !ssoForceDelete {
			totalItems := 1 + len(profiles)
			confirm, err := util.PromptForInput(fmt.Sprintf("Delete SSO session and %d total items? (y/N): ", totalItems))
			if err != nil {
				return err
			}
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				util.InfoColor.Println("Deletion cancelled")
				return nil
			}
		}

		// Delete associated profiles first
		for _, profile := range profiles {
			if err := aws.DeleteProfile(profile); err != nil {
				util.ErrorColor.Printf("Failed to delete profile '%s': %v\n", profile, err)
			} else {
				util.SuccessColor.Printf("✔ Deleted profile '%s'\n", profile)
			}
		}

		// Delete SSO session
		if err := aws.DeleteSSOSession(ssoSession); err != nil {
			return fmt.Errorf("failed to delete SSO session: %w", err)
		}

		util.SuccessColor.Printf("✔ SSO session '%s' deleted successfully\n", ssoSession)
		return nil
	},
}

func init() {
	ssoDeleteCmd.Flags().BoolVarP(&ssoForceDelete, "force", "f", false, "Force deletion without confirmation")
	ssoCmd.AddCommand(ssoDeleteCmd)
}
