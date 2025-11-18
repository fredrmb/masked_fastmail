package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

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
		Use: `masked_fastmail <url> "description"	(description is optional)
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
	rootCmd.Flags().BoolP("list", "l", false, "list all aliases for a domain without creating new ones")
	rootCmd.Flags().String("set-description", "", "update the description for an alias")

	// Make flags mutually exclusive
	rootCmd.MarkFlagsMutuallyExclusive("enable", "disable", "delete")
	rootCmd.MarkFlagsMutuallyExclusive("list", "enable", "disable", "delete", "set-description")
	rootCmd.MarkFlagsMutuallyExclusive("set-description", "enable", "disable", "delete")

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
	selectedPriority := getStatePriority(selected.State)

	for i := 1; i < len(aliases); i++ {
		priority := getStatePriority(aliases[i].State)
		if priority < selectedPriority {
			selected = &aliases[i]
			selectedPriority = priority
		}
	}

	return selected
}

func getStatePriority(state AliasState) int {
	if priority, ok := statePriority[state]; ok {
		return priority
	}
	return math.MaxInt
}

// runMaskedFastmail is the main command handler for the CLI application.
// It handles both alias creation/lookup and state management operations.
func runMaskedFastmail(cmd *cobra.Command, args []string) error {
	if len(args) == 0 || len(args) > 2 {
		return fmt.Errorf("specify a domain/alias, optionally followed by a description\n\n%s", cmd.UsageString())
	}

	debug, _ := cmd.Flags().GetBool("debug")
	client, err := NewFastmailClient(debug)
	if err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	identifier := args[0]
	var descriptionArg *string
	if len(args) == 2 {
		desc := args[1]
		descriptionArg = &desc
	}

	// Check for state update flags
	enable, _ := cmd.Flags().GetBool("enable")
	disable, _ := cmd.Flags().GetBool("disable")
	delete, _ := cmd.Flags().GetBool("delete")
	list, _ := cmd.Flags().GetBool("list")
	newDescriptionValue, _ := cmd.Flags().GetString("set-description")
	setDescription := cmd.Flags().Changed("set-description")

	requiresSingleArg := enable || disable || delete || list || setDescription
	if requiresSingleArg && len(args) != 1 {
		return fmt.Errorf("this operation accepts exactly one identifier (alias or domain)")
	}
	if descriptionArg != nil && requiresSingleArg {
		return fmt.Errorf("the positional description argument is only allowed when creating or looking up aliases without flags")
	}

	if setDescription {
		return handleDescriptionUpdate(client, identifier, newDescriptionValue)
	}

	if enable || disable || delete {
		return handleStateUpdate(client, identifier, enable, disable, delete)
	}
	if list {
		return handleAliasList(client, identifier)
	}
	return handleAliasLookupOrCreation(client, identifier, descriptionArg)
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

// handleAliasList prints metadata for all aliases associated with a domain
// without creating or modifying anything.
func handleAliasList(client *FastmailClient, identifier string) error {
	displayInput, normalizedDomain, err := prepareDomainInput(identifier)
	if err != nil {
		return err
	}

	aliases, err := client.FetchAllAliases()
	if err != nil {
		return formatAPIError("failed to list aliases", err)
	}

	matching, related := filterAliasesForList(aliases, normalizedDomain, displayInput)
	if len(matching) == 0 && len(related) == 0 {
		fmt.Printf("No aliases found matching %s\n", displayInput)
		return nil
	}

	type aliasRow struct {
		email       string
		state       string
		url         string
		description string
	}

	buildRows := func(in []MaskedEmailInfo) []aliasRow {
		rows := make([]aliasRow, 0, len(in))
		for _, alias := range in {
			description := alias.Description
			if strings.TrimSpace(description) == "" {
				description = "(no description)"
			}
			url := strings.TrimSpace(alias.ForDomain)
			if url == "" {
				url = "(unknown domain)"
			}
			rows = append(rows, aliasRow{
				email:       alias.Email,
				state:       string(alias.State),
				url:         url,
				description: description,
			})
		}
		return rows
	}

	matchingRows := buildRows(matching)
	relatedRows := buildRows(related)
	allRows := append(append([]aliasRow{}, matchingRows...), relatedRows...)
	maxEmailWidth := 0

	for _, row := range allRows {
		if emailWidth := utf8.RuneCountInString(row.email); emailWidth > maxEmailWidth {
			maxEmailWidth = emailWidth
		}
	}

	firstLineFormat := fmt.Sprintf("- %%-%ds (state: %%s)\n", maxEmailWidth)
	printRows := func(rows []aliasRow, includeURL bool) {
		for idx, row := range rows {
			fmt.Printf(firstLineFormat, row.email, row.state)
			if includeURL {
				domainLabel := strings.TrimSpace(row.url)
				if domainLabel == "" {
					domainLabel = "(unknown domain)"
				}
				fmt.Printf("  Domain:      %s\n", domainLabel)
			}
			fmt.Printf("  Description: %s\n", row.description)
			if idx < len(rows)-1 {
				fmt.Println()
			}
		}
	}

	if len(matchingRows) == 0 {
		fmt.Printf("No aliases found for domain %s\n", normalizedDomain)
	} else {
		fmt.Printf("Aliases for %s:\n", normalizedDomain)
		printRows(matchingRows, false)
	}

	if len(relatedRows) > 0 {
		if len(matchingRows) > 0 {
			fmt.Println()
		} else {
			fmt.Println()
		}
		fmt.Printf("Additional matches containing %q:\n", strings.TrimSpace(displayInput))
		printRows(relatedRows, true)
	}

	return nil
}

// handleAliasLookupOrCreation handles alias lookup and creation if needed
func handleAliasLookupOrCreation(client *FastmailClient, identifier string, description *string) error {
	_, normalizedDomain, err := prepareDomainInput(identifier)
	if err != nil {
		return err
	}

	aliases, err := client.GetAliases(normalizedDomain)
	if err != nil {
		return formatAPIError("failed to get aliases", err)
	}
	selectedAlias := selectPreferredAlias(aliases)

	createdNew := false
	if selectedAlias == nil {
		// Create new alias
		fmt.Printf("No alias found for %s, creating new one...\n", normalizedDomain)
		newAlias, err := client.CreateAlias(normalizedDomain, description)
		if err != nil {
			return formatAPIError("failed to create alias", err)
		}
		selectedAlias = newAlias
		createdNew = true
	} else if len(aliases) > 1 {
		fmt.Printf("Found %d aliases for %s:\n", len(aliases), normalizedDomain)
		for _, alias := range aliases {
			fmt.Printf("- %s (state: %s)\n", alias.Email, alias.State)
		}
		fmt.Println("\nSelected alias:")
	}

	if description != nil && !createdNew {
		trimmed := strings.TrimSpace(*description)
		if trimmed != "" {
			fmt.Fprintf(os.Stderr, "Note: description not updated for existing alias. Use --set-description to change it.\n")
		}
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

// handleDescriptionUpdate updates the description for an existing alias identified by email.
func handleDescriptionUpdate(client *FastmailClient, identifier string, newDescription string) error {
	email, err := normalizeEmailInput(identifier)
	if err != nil {
		return fmt.Errorf("--set-description requires an alias email address: %w", err)
	}

	alias, err := client.GetAliasByEmail(email)
	if err != nil {
		return formatAPIError("failed to get alias", err)
	}

	if alias.Description == newDescription {
		fmt.Println("Description already set to the requested value.")
		return nil
	}

	if err := client.UpdateAliasDescription(alias, newDescription); err != nil {
		return formatAPIError("failed to update alias description", err)
	}

	fmt.Println("Description updated.")
	return nil
}

// filterAliasesForList splits aliases into primary (forDomain matches) and related (search matches).
func filterAliasesForList(aliases []MaskedEmailInfo, normalizedDomain string, searchInput string) (primary []MaskedEmailInfo, related []MaskedEmailInfo) {
	needleDomain := strings.ToLower(strings.TrimSpace(normalizedDomain))
	needleSearch := strings.ToLower(strings.TrimSpace(searchInput))
	seen := make(map[string]struct{})

	for _, alias := range aliases {
		if alias.State == AliasDeleted {
			continue
		}

		if aliasMatchesDomain(alias, normalizedDomain) {
			primary = append(primary, alias)
			if alias.ID != "" {
				seen[alias.ID] = struct{}{}
			}
			continue
		}

		if aliasMatchesSubdomain(alias, normalizedDomain) {
			if alias.ID != "" {
				if _, ok := seen[alias.ID]; ok {
					continue
				}
				seen[alias.ID] = struct{}{}
			}
			related = append(related, alias)
			continue
		}

		if aliasMatchesSearch(alias, needleDomain, needleSearch) {
			if alias.ID != "" {
				if _, ok := seen[alias.ID]; ok {
					continue
				}
				seen[alias.ID] = struct{}{}
			}
			related = append(related, alias)
		}
	}

	return primary, related
}

func aliasMatchesSearch(alias MaskedEmailInfo, needles ...string) bool {
	fields := []string{
		strings.ToLower(alias.Email),
		strings.ToLower(alias.Description),
		strings.ToLower(alias.ForDomain),
		strings.ToLower(alias.ID),
	}

	for _, needle := range needles {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		for _, field := range fields {
			if field != "" && strings.Contains(field, needle) {
				return true
			}
		}
	}

	return false
}

func aliasMatchesSubdomain(alias MaskedEmailInfo, targetDomain string) bool {
	targetHost := hostFromOrigin(targetDomain)
	if targetHost == "" {
		return false
	}

	candidate := alias.ForDomain
	if strings.TrimSpace(candidate) == "" {
		candidate = alias.Description
	}

	aliasHost := hostFromOrigin(candidate)
	if aliasHost == "" {
		return false
	}

	return isSubdomain(aliasHost, targetHost)
}
