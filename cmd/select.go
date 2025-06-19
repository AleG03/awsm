package cmd

import (
	"fmt"

	"awsm/internal/tui"

	"github.com/spf13/cobra"
)

var selectCmd = &cobra.Command{
	Use:     "select",
	Short:   "Interactively select and export an AWS profile",
	Long:    `Opens an interactive profile selector to choose and export AWS credentials.`,
	Aliases: []string{"s"},
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, err := tui.SelectProfile()
		if err != nil {
			return err
		}

		if profileName == "" {
			fmt.Println(tui.MutedStyle.Render("No profile selected."))
			return nil
		}

		// Export the selected profile
		return runProfileSet(cmd, []string{profileName})
	},
}

func init() {
	rootCmd.AddCommand(selectCmd)
}
