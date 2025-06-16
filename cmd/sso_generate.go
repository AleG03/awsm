package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate <sso-session-name> <aws-region>",
	Short: "Generates AWS config profiles for all accessible SSO accounts and roles",
	Long: `This powerful command logs into an SSO session, discovers all accounts and roles
you have access to, and generates the corresponding AWS profile configurations.

The generated profiles are saved to '~/.aws/aws_sso_profiles.conf'. You can then
copy the profiles you need into your main '~/.aws/config' file.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ssoSession := args[0]
		awsRegion := args[1]
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot find home directory: %w", err)
		}
		outputFile := filepath.Join(home, ".aws", "config")

		// 1. Log in to get a fresh token cached by the AWS CLI
		util.InfoColor.Printf("Attempting SSO login for session: %s\n", util.BoldColor.Sprint(ssoSession))
		awsCmd := exec.Command("aws", "sso", "login", "--sso-session", ssoSession)
		awsCmd.Stdin = os.Stdin
		awsCmd.Stdout = os.Stdout
		awsCmd.Stderr = os.Stderr
		if err := awsCmd.Run(); err != nil {
			return fmt.Errorf("aws sso login failed: %w", err)
		}
		util.SuccessColor.Println("\n✔ SSO login successful.")

		// 2. Find the cached access token from the filesystem
		util.InfoColor.Println("Finding cached SSO access token...")
		accessToken, err := findLatestSsoToken(filepath.Join(home, ".aws", "sso", "cache"))
		if err != nil {
			return fmt.Errorf("could not find cached SSO token: %w", err)
		}
		util.SuccessColor.Println("✔ Found access token.")

		// 3. Create a basic AWS config with just the region specified.
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
		if err != nil {
			return fmt.Errorf("could not create basic AWS config: %w", err)
		}
		ssoClient := sso.NewFromConfig(cfg)

		// 4. List Accounts using the access token
		util.InfoColor.Println("Fetching all accessible accounts...")
		var accounts []*sso.ListAccountsOutput
		accountsPaginator := sso.NewListAccountsPaginator(ssoClient, &sso.ListAccountsInput{
			AccessToken: &accessToken,
		})
		for accountsPaginator.HasMorePages() {
			page, err := accountsPaginator.NextPage(context.TODO())
			if err != nil {
				return fmt.Errorf("failed to list accounts: %w", err)
			}
			accounts = append(accounts, page)
		}

		totalAccounts := 0
		for _, page := range accounts {
			totalAccounts += len(page.AccountList)
		}
		util.SuccessColor.Printf("✔ Found %d accounts.\n", totalAccounts)

		// 5. Build the profiles from the discovered accounts and roles
		var profileBuilder strings.Builder
		cleaner := regexp.MustCompile(`[^a-zA-Z0-9-]`)
		profileCount := 0

		util.InfoColor.Println("Generating profiles...")
		for _, page := range accounts {
			for _, acc := range page.AccountList {
				util.InfoColor.Fprintf(os.Stderr, "  -> Processing account: %s (%s)\n", *acc.AccountName, *acc.AccountId)

				rolesPaginator := sso.NewListAccountRolesPaginator(ssoClient, &sso.ListAccountRolesInput{
					AccessToken: &accessToken,
					AccountId:   acc.AccountId,
				})
				for rolesPaginator.HasMorePages() {
					rolesPage, err := rolesPaginator.NextPage(context.TODO())
					if err != nil {
						util.ErrorColor.Fprintf(os.Stderr, "    Could not list roles for account %s: %v\n", *acc.AccountId, err)
						continue
					}
					for _, role := range rolesPage.RoleList {
						// Sanitize names for the profile
						cleanAccountName := strings.ToLower(*acc.AccountName)
						cleanAccountName = cleaner.ReplaceAllString(cleanAccountName, "-")
						cleanRoleName := strings.ToLower(*role.RoleName)
						cleanRoleName = cleaner.ReplaceAllString(cleanRoleName, "-")

						profileName := fmt.Sprintf("%s-%s", cleanAccountName, cleanRoleName)

						profileBuilder.WriteString(fmt.Sprintf("[profile %s]\n", profileName))
						profileBuilder.WriteString(fmt.Sprintf("sso_session = %s\n", ssoSession))
						profileBuilder.WriteString(fmt.Sprintf("sso_account_id = %s\n", *acc.AccountId))
						profileBuilder.WriteString(fmt.Sprintf("sso_role_name = %s\n", *role.RoleName))
						profileBuilder.WriteString(fmt.Sprintf("region = %s\n", awsRegion))
						profileBuilder.WriteString("\n")
					}
				}
			}
		}

		// Read existing config
		existingConfigBytes, err := os.ReadFile(outputFile)
		existingConfig := ""
		if err == nil {
			existingConfig = string(existingConfigBytes)
		}

		// Parse existing profile names
		existingProfiles := make(map[string]bool)
		profileHeaderRegex := regexp.MustCompile(`(?m)^\[profile ([^\]]+)\]`)
		for _, match := range profileHeaderRegex.FindAllStringSubmatch(existingConfig, -1) {
			if len(match) > 1 {
				existingProfiles[match[1]] = true
			}
		}

		// Instead of writing all profiles, only append new ones
		var newProfilesBuilder strings.Builder
		for _, page := range accounts {
			for _, acc := range page.AccountList {
				for rolesPaginator := sso.NewListAccountRolesPaginator(ssoClient, &sso.ListAccountRolesInput{
					AccessToken: &accessToken,
					AccountId:   acc.AccountId,
				}); rolesPaginator.HasMorePages(); {
					rolesPage, err := rolesPaginator.NextPage(context.TODO())
					if err != nil {
						util.ErrorColor.Fprintf(os.Stderr, "    Could not list roles for account %s: %v\n", *acc.AccountId, err)
						continue
					}
					for _, role := range rolesPage.RoleList {
						cleanAccountName := strings.ToLower(*acc.AccountName)
						cleanAccountName = cleaner.ReplaceAllString(cleanAccountName, "-")
						cleanRoleName := strings.ToLower(*role.RoleName)
						cleanRoleName = cleaner.ReplaceAllString(cleanRoleName, "-")
						profileName := fmt.Sprintf("%s-%s", cleanAccountName, cleanRoleName)
						if existingProfiles[profileName] {
							continue // Skip if profile already exists
						}
						newProfilesBuilder.WriteString(fmt.Sprintf("[profile %s]\n", profileName))
						newProfilesBuilder.WriteString(fmt.Sprintf("sso_session = %s\n", ssoSession))
						newProfilesBuilder.WriteString(fmt.Sprintf("sso_account_id = %s\n", *acc.AccountId))
						newProfilesBuilder.WriteString(fmt.Sprintf("sso_role_name = %s\n", *role.RoleName))
						newProfilesBuilder.WriteString(fmt.Sprintf("region = %s\n", awsRegion))
						newProfilesBuilder.WriteString("\n")
						profileCount++
					}
				}
			}
		}

		// Only append if there are new profiles
		if newProfilesBuilder.Len() > 0 {
			// Ensure a newline after the previous session
			appendContent := newProfilesBuilder.String()
			if len(existingConfig) > 0 && !strings.HasSuffix(existingConfig, "\n") {
				appendContent = "\n" + appendContent
			}

			f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("failed to open %s for appending: %w", outputFile, err)
			}
			defer f.Close()
			if _, err := f.WriteString(appendContent); err != nil {
				return fmt.Errorf("failed to append profiles to %s: %w", outputFile, err)
			}
			util.SuccessColor.Printf("\n✔ Done! %d new profiles appended to %s\n", profileCount, util.BoldColor.Sprint(outputFile))
		} else {
			util.InfoColor.Println("No new profiles to add. All profiles already exist in your config.")
		}

		util.InfoColor.Println("You can now use the new profiles from your ~/.aws/config.")
		return nil
	},
}

func findLatestSsoToken(cacheDir string) (string, error) {
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("could not read SSO cache directory at %s: %w", cacheDir, err)
	}

	var latestFile os.FileInfo
	var latestTime time.Time

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = info
		}
	}

	if latestFile == nil {
		return "", fmt.Errorf("no valid SSO token cache file found in %s", cacheDir)
	}

	fullPath := filepath.Join(cacheDir, latestFile.Name())
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not read token file %s: %w", fullPath, err)
	}

	var tokenData struct {
		AccessToken string `json:"accessToken"`
	}

	if err := json.Unmarshal(data, &tokenData); err != nil {
		return "", fmt.Errorf("could not parse token file %s: %w", fullPath, err)
	}

	if tokenData.AccessToken == "" {
		return "", fmt.Errorf("accessToken not found in token file %s", fullPath)
	}

	return tokenData.AccessToken, nil
}

func init() {
	ssoCmd.AddCommand(generateCmd)
}
