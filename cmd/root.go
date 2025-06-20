/*
AWSM - AWS Manager
Copyright (c) 2024 Alessandro Gallo. All rights reserved.

Licensed under the Business Source License 1.1.
See LICENSE file for full terms.
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version string
	commit  string
	date    string
)

var rootCmd = &cobra.Command{
	Use:               "awsm",
	Short:             "A fancy CLI to manage your AWS profiles and sessions",
	Long:              `AWSM (AWS Manager) is a tool to simplify switching between AWS profiles, managing regions, and assuming roles with MFA.`,
	Version:           version,
	SilenceUsage:      true,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, date)
}
