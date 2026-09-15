package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/tui"
	"fmt"

	"github.com/spf13/cobra"
)

var profileEditCmd = &cobra.Command{
	Use:               "edit <profile-name>",
	Short:             "Edit an existing AWS profile",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProfiles,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Check if profile exists
		exists, err := aws.ProfileExists(profileName)
		if err != nil {
			return err
		}
		if !exists {
			tui.PrintWarning(fmt.Sprintf("Profile '%s' does not exist", profileName))
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

		tui.PrintInfo(fmt.Sprintf("Editing profile: %s (Type: %s)",
			tui.FormatBold(profileName), tui.FormatProfileType(string(currentProfile.Type))))

		switch currentProfile.Type {
		case aws.ProfileTypeSSO:
			tui.PrintWarning("SSO profiles cannot be edited directly. Use 'awsm sso generate' to recreate them.")
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
	tui.PrintHeader("Current IAM Role Configuration")
	tui.PrintKeyValue("Role ARN", current.RoleARN)
	tui.PrintKeyValue("Source Profile", current.SourceProfile)
	tui.PrintKeyValue("MFA Serial", current.MFASerial)
	tui.PrintKeyValue("External ID", current.ExternalID)
	tui.PrintKeyValue("Region", current.Region)

	roleArn, err := tui.PromptInput("Role ARN", tui.WithDefault(current.RoleARN))
	if err != nil {
		return err
	}
	if roleArn == "" {
		roleArn = current.RoleARN
	}

	sourceProfile, err := tui.PromptInput("Source profile", tui.WithDefault(current.SourceProfile))
	if err != nil {
		return err
	}
	if sourceProfile == "" {
		sourceProfile = current.SourceProfile
	}

	mfaSerial, err := tui.PromptInput("MFA Serial", tui.WithDefault(current.MFASerial))
	if err != nil {
		return err
	}
	if mfaSerial == "" {
		mfaSerial = current.MFASerial
	}

	externalID, err := tui.PromptInput("External ID", tui.WithDefault(current.ExternalID))
	if err != nil {
		return err
	}
	if externalID == "" {
		externalID = current.ExternalID
	}

	region, err := tui.PromptInput("Region", tui.WithDefault(current.Region))
	if err != nil {
		return err
	}
	if region == "" {
		region = current.Region
	}

	if err := aws.UpdateIAMRoleProfile(profileName, aws.IAMRoleProfile{
		RoleARN:       roleArn,
		SourceProfile: sourceProfile,
		MFASerial:     mfaSerial,
		ExternalID:    externalID,
		Region:        region,
	}); err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	tui.PrintSuccess(fmt.Sprintf("Profile '%s' updated successfully", profileName))
	return nil
}

func editIAMUserProfile(profileName string, current *aws.ProfileInfo) error {
	tui.PrintHeader("Current IAM User Configuration")
	tui.PrintKeyValue("Region", current.Region)
	tui.PrintWarning("Access keys cannot be displayed for security reasons")

	updateKeys, err := tui.Confirm("Update access keys?")
	if err != nil {
		return err
	}

	var accessKey, secretKey string
	if updateKeys {
		accessKey, err = tui.PromptInput("New AWS Access Key ID",
			tui.WithRequired(), tui.WithPlaceholder("AKIA..."))
		if err != nil {
			return err
		}

		secretKey, err = tui.PromptInput("New AWS Secret Access Key",
			tui.WithRequired(), tui.WithEchoPassword())
		if err != nil {
			return err
		}
	}

	region, err := tui.PromptInput("Region", tui.WithDefault(current.Region))
	if err != nil {
		return err
	}
	if region == "" {
		region = current.Region
	}

	if accessKey != "" && secretKey != "" {
		if err := aws.UpdateIAMUserProfile(profileName, accessKey, secretKey, region); err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}
	} else {
		if err := aws.UpdateProfileRegion(profileName, region); err != nil {
			return fmt.Errorf("failed to update region: %w", err)
		}
	}

	tui.PrintSuccess(fmt.Sprintf("Profile '%s' updated successfully", profileName))
	if accessKey != "" && secretKey != "" {
		offerToRestrictConfig()
	}
	return nil
}

func init() {
	profileCmd.AddCommand(profileEditCmd)
}
