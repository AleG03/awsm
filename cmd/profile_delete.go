package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	deleteAllSSO bool
	forceDelete  bool
)

var profileDeleteCmd = &cobra.Command{
	Use:               "delete <profile-name>",
	Short:             "Delete an AWS profile",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProfiles,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		if deleteAllSSO {
			return deleteAllSSOProfiles(profileName)
		}

		// Check if profile exists
		exists, err := aws.ProfileExists(profileName)
		if err != nil {
			return err
		}
		if !exists {
			util.WarnColor.Printf("Profile '%s' does not exist\n", profileName)
			return nil
		}

		// Confirm deletion unless forced
		if !forceDelete {
			confirm, err := util.PromptForInput(fmt.Sprintf("Delete profile '%s'? (y/N): ", profileName))
			if err != nil {
				return err
			}
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				util.InfoColor.Println("Deletion cancelled")
				return nil
			}
		}

		if err := aws.DeleteProfile(profileName); err != nil {
			return fmt.Errorf("failed to delete profile: %w", err)
		}

		util.SuccessColor.Printf("✔ Profile '%s' deleted successfully\n", profileName)
		return nil
	},
}

func deleteAllSSOProfiles(ssoSession string) error {
	profiles, err := aws.GetProfilesBySSO(ssoSession)
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		util.WarnColor.Printf("No profiles found for SSO session '%s'\n", ssoSession)
		return nil
	}

	util.InfoColor.Printf("Found %d profiles for SSO session '%s':\n", len(profiles), ssoSession)
	for _, profile := range profiles {
		fmt.Printf("  - %s\n", profile)
	}

	if !forceDelete {
		confirm, err := util.PromptForInput(fmt.Sprintf("Delete all %d profiles? (y/N): ", len(profiles)))
		if err != nil {
			return err
		}
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			util.InfoColor.Println("Deletion cancelled")
			return nil
		}
	}

	for _, profile := range profiles {
		if err := aws.DeleteProfile(profile); err != nil {
			util.ErrorColor.Printf("Failed to delete profile '%s': %v\n", profile, err)
		} else {
			util.SuccessColor.Printf("✔ Deleted profile '%s'\n", profile)
		}
	}

	return nil
}

func init() {
	profileDeleteCmd.Flags().BoolVar(&deleteAllSSO, "all-sso", false, "Delete all profiles for the specified SSO session")
	profileDeleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Force deletion without confirmation")
	profileCmd.AddCommand(profileDeleteCmd)
}
