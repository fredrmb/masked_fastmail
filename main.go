package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

// Version information
// (set via ldflags during build, or extracted from build info)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	shortCommitLength = 7 // length of short commit hash
)

// initVersionInfo attempts to populate version information from Go's build info
// when installed via `go install`. This is a fallback when ldflags are not set.
func initVersionInfo() {
	// If version was set via ldflags (not defaults), use those values
	if version != "dev" || commit != "none" || date != "unknown" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		// No build info available, try embedded version info as last resort
		checkEmbeddedVersionInfo()
		return
	}

	// Get version from module
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	// Get VCS info from build settings (only available when building from git repo)
	// VCS settings take priority over embedded info since they're always current
	hasVCSInfo := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if setting.Value != "" {
				if len(setting.Value) >= shortCommitLength {
					commit = setting.Value[:shortCommitLength]
				} else {
					commit = setting.Value
				}
				hasVCSInfo = true
			}
		case "vcs.time":
			if setting.Value != "" {
				// Parse and format the time
				if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
					date = t.Format("2006-01-02T15:04:05Z")
					hasVCSInfo = true
				}
			}
		}
	}

	// Only use embedded version info if VCS info is not available
	// (e.g., when installing from a remote module without git context)
	if !hasVCSInfo {
		checkEmbeddedVersionInfo()
	}
}

// checkEmbeddedVersionInfo checks for version info embedded by GitHub Actions
// during the release process. This is used when building from a remote module
// where VCS information is not available.
func checkEmbeddedVersionInfo() {
	if embeddedVersion != "" && embeddedVersion != "dev" {
		version = embeddedVersion
	}
	if embeddedCommit != "" && embeddedCommit != "none" {
		commit = embeddedCommit
	}
	if embeddedDate != "" && embeddedDate != "unknown" {
		date = embeddedDate
	}
}

// statePriority defines the precedence of alias states for selection
var statePriority = map[AliasState]int{
	AliasEnabled:  0,
	AliasPending:  1,
	AliasDisabled: 2,
	AliasDeleted:  3,
}

func main() {
	// Initialize version info from build info (fallback when ldflags aren't set)
	initVersionInfo()

	rootCmd := &cobra.Command{
		Use: `masked_fastmail <url>   (no flags)
  manage_fastmail <alias>`,
		Short: "Manage masked email aliases",
		Long: `A command-line tool to manage Fastmail.com masked email addresses.
Requires FASTMAIL_ACCOUNT_ID and FASTMAIL_API_KEY environment variables to be set.`,
		Example: `  # Create or get alias for a website:
  masked_fastmail example.com

  # Enable an existing alias:
  masked_fastmail --enable user.1234@fastmail.com`,

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			showVersion, _ := cmd.Flags().GetBool("version")
			if showVersion {
				fmt.Printf("Version:\t%s\nCommit:\t\t%s\nBuild date:\t%s\n", version, commit, date)
				return nil
			}
			return runMaskedFastmail(cmd, args)
		},
	}

	rootCmd.Flags().BoolP("version", "v", false, "show version information")
	rootCmd.Flags().BoolP("enable", "e", false, "enable alias")
	rootCmd.Flags().BoolP("disable", "d", false, "disable alias (send to trash)")
	rootCmd.Flags().Bool("delete", false, "delete alias (bounce messages)")
	rootCmd.Flags().Bool("debug", false, "enable debug output (shows raw API requests and responses)")

	// Make flags mutually exclusive
	rootCmd.MarkFlagsMutuallyExclusive("enable", "disable", "delete")

	// Add completion support
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// selectPreferredAlias selects the best alias based on state priority
// Priority order: enabled > pending > disabled > deleted
// Returns nil if the input slice is empty.
func selectPreferredAlias(aliases []MaskedEmailInfo) *MaskedEmailInfo {
	if len(aliases) == 0 {
		return nil
	}

	// Validate all states are recognized
	for _, alias := range aliases {
		if _, ok := statePriority[alias.State]; !ok {
			// Log warning but continue with known states
			fmt.Fprintf(os.Stderr, "Warning: unknown alias state: %s\n", alias.State)
		}
	}

	selected := &aliases[0]
	selectedPriority := statePriority[selected.State]

	for i := 1; i < len(aliases); i++ {
		priority := statePriority[aliases[i].State]
		if priority < selectedPriority {
			selected = &aliases[i]
			selectedPriority = priority
		}
	}

	return selected
}

// runMaskedFastmail is the main command handler for the CLI application.
// It handles both alias creation/lookup and state management operations.
func runMaskedFastmail(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one URL or alias must be specified\n\n%s", cmd.UsageString())
	}

	debug, _ := cmd.Flags().GetBool("debug")
	client, err := NewFastmailClient(debug)
	if err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	identifier := args[0]

	// Check for state update flags
	enable, _ := cmd.Flags().GetBool("enable")
	disable, _ := cmd.Flags().GetBool("disable")
	delete, _ := cmd.Flags().GetBool("delete")

	if enable || disable || delete {
		return handleStateUpdate(client, identifier, enable, disable, delete)
	}
	return handleAliasLookupOrCreation(client, identifier)
}

// handleStateUpdate manages the state changes of existing aliases
func handleStateUpdate(client *FastmailClient, identifier string, enable, disable, delete bool) error {
	email, err := normalizeEmailInput(identifier)
	if err != nil {
		return err
	}

	var newState AliasState
	switch {
	case enable:
		newState = AliasEnabled
	case disable:
		newState = AliasDisabled
	case delete:
		newState = AliasDeleted
	}

	// Get current state
	targetAlias, err := client.GetAliasByEmail(email)
	if err != nil {
		return formatAPIError("failed to get alias", err)
	}

	err = client.UpdateAliasStatus(targetAlias, newState)
	if err != nil {
		return formatAPIError("failed to update alias status", err)
	}
	return nil
}

// handleAliasLookupOrCreation handles alias lookup and creation if needed
func handleAliasLookupOrCreation(client *FastmailClient, identifier string) error {
	displayInput, normalizedDomain, err := prepareDomainInput(identifier)
	if err != nil {
		return err
	}

	aliases, err := client.GetAliases(normalizedDomain)
	if err != nil {
		return formatAPIError("failed to get aliases", err)
	}
	selectedAlias := selectPreferredAlias(aliases)

	if selectedAlias == nil {
		// Create new alias
		fmt.Printf("No alias found for %s, creating new one...\n", normalizedDomain)
		newAlias, err := client.CreateAlias(normalizedDomain, displayInput)
		if err != nil {
			return formatAPIError("failed to create alias", err)
		}
		selectedAlias = newAlias
	} else if len(aliases) > 1 {
		fmt.Printf("Found %d aliases for %s:\n", len(aliases), normalizedDomain)
		for _, alias := range aliases {
			fmt.Printf("- %s (state: %s)\n", alias.Email, alias.State)
		}
		fmt.Println("\nSelected alias:")
	}

	fmt.Printf("%s (state: %s)", selectedAlias.Email, selectedAlias.State)
	if err := copyToClipboard(selectedAlias.Email); err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: Could not copy to clipboard: %v\n", err)
	} else {
		fmt.Println(" (copied to clipboard)")
	}
	return nil
}

// copyToClipboard attempts to copy the given text to the system clipboard
func copyToClipboard(text string) error {
	if err := clipboard.WriteAll(text); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return nil
}

// formatAPIError augments Fastmail API errors with helpful context so users
// can understand failures without enabling debug mode.
func formatAPIError(action string, err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode > 0:
			body := strings.TrimSpace(apiErr.ResponseBody)
			if body == "" {
				body = apiErr.Message
			}
			return fmt.Errorf("%s: Fastmail API returned HTTP %d: %s", action, apiErr.StatusCode, body)
		case apiErr.Type != "":
			return fmt.Errorf("%s: Fastmail API error (%s): %s", action, apiErr.Type, apiErr.Message)
		default:
			return fmt.Errorf("%s: Fastmail API error: %s", action, apiErr.Message)
		}
	}
	return fmt.Errorf("%s: %w", action, err)
}
