# Masked Fastmail

A simple CLI tool for managing [Fastmail masked email aliases](https://www.fastmail.com/features/masked-email/).

Easily create new aliases for websites and manage existing ones.

> [!NOTE]
> This is a personal project and is not in any way affiliated with Fastmail.com.

## Features

- Get or create masked email addresses for domains
- Aliases are automatically copied to clipboard
- Enable, disable and delete aliases
- List existing aliases for a domain without creating new ones

## Usage

![demo](https://raw.githubusercontent.com/fredrmb/masked_fastmail/main/demo.gif)

```text
Usage:
  masked_fastmail <url> "description"	(description is optional)
  manage_fastmail <alias> [flags]

Flags:
      --delete    delete alias (bounce messages)
  -d, --disable   disable alias (send to trash)
  -e, --enable    enable alias
  -l, --list      list aliases for a domain without creating anything
      --set-description string
                   update the description for an existing alias
  -h, --help      show this message
  -v, --version   show version information
```

See more [usage examples](#examples) below.

The following environment variables must be set:

```shell
export FASTMAIL_ACCOUNT_ID=your_account_id
export FASTMAIL_API_KEY=your_api_key
```

## Installation

### Option 1: Download a pre-built binary

Download the latest release from the [releases page](https://github.com/fredrmb/masked_fastmail/releases/latest).

### Option 2: Use `go install`

```shell
go install github.com/fredrmb/masked_fastmail@latest
```

You can verify the installation and check the version by running:

```shell
masked_fastmail --version
```

The binary will be installed to `$GOBIN` (or `$GOPATH/bin`, or `~/go/bin` if neither is set). Make sure this directory is in your `PATH`.

### Option 3: Build from source

1. Clone the repository
2. Run `./build.sh` (includes version information) or `go build -o masked_fastmail`

#### Prerequisites

- Go 1.22+
- Fastmail API credentials

See [DEVELOPMENT.md](./DEVELOPMENT.md) for more information about building, running and using this code.

## Examples

### Get or create alias

A new alias will only be created if one does not already exist.
In either case, the alias is automatically copied to the clipboard.[^1]

[^1]: Copying is done with [Clipboard for Go](https://pkg.go.dev/github.com/atotto/clipboard#section-readme) and should work on all platforms.

```shell
masked_fastmail example.com
```

You can optionally supply a description. Quotes are passed straight through the shell, so all characters are preserved:

```shell
masked_fastmail example.com "Shopping account at Example"
```

Descriptions supplied with an existing alias will be ignored to avoid accidental overwrites.

Use `--set-description` if you intend to update an existing alias. See [example below](#update-an-alias-description).

### Enable an existing alias

New Fastmail aliases are initialized to `pending`, and are set to `enabled` once they receive their first email.
However, they get automatically deleted if no email is received within 24 hours.
Some services may not send a timely welcome email, in which case it's helpful to manually enable the alias.

```shell
masked_fastmail --enable user.1234@fastmail.com
```

### Disable an alias

This causes all new new emails to be moved to trash.

```shell
masked_fastmail --disable user.1234@fastmail.com
```

### Delete an alias

This causes all new emails to bounce.

```shell
masked_fastmail --delete user.1234@fastmail.com
```

### List aliases for a domain

Prints all known aliases for the site without creating a new one or copying to the clipboard. Results whose `forDomain` matches the normalized input are listed first, followed by aliases where the search text appears in the `email`, `description`, or `forDomain` fields.

```shell
masked_fastmail --list example.com
```

### Update an alias description

Descriptions can only be updated explicitly to avoid accidental changes. Pass the alias email plus the new description:

```shell
masked_fastmail user.1234@fastmail.com --set-description "Personal finance login"
```

### How domains are normalized

When you pass a URL or domain, the CLI normalizes it before talking to Fastmail:

- Paths, query strings, ports, and fragments are dropped (only scheme + host remain)
- `https://` is assumed if you omit the scheme; `http://` is preserved if you specify it
- Host names are lower-cased and trailing dots/slashes are removed
  - In other words, `https://example.com`, `example.com`, `https://EXAMPLE.com/login` and `example.com/login` are all treated as equal
- Subdomains stay distinct (`shop.example.com` is different from `example.com`)

The normalized value is stored in Fastmail's `forDomain` field. The `description` field is only populated with text you explicitly (and optionally) provide.


## License

BSD 3-Clause License
