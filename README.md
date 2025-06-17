# `awsm` - The AWS Manager

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> A fast command-line tool to manage your AWS profiles, sessions, and console access with ease.

`awsm` (AWS Manager) is a standalone binary written in Go that simplifies the tedious process of switching between AWS profiles, handling MFA, assuming roles, and generating console sign-in links. It's a robust and portable replacement for shell function-based helpers.


## Features

-   🚀 **Standalone & Portable:** A single binary with no runtime dependencies.
-   ✨ **Clean Interface:** Clean, colorized output and formatted tables make it easy to read.
-   🧩 **Intelligent Credential Handling:** Natively supports both **IAM** (MFA/role assumption) and **AWS SSO** (IAM Identity Center) profiles.
-   ⚡️ **Fast & Reliable:** Written in Go.
-   🧠 **Powerful Autocompletion:** Press `<TAB>` to complete commands, flags, and even your personal AWS profile names.
-   🌐 **Console Login:** Quickly generate a federated sign-in URL to open the AWS Console with your current CLI session.


## Table of Contents

- [`awsm` - The AWS Manager](#awsm---the-aws-manager)
  - [Features](#features)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
    - [Option 1: From Binaries (Recommended)](#option-1-from-binaries-recommended)
    - [Option 2: Building from Source](#option-2-building-from-source)
  - [Shell Configuration (Crucial)](#shell-configuration-crucial)
    - [Step 1: Creating the Alias and enable refreshing credentials](#step-1-creating-the-alias-and-enable-refreshing-credentials)
    - [Step 2: Enabling Autocompletion (Recommended)](#step-2-enabling-autocompletion-recommended)
      - [For Zsh](#for-zsh)
      - [For Bash](#for-bash)
  - [Core Concept: Why `awsmp` Uses `export` and `eval`](#core-concept-why-awsmp-uses-export-and-eval)
  - [Usage Guide](#usage-guide)
    - [Working with AWS SSO Profiles](#working-with-aws-sso-profiles)
    - [Working with IAM Profiles](#working-with-iam-profiles)
    - [Switch AWS Profile](#switch-aws-profile)
    - [Switch AWS Region](#switch-aws-region)
    - [AWS Console Login](#aws-console-login)
    - [Using Dedicated Chrome Profiles](#using-dedicated-chrome-profiles)
    - [Using Firefox Multi-Account Containers](#using-firefox-multi-account-containers)
    - [AWS SSO Profiles Generator](#aws-sso-profiles-generator)
      - [Setting up SSO Sessions](#setting-up-sso-sessions)
    - [Listing Profiles \& Regions](#listing-profiles--regions)
    - [Full Command Reference](#full-command-reference)
  - [Version Information](#version-information)
  - [Development](#development)

---


## Installation


### Option 1: From Binaries (Recommended)

_Once binaries are available, find them on the [Releases Page](https://github.com/AleG03/awsm/releases)._

1.  Download the appropriate binary for your operating system and architecture.
2.  Move the binary to a directory in your `PATH`.
    ```bash
    # Move the downloaded file and make it executable
    mv ~/Downloads/awsm /usr/local/bin/awsm
    chmod +x ~/.local/bin/awsm
    ```
3.  Proceed to the [Shell Configuration](#shell-configuration-crucial) section below.


### Option 2: Building from Source

If you have Go (1.24+) installed, you can build `awsm` from source.

1.  **Clone the repository:** 
    ```bash
    git clone https://github.com/your-username/awsm.git
    cd awsm
    ```


2.  **Build the binary:**
    ```bash
    go build .
    ```

3.  **Install the binary:**
    Move the compiled `awsm` file to your local bin directory.
    ```bash
    # Move the new binary
    mv awsm /usr/local/bin/
    ```

---


## Shell Configuration (Crucial)

To get the full power of `awsm`, you need to configure your shell. These steps only need to be done once.


### Step 1: Creating the Alias and enable refreshing credentials

Because a program cannot change the environment of its parent shell, we need to use a shell function and `eval` to export the AWS credentials. This `awsmp` (Assume/Activate Cloud Profile) alias makes the process seamless. Moreover, the `awsmr` alias makes easier to switch region on the fly.

1.  Open your shell's configuration file (`~/.zshrc` or `~/.bashrc`).
2.  Add the following function **after** the PATH export:

    ```sh
    # A smart, multi-purpose helper for activating AWS profiles.
    # This is the main command you will use to interact with awsm.
    #
    # Usage:
    #   awsmp <profile-name>      - Activates a profile, automatically refreshing if SSO is expired.
    #   awsmp login <profile-name>  - Forces a new SSO login and then activates the profile.
    #   awsmp                       - Clears the current AWS session.
    #
    awsmp() {

    if [[ -z "$1" ]]; then

        unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_PROFILE AWS_REGION AWS_DEFAULT_REGION;
        echo "AWS session cleared.";
        return 0;
    fi;

    if [[ "$1" == "login" ]]; then
        if [[ -z "$2" ]]; then
        echo "Usage: awsmp login <profile-name>" >&2;
        return 1;
        fi;
        local profile_name="$2"
        echo -e "\033[33mForcing new SSO login for profile '$profile_name'...\033[0m" >&2
        eval $(awsm sso login "$profile_name");
        if [[ $? -eq 0 ]]; then
        echo -e "\033[32mSSO login successful. Activating profile '$profile_name'...\033[0m" >&2;
        export_commands=$(awsm export "$profile_name")
        eval "$export_commands";
        else
        echo -e "\033[31mSSO login failed.\033[0m" >&2;
        return 1;
        fi;
        return $?;
    fi;

    local profile_name="$1"
    local export_commands
    local exit_code

    export_commands=$(awsm export "$profile_name")
    exit_code=$?

    if [[ $exit_code -eq 10 ]]; then # Expired SSO session
        echo -e "\033[33mSSO session expired. Attempting to refresh...\033[0m" >&2;
        if awsmp login "$profile_name"; then
        return 0
        else
        echo -e "\033[31mSSO login failed. Cannot activate profile.\033[0m" >&2;
        return 1;
        fi
    elif [[ $exit_code -eq 0 ]]; then # Normal success
        eval "$export_commands";
        return 0;
    else # Any other error
        echo -e "\033[31mFailed to switch profile '$profile_name'.\033[0m" >&2;
        return $exit_code;
    fi
    }

    # A helper for switching AWS regions
    awsmr() {
    # `awsmr`: Clear the region override
    if [[ -z "$1" ]]; then
        unset AWS_REGION AWS_DEFAULT_REGION;
        echo "AWS Region override cleared.";
        return 0;
    fi;
    
    # `awsmr <region>`: Set the region for the session directly
    export AWS_REGION="$1"
    export AWS_DEFAULT_REGION="$1"
    echo "✔ AWS Region set to '$1'."
    }

    # A wrapper for the aws command that performs a "pre-flight check"
    # to auto-refresh expired tokens before running the real command.
    # This correctly handles interactive commands like `ssm start-session`.
    aws() {
    # If AWS_PROFILE isn't set, there's nothing for us to do. Run the command directly.
    if [[ -z "$AWS_PROFILE" ]]; then
        command aws "$@";
        return;
    fi;

    # PRE-FLIGHT CHECK:
    # Run a read-only command to check if credentials are valid.
    # We redirect stdout to /dev/null because we only care if it fails.
    if ! command aws sts get-caller-identity > /dev/null 2>&1; then
        # The pre-flight check failed.
        # Re-run the command, but this time capture stderr to check the error message.
        local error_output
        error_output=$(command aws sts get-caller-identity 2>&1 >/dev/null)

        # Check if the error was specifically due to an expired token.
        if echo "$error_output" | grep -q -E 'ExpiredToken|token.*expired'; then
        echo -e "\033[33mAWS token expired. Refreshing session for '$AWS_PROFILE'...\033[0m" >&2;
        
        # If it's expired, call our `awsmp` helper to refresh the session.
        if ! awsmp "$AWS_PROFILE"; then
            echo -e "\033[31mFailed to refresh session. Aborting command.\033[0m" >&2;
            # Print the original error so the user knows what happened.
            echo "$error_output" >&2
            return 1;
        fi
        echo -e "\033[32mSession refreshed. Proceeding with original command...\033[0m" >&2;
        fi
    fi

    # EXECUTE THE REAL COMMAND:
    # If we get here, credentials are valid. Run the user's original command
    # and connect it directly to the terminal.
    command aws "$@";
    }
    ```


### Step 2: Enabling Autocompletion (Recommended)

This step will enable `<TAB>` completion for commands, flags, and profile names, making the tool much faster to use.


#### For Zsh

1.  Create a directory for completion scripts if you don't already have one:
    ```bash
    mkdir -p ~/.zsh/completion
    ```
2.  Generate the completion script and save it:
    ```bash
    awsm completion zsh > ~/.zsh/completion/_awsm
    ```
3.  Add the following to the **end** of your `~/.zshrc` file:
    ```sh
    # Add my custom completion scripts to the function path
    fpath=($HOME/.zsh/completion $fpath)

    # Initialize the completion system
    autoload -U compinit && compinit
    ```


#### For Bash

1.  Install `bash-completion` if you haven't already (e.g., `brew install bash-completion` on macOS, `sudo apt-get install bash-completion` on Debian/Ubuntu).
2.  Generate the completion script into the correct directory:
    ```bash
    awsm completion bash > $(brew --prefix)/etc/bash_completion.d/awsm
    ```
3.  Add the following to your `~/.bashrc` file to source the completions:
    ```sh
    if [ -f "$(brew --prefix)/etc/bash_completion" ]; then
      . "$(brew --prefix)/etc/bash_completion"
    fi
    ```

**Finally, restart your shell or open a new terminal window to apply all changes.**

---


## Core Concept: Why `awsmp` Uses `export` and `eval`

You might be thinking: *"Why use the `awsmp` function? Why not just run `awsm export my-profile` directly?"*

Good question — it comes down to how the shell environment works.

When you run a command like `awsm`, it executes in a **subshell**. This means it **can’t modify your current shell’s environment variables** — including your AWS credentials.

To work around this, `awsm export` doesn’t actually set any variables. Instead, it **prints** the `export` statements you need.

For example:

```
export AWS_ACCESS_KEY_ID='ASI...'
export AWS_SECRET_ACCESS_KEY='abc...'
export AWS_SESSION_TOKEN='xyz...'
export AWS_PROFILE='my-profile'
```

To apply these in your current shell, you wrap the command in `eval`, like this:

```sh
eval $(awsm export my-profile)
```

This causes your shell to:

1. Run `awsm export my-profile`, capturing the output.
2. Pass that output to `eval`, which executes it as shell commands.

The `awsmp` function just wraps this pattern in a convenient, memorable shortcut.

---


## Usage Guide


### Working with AWS SSO Profiles

This is the modern, recommended way to use AWS. The flow is two steps: log in once, then activate the profile as needed.

**Log in to your SSO session:**
This is a one-time action per session (e.g., once a day). `awsm` will open your browser for you to authenticate.
```bash
$ awsmp login your-sso-profile-name
Attempting SSO login for session: my-sso-session-name
Your browser should open...
✔ SSO login successful.
Activating profile 'your-sso-profile-name'...
✔ Credentials for profile 'your-sso-profile-name' are set.
```


### Working with IAM Profiles

This flow is for legacy profiles that use an IAM user's `role_arn` and `mfa_serial`.

**Activate the profile and get credentials:**
The `awsmp` command will prompt you for your MFA token.
```bash
$ awsmp your-iam-profile-name
IAM profile detected. Using STS to get credentials...
Enter MFA token for arn:aws:iam::...: 123456
✔ Credentials for profile 'your-iam-profile-name' are set.
```


### Switch AWS Profile

**Switch the profile:**
Use the `awsmp` alias to get temporary credentials into your shell.
```bash
$ awsmp your-sso-profile-name
SSO profile detected. Using cached session to get credentials...
✔ Credentials for profile 'your-sso-profile-name' are set.

# Now you can use any AWS command
$ aws sts get-caller-identity
```


### Switch AWS Region

If you need to switch Region after setting a profile, use `awsmr` command.
```bash
awsmr eu-west-1
```
When you're done, you can clear the override to go back to your profile's default
```bash
awsmr
```


### AWS Console Login
This feature allows you to generate an AWS Console URL using the current CLI session credentials. You can use the following commands:

1. First, make sure you have an active session
```bash
awsmp my-dev-profile
```
1. Then, beam to the console! It opens your browser by default.
```bash
awsm console
```
If you only want the URL without opening the browser:
```bash
awsm console --no-open
```


### Using Dedicated Chrome Profiles

Do you keep your "Work" and "Personal" lives separate using different Google Chrome profiles? `awsm` can integrate directly into this workflow! You can tell `awsm` to open the AWS Console in a specific profile, preventing session conflicts and keeping your contexts clean.

This is a two-step, one-time setup:

**1. Create a Configuration File for `awsm`**

First, let's give `awsm` a place to store your personal settings.

```bash
# Create the directory
mkdir -p ~/.config/awsm

# Create and open the config file with your favorite editor
nano ~/.config/awsm/config.toml
```

**2. Map Your Friendly Names to Chrome's Internal Names**

Now, let's create simple aliases for your profiles. Open the `config.toml` file you just created and paste this snippet, customizing it with your own details:

```toml
# ~/.config/awsm/config.toml

[chrome_profiles]
# format: alias = "Actual Profile Directory Name"
# To find the directory name, go to chrome://version in that profile.

# Examples:
work = "Profile 1"
personal = "Default"
client-x = "Profile 2"
```

**How do I find my "Profile Directory Name"?**
Easy! Open the Chrome profile you want to use, type `chrome://version` into the address bar, and look for the **Profile Path**. The last part of that path (e.g., `Profile 1`) is what you need.

**3. Use Your New Alias!**

Now, when you want to open the console, just use your new friendly name.

```bash
# First, get a session
awsmp my-dev-profile

# Now, open the console directly in your "work" profile
awsm console --chrome-profile work
```

`awsm` will automatically look up your `work` alias and open the console in the correct Chrome profile.


### Using Firefox Multi-Account Containers

When managing multiple AWS accounts, you can use Firefox's Multi-Account Containers feature to isolate your AWS Console sessions. `awsm` can automatically create and use containers named after your AWS profiles:

```bash
# Open AWS Console in a Firefox container named after your AWS profile
awsm console --firefox-container

# Or use the shorter alias
awsm c --firefox-container
```

The container will be automatically created if it doesn't exist, and Firefox will open the AWS Console in a new tab within that container. This helps you maintain separate sessions for different AWS accounts and prevents cross-account contamination.

Note: You need to have Firefox installed with both Multi-Account Containers extension and Open external links in a container extension.
https://addons.mozilla.org/en-US/firefox/addon/multi-account-containers/ |
https://addons.mozilla.org/en-US/firefox/addon/open-url-in-container/


### AWS SSO Profiles Generator
This feature generates AWS CLI SSO profiles for each account and role. It creates a configuration file with profiles for all accounts and roles accessible via AWS SSO.

#### Setting up SSO Sessions

Before generating profiles, you need to configure your SSO session. You have two options:

1. **Using the AWS CLI** (Recommended):
   ```bash
   # Configure a new SSO session
   aws configure sso
   # Follow the prompts to enter:
   # - SSO start URL (e.g., https://my-sso.awsapps.com/start)
   # - SSO Region (e.g., us-east-1)
   # - Session name (e.g., my-company)
   ```

2. **Manually editing `~/.aws/config`**:
   Add the following to your AWS config file:
   ```ini
   [sso-session my-company]
   sso_start_url = https://my-sso.awsapps.com/start
   sso_region = us-east-1
   sso_registration_scopes = sso:account:access
   ```

Once your SSO session is configured, you can generate profiles:

```bash
# Generate profiles using the configured SSO session
$ awsm sso generate my-company us-east-1
```

This will create profiles for all accounts and roles you have access to via SSO.

### Listing Profiles & Regions

```bash
# List all configured AWS profiles
$ awsm profile list

# List all available AWS regions
$ awsm region list
```

### Full Command Reference

```
awsm
├── console       Opens the AWS console in your browser
├── export        Export temporary credentials for a profile
├── profile
│   └── list      List all available AWS profiles
├── region
│   └── list      List all available AWS regions
├── sso
│   ├── login     Log in to an AWS SSO session
│   └── generate  Generate profiles for all accessible SSO accounts/roles
└── completion    Generate shell autocompletion scripts
```


## Version Information

The `awsm` CLI includes version information that is displayed using the `--version` flag. This includes:

- **Version**: The current version of the CLI.
- **Commit**: The Git commit hash used to build the binary.
- **Date**: The build date.

Example:

```bash
$ awsm --version
Version: 1.0.0
Commit: abc1234
Date: 2025-06-14
```


## Development

Interested in contributing?

1.  Ensure you have Go (1.24+) installed.
2.  Clone the repository.
3.  Dependencies are managed with Go Modules.
4.  Build the project with `go build .`.
5.  Run tests with `go test ./...`.