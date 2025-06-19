/*
AWSM - AWS Manager
Copyright (c) 2024 Alessandro Gallo. All rights reserved.

Licensed under the Business Source License 1.1.
See LICENSE file for full terms.
*/

package main

import (
	"awsm/cmd"
	"awsm/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	config.InitConfig()
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
