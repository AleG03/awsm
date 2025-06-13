package cmd

import (

	"awsm/internal/aws"
	"awsm/internal/util"

	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:     "profile",
	Short:   "Manage AWS profiles",
	Aliases: []string{"p"},
}

var profileListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all available AWS profiles",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := aws.ListProfiles()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			util.WarnColor.Println("No profiles found.")
			return nil
		}

		util.InfoColor.Println("Available AWS Profiles:")
		var data [][]string
		for _, p := range profiles {
			data = append(data, []string{p})
		}

		util.PrintTable([]string{"Profile"}, data)
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	rootCmd.AddCommand(profileCmd)
}
