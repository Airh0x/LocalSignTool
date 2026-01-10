# LocalSignTools

A simplified local-only version of SignTools for signing iOS applications. This version uses only the integrated builder, allowing all signing operations to run entirely on a single machine without requiring external builder servers.

## Overview

LocalSignTools is a streamlined local execution version of SignTools that includes only the integrated builder functionality. External builders and CI/CD services are not required, as all processing runs locally.

## Features

- **Integrated Builder**: Signing operations run internally (no separate server required)
- **Simple Configuration**: Only the integrated version is included, keeping the codebase clean
- **Automatic Port Management**: Automatically uses a random port if the default port is in use
- **Automatic Cleanup**: Automatically removes old upload files
- **2FA Support**: Supports Apple Developer Account two-factor authentication
- **Web Interface**: Easy-to-use browser-based interface

## Requirements

- macOS (required for iOS app signing)
- Go 1.24.0 or higher (for building)
- fastlane (required for signing operations)
- Python 3 (required for signing scripts)
- Node.js (required for signing scripts)

### Installing Dependencies

```bash
# Install fastlane
brew install fastlane
# or
gem install fastlane

# Python 3 and Node.js are typically included with macOS
```

## Setup

### 1. Build the Project

#### Option A: Build .app Bundle (Recommended)

```bash
cd LocalSignTools

# Download dependencies
go mod download

# Build .app bundle (includes building the executable)
./build_app.sh
```

This will create `SignTools.app` which can be double-clicked to launch.

**Note:** You only need to run `build_app.sh` when:
- You modify the source code (`main.go` or files in `src/`)
- You update dependencies (`go.mod`)
- You change the `.app` bundle structure or wrapper script

You do **NOT** need to rebuild when:
- Adding or modifying files in the `data/` folder (profiles, apps, uploads)
- Changing the `signer-cfg.yml` configuration file
- Modifying files in the `builder/` directory

The `data/` folder contents are loaded at runtime, so changes take effect immediately when you restart the application.

#### Option B: Build Executable Only

```bash
cd LocalSignTools

# Download dependencies
go mod download

# Build
go build -o SignTools
```

### 2. Prepare Configuration File

The `signer-cfg.yml` file will be automatically generated on first run. Edit it as needed:

```yaml
builder:
    integrated:
        enable: true
        sign_files_dir: ./builder
        entrypoint: sign.py
        job_timeout_mins: 15
server_url: http://localhost:8080
save_dir: data
cleanup_interval_mins: 5
sign_timeout_mins: 60
```

### 3. Set Up Signing Profiles

Create signing profiles in the `data/profiles/` directory.

#### Developer Account Profile

Place the following files in `data/profiles/developer_account/`:

- `cert.p12`: Certificate file
- `cert_pass.txt`: Certificate password
- `account_name.txt`: Apple Developer Account email address
- `account_pass.txt`: Apple Developer Account password
- `name.txt`: Developer name (e.g., "Tatsuya Kawabata")

#### Custom Provisioning Profile

Place the following files in `data/profiles/custom_profile/`:

- `cert.p12`: Certificate file
- `cert_pass.txt`: Certificate password
- `prov.mobileprovision`: Provisioning profile (required for custom profiles)
- `name.txt`: Profile name

**Note:** When using a custom provisioning profile, the system will:
- Skip App ID registration (uses the provisioning profile's App ID)
- Use the provisioning profile directly without generating a new one
- Support both server mode and CLI mode

**Important:** Do **NOT** include `account_name.txt` or `account_pass.txt` in a custom provisioning profile directory. If these files are present, the profile will be treated as a developer account profile instead.

### 4. Set Permissions for Sensitive Files

Restrict permissions on files containing sensitive information:

```bash
chmod 600 data/profiles/*/cert.p12
chmod 600 data/profiles/*/cert_pass.txt
chmod 600 data/profiles/*/account_name.txt
chmod 600 data/profiles/*/account_pass.txt
```

## Usage

### Method 1: Headless CLI Mode (Command Line)

Run signing operations directly from the command line without starting a web server:

```bash
./SignTools -headless \
  -ipa /path/to/app.ipa \
  -profile developer_account \
  -output /path/to/signed.ipa \
  -args "-a -d" \
  -bundle-id com.example.app
```

**CLI Mode Flags:**
- `-headless`: Enable CLI mode (required)
- `-ipa <path>`: Path to the IPA file to sign (required)
- `-profile <name>`: Profile name from `data/profiles/` (required)
- `-output <path>`: Output path for signed IPA (required)
- `-args "<args>"`: Optional signing arguments (e.g., `-a -d -m`)
  - `-a`: All devices
  - `-d`: Debug
  - `-m`: macOS
  - `-s`: File sharing
  - `-e`: Encode bundle ID
  - `-p`: Patch bundle ID
  - `-o`: Force original bundle ID
- `-bundle-id <id>`: Optional custom bundle ID
- `-builder <id>`: Optional builder ID (defaults to "Integrated")

**Example:**
```bash
# Basic signing
./SignTools -headless -ipa MyApp.ipa -profile developer_account -output MyApp-signed.ipa

# With signing arguments
./SignTools -headless -ipa MyApp.ipa -profile developer_account -output MyApp-signed.ipa -args "-a -d"

# With custom bundle ID
./SignTools -headless -ipa MyApp.ipa -profile developer_account -output MyApp-signed.ipa -bundle-id com.custom.bundle

# Using custom provisioning profile
./SignTools -headless -ipa MyApp.ipa -profile custom_profile -output MyApp-signed.ipa
```

**Note:** In CLI mode, if 2FA is required, fastlane will prompt you directly on the command line. Make sure your terminal is interactive to enter the 2FA code.

### Method 2: Using .app Bundle (Recommended for Double-Click Launch)

1. Build the .app bundle:
```bash
./build_app.sh
```

2. Double-click `SignTools.app` in Finder, or run:
```bash
open SignTools.app
```

The application will launch in a Terminal window. The web interface will be available at `http://localhost:8080` (or a random port if 8080 is in use).

#### Running CLI Mode from .app Bundle

You can also run CLI mode directly from the `.app` bundle:

```bash
# Method 1: Using the wrapper script (recommended)
SignTools.app/Contents/MacOS/SignTools \
  -headless \
  -ipa /path/to/app.ipa \
  -profile developer_account \
  -output /path/to/signed.ipa \
  -args "-a -d"

# Method 2: Using the binary directly
SignTools.app/Contents/MacOS/SignTools.bin \
  -headless \
  -ipa /path/to/app.ipa \
  -profile developer_account \
  -output /path/to/signed.ipa
```

**Note:** When using CLI mode from the `.app` bundle, the wrapper script will detect the `-headless` flag and run the command directly in your current terminal (without opening a new Terminal window). For server mode (without `-headless`), a new Terminal window will be opened.

### Method 3: Web Server Mode (Traditional)

Start the web server and use the browser interface:

```bash
./SignTools
# or with custom port
./SignTools -port 8080
```

### Access Web Interface

Open the following URL in your browser:

```
http://localhost:8080
```

If port 8080 is in use, a random port will be automatically assigned. Check the startup logs to see the actual port number.

## CLI Mode Details

### Signing Arguments

The `-args` flag accepts the following signing arguments:

- `-a`: Enable for all devices (Ad Hoc distribution)
- `-d`: Enable debug mode
- `-m`: Enable macOS support
- `-s`: Enable file sharing
- `-e`: Encode bundle ID
- `-p`: Patch bundle ID
- `-o`: Force original bundle ID

Multiple arguments can be combined: `-args "-a -d -m"`

### 2FA in CLI Mode

When 2FA is required in CLI mode:
1. Fastlane will prompt you directly on the command line for the 2FA code
2. Enter the code when prompted
3. The signing process will continue automatically

**Important:** Make sure your terminal is interactive (not running as a background job) to enter 2FA codes.

**Note:** 2FA-related log messages are now displayed only once per signing operation to reduce log noise. The actual 2FA prompt from fastlane will still appear as needed.

### Exit Codes

- `0`: Signing completed successfully
- Non-zero: Signing failed (check error messages)

## Two-Factor Authentication (2FA)

When 2FA is enabled on your Apple Developer Account, you will be prompted to enter a 2FA code during signing.

### Entering 2FA Code

1. When you start a signing job, a 2FA input screen will appear in the web interface
2. Enter the 2FA code sent by Apple
3. The signing process will continue

### Troubleshooting 2FA

If you're not receiving 2FA codes, use the included debugging scripts:

```bash
# Check 2FA configuration and system status
./check_2fa.sh

# Test fastlane authentication directly (interactive)
./debug_2fa.sh
```

**`check_2fa.sh`** checks:
- Builder server status
- SignTools service status
- Builder logs for 2FA-related messages
- Signing profile configuration

**`debug_2fa.sh`** performs:
- Profile file validation
- fastlane installation check
- Direct fastlane authentication test (will prompt for 2FA code)

Things to check:
- Is the email address in `account_name.txt` correct?
- Is 2FA enabled on your Apple Developer Account?
- Is your account locked?
- Are you receiving 2FA codes within 60 seconds?

## File Management

### Automatic Cleanup

- **Upload Files**: Files older than 60 minutes are automatically deleted
- **Jobs**: Automatically deleted after timeout
- Cleanup runs on startup and periodically (every 5 minutes)

### Manual Cleanup

```bash
# Delete old files in uploads directory
find data/uploads -type f -mtime +1 -delete
```

## Troubleshooting

### Port Already in Use

If the default port (8080) is in use, a random port will be automatically assigned. Check the startup logs to see the actual port number.

### Signing Fails

1. Verify that fastlane is correctly installed
2. Check that certificates and profiles are correctly configured
3. Check the logs (use `-log-level 0` for detailed logs)

```bash
./SignTools -log-level 0
```

**Improved Error Messages:**
- App ID registration errors now show clear messages with suggested solutions
- If an App ID already exists, the process continues automatically
- App ID creation failures include detailed error information
- Provisioning profile errors provide specific guidance

**Common Issues:**
- **App ID not found**: The system will attempt to create it automatically. If creation fails, check your Apple Developer account permissions.
- **Provisioning profile not found**: For custom profiles, ensure `prov.mobileprovision` exists in the profile directory.
- **Certificate issues**: Verify that `cert.p12` and `cert_pass.txt` are correct and accessible.

### 2FA Codes Not Received

1. Verify the email address in `account_name.txt`
2. Test fastlane authentication with `debug_2fa.sh`
3. Check your Apple Developer Account settings

## Directory Structure

```
LocalSignTools/
├── SignTools                 # Main executable (built)
├── SignTools.app            # macOS .app bundle (built)
├── build_app.sh             # Script to build .app bundle
├── debug_2fa.sh             # 2FA debugging script
├── check_2fa.sh             # 2FA troubleshooting script
├── signer-cfg.yml           # Configuration file
├── data/                    # Data directory
│   ├── apps/               # Uploaded applications
│   ├── profiles/           # Signing profiles
│   │   └── developer_account/  # Example profile
│   └── uploads/            # Temporary upload files
├── builder/                # Signing scripts
│   ├── sign.py             # Main signing script
│   ├── node-utils/         # Node.js utilities
│   └── lib*/               # Library files
└── src/                     # Source code
    ├── assets/             # Web assets and templates
    ├── builders/           # Builder implementations
    ├── config/             # Configuration management
    ├── server/             # Server utilities
    ├── signing/            # Signing logic (CLI mode)
    ├── storage/            # Storage management
    ├── tunnel/             # Tunnel providers (ngrok, cloudflare)
    └── util/               # Utility functions
```

## Configuration Details

### Main Settings

- `cleanup_interval_mins`: Cleanup execution interval (minutes)
- `sign_timeout_mins`: Signing timeout (minutes)
- `server_url`: Server URL
- `save_dir`: Data storage directory

### Builder Settings

- `integrated.enable`: Enable/disable integrated builder (always true)
- `integrated.sign_files_dir`: Directory containing signing scripts
- `integrated.entrypoint`: Entry point script
- `integrated.job_timeout_mins`: Job timeout (minutes)

## Security

- Set permissions to `600` for sensitive files (certificates, passwords)
- Designed for local environment use
- Enable Basic authentication if needed

```yaml
basic_auth:
    enable: true
    username: admin
    password: YOUR_PASSWORD
```

## Differences from Original SignTools

LocalSignTools is a simplified version of SignTools with the following features removed:

- GitHub Actions Builder
- Semaphore CI Builder
- Self-hosted Builder (external server)

Only the integrated builder is supported, optimized for local environment use.

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

See the `LICENSE` file for the full license text.

### License Summary

- ✅ **Commercial use**: Allowed
- ✅ **Modification**: Allowed
- ✅ **Distribution**: Allowed
- ✅ **Private use**: Allowed
- ✅ **Patent use**: Allowed
- ❌ **Sublicensing**: Not allowed
- ❌ **Liability**: No warranty provided
- ⚠️ **Copyleft**: Modifications must be released under the same license
- ⚠️ **Network use**: If you modify and run on a server, you must provide source code to users

This project is based on [SignTools](https://github.com/SignTools/SignTools), which is also licensed under AGPL-3.0.

## Special Thanks

This project is based on [SignTools](https://github.com/SignTools/SignTools), an open-source iOS app signing tool. LocalSignTools is a simplified local-only version that maintains compatibility with the original project's signing infrastructure while focusing on ease of use for local development.

We would like to express our gratitude to the original SignTools project and its contributors for creating an excellent foundation for iOS app signing workflows.

## Recent Improvements

### CLI Mode Enhancements

- **Headless CLI Mode**: Complete signing operations via command line without starting a web server
- **Custom Provisioning Profile Support**: Full support for custom provisioning profiles in CLI mode
- **Improved Error Handling**: Better error messages for App ID registration and provisioning profile operations
- **Reduced Log Noise**: 2FA-related log messages are now displayed only once per operation

### Code Quality

- **Refactoring**: Eliminated code duplication (e.g., `setBuilderSecrets` function)
- **Better Organization**: Clearer separation of concerns between server mode and CLI mode
- **Improved Documentation**: Enhanced README with detailed usage examples

### Developer Tools

- **Debug Scripts**: Added `debug_2fa.sh` and `check_2fa.sh` for troubleshooting 2FA issues
- **App Bundle Support**: CLI mode works seamlessly from the `.app` bundle

## Contributing

This is a local-only version focused on simplicity. For contributions to the main SignTools project, please refer to the [original repository](https://github.com/SignTools/SignTools).
