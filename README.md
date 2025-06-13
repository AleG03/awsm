# `awsm` - The AWS Manager

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> A fast and fancy command-line tool to manage your AWS profiles, sessions, and console access with ease.

`awsm` (AWS Manager) is a standalone binary written in Go that simplifies the tedious process of switching between AWS profiles, handling MFA, assuming roles, and generating console sign-in links. It's a robust and portable replacement for shell function-based helpers.

## Features

-   🚀 **Standalone & Portable:** A single binary with no runtime dependencies.
-   ✨ **Fancy Interface:** Clean, colorized output and formatted tables make it easy to read.
-   🧩 **Intelligent Credential Handling:** Natively supports both **IAM** (MFA/role assumption) and **AWS SSO** (IAM Identity Center) profiles.
-   ⚡️ **Fast & Reliable:** Written in Go, it's significantly faster and more robust than complex shell scripts.
-   🧠 **Powerful Autocompletion:** Press `<TAB>` to complete commands, flags, and even your personal AWS profile names.
-   🌐 **Console Login:** Quickly generate a federated sign-in URL to open the AWS Console with your current CLI session.
-   🛠️ **Safe & Contained Execution:** Run a single command under a specific profile without cluttering your main shell session using awsm exec.

## Table of Contents

- [`awsm` - The AWS Manager](#awsm---the-aws-manager)
  - [Features](#features)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
    - [\[Work In Progess\] Option 1: From Binaries (Recommended)](#work-in-progess-option-1-from-binaries-recommended)
    - [\[Use this for now\] Option 2: Building from Source](#use-this-for-now-option-2-building-from-source)
  - [Shell Configuration (Crucial)](#shell-configuration-crucial)
    - [Step 1: Creating the Alias and enable refreshing credentials](#step-1-creating-the-alias-and-enable-refreshing-credentials)
    - [Step 2: Enabling Autocompletion (Recommended)](#step-2-enabling-autocompletion-recommended)
      - [For Zsh](#for-zsh)
      - [For Bash](#for-bash)
  - [Core Concept: Why `acp` Uses `export` and `eval`](#core-concept-why-acp-uses-export-and-eval)
  - [Usage Guide](#usage-guide)
    - [Working with AWS SSO Profiles](#working-with-aws-sso-profiles)
    - [Working with IAM Profiles](#working-with-iam-profiles)
    - [Switch AWS Region](#switch-aws-region)
    - [Executing a Single Command](#executing-a-single-command)
    - [AWS Console Login](#aws-console-login)
    - [AWS SSO Profiles Generator](#aws-sso-profiles-generator)
    - [Listing Profiles \& Regions](#listing-profiles--regions)
    - [Full Command Reference](#full-command-reference)
  - [Development](#development)

---

## Installation

### [Work In Progess] Option 1: From Binaries (Recommended)

_Once binaries are available, find them on the [Releases Page](https://github.com/AleG03/awsm/releases)._

1.  Download the appropriate binary for your operating system and architecture.
2.  Move the binary to a directory in your `PATH`.
    ```bash
    # Move the downloaded file and make it executable
    mv ~/Downloads/awsm /usr/local/bin/awsm
    chmod +x ~/.local/bin/awsm
    ```
3.  Proceed to the [Shell Configuration](#shell-configuration-crucial) section below.

### [Use this for now] Option 2: Building from Source

If you have Go (1.18+) installed, you can build `awsm` from source.

1.  [Work in Progress] **Clone the repository:** 
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

Because a program cannot change the environment of its parent shell, we need to use a shell function and `eval` to export the AWS credentials. This `acp` (Assume/Activate Cloud Profile) alias makes the process seamless. Moreover, the `asr` alias makes easier to switch region on the fly.

1.  Open your shell's configuration file (`~/.zshrc` or `~/.bashrc`).
2.  Add the following function **after** the PATH export:

    ```sh
    acp() {
    if [[ -z "$1" ]]; then
        unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_PROFILE;
        echo "AWS session cleared.";
        return;
    fi;
    eval $(awsm export "$1");
    }

    # Helper for switching AWS regions
    asr() {
    # If called with no argument, clear the region override
    if [[ -z "$1" ]]; then
        unset AWS_REGION AWS_DEFAULT_REGION;
        echo "AWS Region override cleared.";
        return;
    fi;
    eval $(awsm region set "$1");
    }

    # A "smart" wrapper for the aws command that automatically refreshes expired tokens.
    aws() {
    # If we don't have an AWS_PROFILE set, there's nothing to refresh.
    # Just run the command and exit.
    if [[ -z "$AWS_PROFILE" ]]; then
        command aws "$@"
        return
    fi

    # Run the real `aws` command, capturing all output (stdout and stderr)
    # into a variable. We also capture the exit code.
    local output
    local exit_code
    output=$(command aws "$@" 2>&1)
    exit_code=$?

    # Check if the output contains the classic "token expired" error messages.
    # The -E flag allows for extended regex (using | for OR).
    # The -q flag makes grep silent; we only care about its exit code.
    if echo "$output" | grep -q -E 'ExpiredToken|token.*expired'; then
        # The token is expired! Time for some magic.
        echo -e "\033[33mAWS token expired. Refreshing session for '$AWS_PROFILE'...\033[0m" >&2

        # Call our acp helper to refresh the credentials.
        if acp "$AWS_PROFILE"; then
        echo -e "\033[32mSession refreshed. Retrying original command...\033[0m" >&2
        # Re-run the original command now that the session is fresh.
        command aws "$@"
        else
        # If `acp` failed (e.g., bad MFA), show the original error.
        echo -e "\033[31mFailed to refresh session. Original error:\033[0m" >&2
        echo "$output"
        return $exit_code
        fi
    else
        # The command ran without an expiration error.
        # Print its original output and return its original exit code.
        echo "$output"
        return $exit_code
    fi
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

## Core Concept: Why `acp` Uses `export` and `eval`

You might be thinking: *"Why use the `acp` function? Why not just run `awsm export my-profile` directly?"*

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

The `acp` function just wraps this pattern in a convenient, memorable shortcut.

---

## Usage Guide

### Working with AWS SSO Profiles

This is the modern, recommended way to use AWS. The flow is two steps: log in once, then activate the profile as needed.

1.  **Log in to your SSO session:**
    This is a one-time action per session (e.g., once a day). `awsm` will open your browser for you to authenticate.
    ```bash
    $ awsm sso login your-sso-profile-name
    Attempting SSO login for session: my-sso-session-name
    Your browser should open...
    ✔ SSO login successful.
    ```
2.  **Activate the profile:**
    Use the `acp` alias to get temporary credentials into your shell.
    ```bash
    $ acp your-sso-profile-name
    SSO profile detected. Using cached session to get credentials...
    ✔ Credentials for profile 'your-sso-profile-name' are set.

    # Now you can use any AWS command
    $ aws sts get-caller-identity
    ```

### Working with IAM Profiles

This flow is for legacy profiles that use an IAM user's `role_arn` and `mfa_serial`.

1.  **Activate the profile and get credentials:**
    The `acp` command will prompt you for your MFA token.
    ```bash
    $ acp your-iam-profile-name
    IAM profile detected. Using STS to get credentials...
    Enter MFA token for arn:aws:iam::...: 123456
    ✔ Credentials for profile 'your-iam-profile-name' are set.
    ```

### Switch AWS Region

If you need to switch Region after setting a profile, use `asr` command.
```bash
asr eu-west-1
```
When you're done, you can clear the override to go back to your profile's default
```bash
asr
```


### Executing a Single Command

If you only need to run one command with a specific profile, `awsm exec` is a safer alternative that doesn't modify your current shell's environment.

```bash
$ awsm exec your-profile-name -- aws s3 ls my-bucket
```

### AWS Console Login
This feature allows you to generate an AWS Console URL using the current CLI session credentials. You can use the following commands:

1. First, make sure you have an active session
```bash
acp my-dev-profile
```
1. Then, beam to the console! It opens your browser by default.
```bash
awsm console
```
If you only want the URL without opening the browser:
```bash
awsm console --no-open
```

### AWS SSO Profiles Generator
This feature generates AWS CLI SSO profiles for each account and role. It creates a configuration file with profiles for all accounts and roles accessible via AWS SSO.

Usage:

- `awsm sso generate [sso-session] [aws-region]`: Generates the profiles and writes them to `~/.aws/aws_sso_profiles.conf`. You can copy the contents of this file into your `~/.aws/config` if desired.

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
├── exec          Execute a command with temporary credentials
├── export        (Plumbing) Export temporary credentials for a profile
├── profile
│   └── list      List all available AWS profiles
├── region
│   └── list      List all available AWS regions
├── sso
│   ├── login     Log in to an AWS SSO session
│   └── generate  Generate profiles for all accessible SSO accounts/roles
└── completion    Generate shell autocompletion scripts
```

## Development

Interested in contributing?

1.  Ensure you have Go (1.18+) installed.
2.  Clone the repository.
3.  Dependencies are managed with Go Modules.
4.  Build the project with `go build .`.
5.  Run tests with `go test ./...`.