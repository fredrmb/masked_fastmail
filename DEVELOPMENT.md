## Development

### Prerequisites

- Go 1.22+
- Fastmail API credentials
- [VHS](https://github.com/charmbracelet/vhs) (for generating demo GIF)

### Building and running

Build the binary:

```shell
./build.sh
```

Or build without version information:

```shell
go build -o masked_fastmail
```

The `build.sh` script automatically sets version information from git (version tag, commit hash, and build date), which will be displayed when running `./masked_fastmail --version`.

### Run with debug output

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

#### Note about `go install` and version info

When installed via `go install`, version information is automatically embedded in the source code as part of the release process. This means `go install github.com/fredrmb/masked_fastmail@latest` will include:
- **Version**: Release tag (e.g., `v1.2.3`)
- **Commit**: Short commit hash of the tagged release
- **Build date**: Timestamp when the release was created

The version information is extracted using the following priority:
1. **Ldflags** (from `build.sh` or GoReleaser) - takes precedence if set
2. **VCS build settings** - commit/date when building from git repo (always current for local builds)
3. **Go build info** - version tag from module metadata
4. **Embedded version info** (from `version_info.go`, updated during release workflow) - used when VCS info is unavailable (e.g., `go install` from remote module)

### API documentation

- The API documentation can be found at [https://www.fastmail.com/dev/](https://www.fastmail.com/dev/)
- It's also helpful to review the [JMAP protocol](https://jmap.io/crash-course.html)
