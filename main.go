package main

import (
	"awsm/cmd"
	"awsm/internal/config"
)

// These variables are set by GoReleaser at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Initialize configuration before doing anything else.
	config.InitConfig()

	// Pass the version info into the cmd package
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
