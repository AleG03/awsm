package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"awsm/internal/aws"
	"awsm/internal/browser"
	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
)

var (
	dontOpenBrowser bool
	useFirefox      bool
	chromeProfile   string
)

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Opens the AWS console in your browser",
	Long: `Generates a federated sign-in URL for the current AWS session and
automatically opens it in your default browser, a specified Chrome profile,
or a Firefox Multi-Account Container.

By default, opens in the default browser.
Use --chrome-profile to open in a specific Chrome profile.
Use --firefox-container to open in a Firefox container matching your AWS profile name.

Make sure to set a session first with 'awsmp <profile-name>'.`,
	Aliases: []string{"c", "open"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Auto-refresh credentials if needed
		currentProfile := os.Getenv("AWS_PROFILE")
		if currentProfile != "" {
			if err := aws.AutoRefreshCredentials(currentProfile); err != nil {
				util.WarnColor.Fprintf(os.Stderr, "Auto-refresh failed: %v\n", err)
			}
		}

		awsCfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w. Have you set credentials?", err)
		}
		creds, err := awsCfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to retrieve credentials: %w. Have you set credentials?", err)
		}
		if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
			return fmt.Errorf("could not find active AWS credentials. Please run 'awsm profile set <profile-name>' first")
		}
		sessionJSON, err := json.Marshal(map[string]string{"sessionId": creds.AccessKeyID, "sessionKey": creds.SecretAccessKey, "sessionToken": creds.SessionToken})
		if err != nil {
			return fmt.Errorf("failed to create session JSON: %w", err)
		}

		// Use POST to avoid URL length issues with long session tokens
		formData := url.Values{}
		formData.Set("Action", "getSigninToken")
		formData.Set("Session", string(sessionJSON))

		resp, err := http.PostForm("https://signin.aws.amazon.com/federation", formData)
		if err != nil {
			return fmt.Errorf("failed to get sign-in token: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("federation service returned status %d: %s", resp.StatusCode, string(body))
		}
		var tokenResp struct {
			SigninToken string `json:"SigninToken"`
		}
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return fmt.Errorf("failed to parse sign-in token response: %w. Response was: %s", err, string(body))
		}
		if tokenResp.SigninToken == "" {
			return fmt.Errorf("sign-in token not found in response. Response was: %s", string(body))
		}
		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = awsCfg.Region
		}
		if region == "" {
			region = "us-east-1"
			util.WarnColor.Fprintln(os.Stderr, "No region found, defaulting to us-east-1")
		}
		destination := fmt.Sprintf("https://%s.console.aws.amazon.com/console/home?region=%s", region, region)
		loginURL := fmt.Sprintf("https://signin.aws.amazon.com/federation?Action=login&Issuer=awsm&Destination=%s&SigninToken=%s", url.QueryEscape(destination), url.QueryEscape(tokenResp.SigninToken))

		if dontOpenBrowser {
			fmt.Println(loginURL)
		} else {
			// If --firefox-container is used, get the current AWS profile name
			var firefoxContainer string
			if useFirefox {
				firefoxContainer = os.Getenv("AWS_PROFILE")
				if firefoxContainer == "" {
					// Fallback to checking credentials file
					firefoxContainer = aws.GetCurrentProfileName()
					if firefoxContainer == "" {
						return fmt.Errorf("no AWS profile set. Please run 'awsm profile set <profile-name>' first")
					}
				}
			}

			if err := browser.OpenURL(loginURL, chromeProfile, firefoxContainer); err != nil {
				fmt.Fprintln(os.Stderr, "Could not open browser automatically. Please copy this URL:")
				fmt.Println(loginURL)
				return fmt.Errorf("could not open browser: %w", err)
			}
		}

		return nil
	},
}

func init() {
	consoleCmd.Flags().BoolVarP(&dontOpenBrowser, "no-open", "n", false, "Don't open the browser, just print the URL")
	consoleCmd.Flags().BoolVarP(&useFirefox, "firefox-container", "f", false, "Open in Firefox using a container named after the AWS profile")
	consoleCmd.Flags().StringVarP(&chromeProfile, "chrome-profile", "c", "", "Specify a Chrome profile alias or directory name (e.g., 'work')")
	rootCmd.AddCommand(consoleCmd)
}
