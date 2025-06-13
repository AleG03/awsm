package cmd

import (
	"fmt"
	"os"

	"awsm/internal/util"
	"github.com/spf13/cobra"
)

var regionSetCmd = &cobra.Command{
	Use:   "set <region>",
	Short: "Sets the AWS region for the current shell session",
	Long: `Outputs shell commands to set the AWS_REGION and AWS_DEFAULT_REGION
environment variables. This should be used with your shell's 'eval' command.

Example:
  eval $(awsm region set us-west-2)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		region := args[0]
		// In a future version, we could validate this is a real AWS region.

		fmt.Printf("export AWS_REGION='%s';\n", region)
		fmt.Printf("export AWS_DEFAULT_REGION='%s';\n", region)

		// Print success message to stderr for the user to see
		util.SuccessColor.Fprintf(os.Stderr, "✔ AWS Region set to '%s'.\n", region)
		return nil
	},
}

func init() {
	regionCmd.AddCommand(regionSetCmd)
}
