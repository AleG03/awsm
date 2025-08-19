package cmd

import (
	"awsm/internal/aws"
	"fmt"

	"github.com/spf13/cobra"
)

var profileCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the currently active profile name",
	Long:  `Display the name of the profile currently set in the default credentials.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := aws.GetCurrentProfileName()
		if profileName == "" {
			return fmt.Errorf("no active profile found")
		}
		fmt.Println(profileName)
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileCurrentCmd)
}
