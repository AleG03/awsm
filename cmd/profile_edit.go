package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var profileEditCmd = &cobra.Command{
	Use:   "edit <profile-name>",
	Short: "Edit an existing AWS profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Check if profile exists
		exists, err := aws.ProfileExists(profileName)
		if err != nil {
			return err
		}
		if !exists {
			util.WarnColor.Printf("Profile '%s' does not exist\n", profileName)
			return nil
		}

		// Get current profile info
		profiles, err := aws.ListProfilesDetailed()
		if err != nil {
			return err
		}

		var currentProfile *aws.ProfileInfo
		for _, p := range profiles {
			if p.Name == profileName {
				currentProfile = &p
				break
			}
		}

		if currentProfile == nil {
			return fmt.Errorf("profile '%s' not found", profileName)
		}

		util.InfoColor.Printf("Editing profile: %s (Type: %s)\n", util.BoldColor.Sprint(profileName), currentProfile.Type)

		switch currentProfile.Type {
		case aws.ProfileTypeSSO:
			util.WarnColor.Println("SSO profiles cannot be edited directly. Use 'awsm sso generate' to recreate them.")
			return nil
		case aws.ProfileTypeIAM:
			return editIAMProfile(profileName, currentProfile)
		case aws.ProfileTypeKey:
			return editIAMUserProfile(profileName, currentProfile)
		default:
			return fmt.Errorf("unknown profile type: %s", currentProfile.Type)
		}
	},
}

func editIAMProfile(profileName string, current *aws.ProfileInfo) error {
	util.InfoColor.Println("Current IAM role configuration:")
	fmt.Printf("  Role ARN: %s\n", current.RoleARN)
	fmt.Printf("  Source Profile: %s\n", current.SourceProfile)
	fmt.Printf("  MFA Serial: %s\n", current.MFASerial)
	fmt.Printf("  Region: %s\n", current.Region)
	fmt.Println()

	roleArn, err := util.PromptForInput(fmt.Sprintf("Role ARN [%s]: ", current.RoleARN))
	if err != nil {
		return err
	}
	if strings.TrimSpace(roleArn) == "" {
		roleArn = current.RoleARN
	}

	sourceProfile, err := util.PromptForInput(fmt.Sprintf("Source profile [%s]: ", current.SourceProfile))
	if err != nil {
		return err
	}
	if strings.TrimSpace(sourceProfile) == "" {
		sourceProfile = current.SourceProfile
	}

	mfaSerial, err := util.PromptForInput(fmt.Sprintf("MFA Serial [%s]: ", current.MFASerial))
	if err != nil {
		return err
	}
	if strings.TrimSpace(mfaSerial) == "" {
		mfaSerial = current.MFASerial
	}

	region, err := util.PromptForInput(fmt.Sprintf("Region [%s]: ", current.Region))
	if err != nil {
		return err
	}
	if strings.TrimSpace(region) == "" {
		region = current.Region
	}

	// Delete old profile and create new one
	if err := aws.DeleteProfile(profileName); err != nil {
		return fmt.Errorf("failed to delete old profile: %w", err)
	}

	if err := aws.AddIAMRoleProfile(profileName, roleArn, sourceProfile, mfaSerial, region); err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	util.SuccessColor.Printf("✔ Profile '%s' updated successfully\n", profileName)
	return nil
}

func editIAMUserProfile(profileName string, current *aws.ProfileInfo) error {
	util.InfoColor.Println("Current IAM user profile configuration:")
	fmt.Printf("  Region: %s\n", current.Region)
	fmt.Println()
	util.WarnColor.Println("Note: Access keys cannot be displayed for security reasons")

	updateKeys, err := util.PromptForInput("Update access keys? (y/N): ")
	if err != nil {
		return err
	}

	var accessKey, secretKey string
	if strings.ToLower(strings.TrimSpace(updateKeys)) == "y" {
		accessKey, err = util.PromptForInput("New AWS Access Key ID: ")
		if err != nil {
			return err
		}

		secretKey, err = util.PromptForInput("New AWS Secret Access Key: ")
		if err != nil {
			return err
		}
	}

	region, err := util.PromptForInput(fmt.Sprintf("Region [%s]: ", current.Region))
	if err != nil {
		return err
	}
	if strings.TrimSpace(region) == "" {
		region = current.Region
	}

	if accessKey != "" && secretKey != "" {
		// Delete old profile and create new one with new keys
		if err := aws.DeleteProfile(profileName); err != nil {
			return fmt.Errorf("failed to delete old profile: %w", err)
		}

		if err := aws.AddIAMUserProfile(profileName, accessKey, secretKey, region); err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}
	} else {
		// Just update region
		if err := aws.UpdateProfileRegion(profileName, region); err != nil {
			return fmt.Errorf("failed to update region: %w", err)
		}
	}

	util.SuccessColor.Printf("✔ Profile '%s' updated successfully\n", profileName)
	return nil
}

func init() {
	profileCmd.AddCommand(profileEditCmd)
}
