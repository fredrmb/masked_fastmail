# Masked Fastmail

A simple CLI tool for managing [Fastmail masked email aliases](https://www.fastmail.com/features/masked-email/).

Easily create new aliases for websites and manage existing ones.

> This project is still under development and may contain bugs.
> It's a personal project and not in any way affiliated with Fastmail.

## Features

- Get or create masked email addresses for domains
- Aliases are automatically copied to clipboard
- Enable, disable and delete aliases

## Usage

![demo](https://raw.githubusercontent.com/fredrmb/masked_fastmail/main/demo.gif)

```text
Usage:
  masked_fastmail <url>   (no flags)
  manage_fastmail <alias> [flags]

Flags:
      --delete    delete alias (bounce messages)
  -d, --disable   disable alias (send to trash)
  -e, --enable    enable alias
  -h, --help      show this message
  -v, --version   show version information
```

The following environment variables must be set:

```shell
export FASTMAIL_ACCOUNT_ID=your_account_id
export FASTMAIL_API_KEY=your_api_key
```

### Examples

#### Get or create alias

A new alias will only be created if one does not already exist.
In either case, the alias is automatically copied to the clipboard.[^1]

[^1]: Copying is done with [Clipboard for Go](https://pkg.go.dev/github.com/atotto/clipboard#section-readme) and should work on all platforms.

```shell
masked_fastmail example.com
```

#### Enable an existing alias

New Fastmail aliases are initialized to `pending`, and are set to `enabled` once they receive their first email.
However, they get automatically deleted if no email is received within 24 hours.
Some services may not send a timely welcome email, in which case it's helpful to manually enable the alias.

```shell
masked_fastmail --enable user.1234@fastmail.com
```

#### Disable an alias

This causes all new new emails to be moved to trash.

```shell
masked_fastmail --disable user.1234@fastmail.com
```

## Installation

### Option 1: Download a pre-built binary

Download the latest release from the [releases page](https://github.com/fredrmb/masked_fastmail/releases/latest).

### Option 2: Use `go install`

```shell
go install github.com/fredrmb/masked_fastmail@latest
```

### Option 3: Build from source

1. Clone the repository
2. Run `go build -o masked_fastmail`

#### Prerequisites

- Go 1.22+
- Fastmail API credentials

#### API documentation

- The API documentation can be found at [https://www.fastmail.com/dev/](https://www.fastmail.com/dev/)
- It's also helpful to review the [JMAP protocol](https://jmap.io/crash-course.html)

## Development

### Prerequisites

- Go 1.22+ (see [Installation](#installation) for details)
- [VHS](https://github.com/charmbracelet/vhs) (for generating demo GIF)

### Building and running

Build the binary:

```shell
go build -o masked_fastmail
```

Run with debug output to see raw API requests and responses:

```shell
./masked_fastmail --debug example.com
```

### Generating demo GIF

The `demo.gif` is generated using [VHS](https://github.com/charmbracelet/vhs). Install VHS and run:

```shell
vhs cassette.tape
```

This will generate a new `demo.gif` file based on the commands in `cassette.tape`.

### Publishing a release

Releases are automatically built and published using [GoReleaser](https://goreleaser.com/) via GitHub Actions when a version tag is pushed.

To create a new release:

1. Create and push a version tag (must follow semantic versioning):
   ```shell
   git tag v1.2.3
   git push origin v1.2.3
   ```

2. The GitHub workflow will automatically:
   - Build binaries for Linux, Windows, and macOS (amd64 and arm64)
   - Create a GitHub release with the changelog
   - Attach all build artifacts to the release

Tag format: `vMAJOR.MINOR.PATCH` (e.g., `v1.0.0`). Pre-release tags are also supported (e.g., `v1.0.0-alpha`).

## License

BSD 3-Clause License
