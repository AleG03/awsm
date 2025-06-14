package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"awsm/internal/browser" // Import our new browser package
	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
)

var dontOpenBrowser bool

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Opens the AWS console in your browser",
	Long: `Generates a federated sign-in URL for the current AWS session and
automatically opens it in your default browser or a specified Chrome profile.

Make sure to set a session first with 'awsmp <profile-name>'.`,
	Aliases: []string{"c", "open"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// This first part of the logic to generate the URL remains unchanged.
		util.InfoColor.Fprintln(os.Stderr, "Getting current credentials...")
		awsCfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w. Have you exported credentials?", err)
		}
		creds, err := awsCfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to retrieve credentials: %w. Have you exported credentials?", err)
		}
		if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
			return fmt.Errorf("could not find active AWS credentials. Please run 'awsmp <profile-name>' first")
		}
		sessionJSON, err := json.Marshal(map[string]string{"sessionId": creds.AccessKeyID, "sessionKey": creds.SecretAccessKey, "sessionToken": creds.SessionToken})
		if err != nil {
			return fmt.Errorf("failed to create session JSON: %w", err)
		}
		util.InfoColor.Fprintln(os.Stderr, "Requesting sign-in token from AWS...")
		reqURL := fmt.Sprintf("https://signin.aws.amazon.com/federation?Action=getSigninToken&Session=%s", url.QueryEscape(string(sessionJSON)))
		resp, err := http.Get(reqURL)
		if err != nil {
			return fmt.Errorf("failed to get sign-in token: %w", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
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

		// This is the updated section for opening the browser.
		if dontOpenBrowser {
			util.SuccessColor.Fprintln(os.Stderr, "✔ Console URL generated successfully!")
			fmt.Println(loginURL)
		} else {
			util.InfoColor.Fprintln(os.Stderr, "Opening AWS Console...")
			// Use our new smart browser opener, passing in the global flag value.
			if err := browser.OpenURL(loginURL, chromeProfile); err != nil {
				util.ErrorColor.Fprintf(os.Stderr, "Could not open browser automatically. Please copy this URL:\n")
				fmt.Println(loginURL)
				return fmt.Errorf("could not open browser: %w", err)
			}
			util.SuccessColor.Fprintln(os.Stderr, "✔ AWS Console opened.")
		}

		return nil
	},
}

func init() {
	consoleCmd.Flags().BoolVarP(&dontOpenBrowser, "no-open", "n", false, "Only print the URL, do not open in a browser")
	rootCmd.AddCommand(consoleCmd)
}
