package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// InitConfig initializes Viper to read the awsm configuration file.
// It should be called once when the application starts.
func InitConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		// This is unlikely to fail, but handle it gracefully.
		fmt.Fprintln(os.Stderr, "Warning: Could not find home directory. Chrome profile mapping will not work.")
		return
	}

	// Set the path for the config file: ~/.config/awsm/
	configPath := filepath.Join(home, ".config", "awsm")

	// Set Viper's configuration
	viper.AddConfigPath(configPath) // Where to look for the config file
	viper.SetConfigName("config")   // Name of the config file (without extension)
	viper.SetConfigType("toml")     // The type of the config file

	// If a config file is found, read it in.
	// It's okay if the file doesn't exist; we just won't have any mappings.
	viper.ReadInConfig()
}

// GetChromeProfileDirectory looks up a friendly profile name (alias)
// in the config file and returns the actual directory name.
// If no alias is found, it assumes the input is already the directory name.
func GetChromeProfileDirectory(alias string) string {
	// If no alias is provided, do nothing.
	if alias == "" {
		return ""
	}

	// Viper keys are case-insensitive.
	// It looks for a key like `chrome_profiles.work` in the config file.
	key := fmt.Sprintf("chrome_profiles.%s", alias)
	if viper.IsSet(key) {
		// The alias exists! Return the mapped directory name.
		return viper.GetString(key)
	}

	// No mapping found, assume the input is already a directory name.
	return alias
}
