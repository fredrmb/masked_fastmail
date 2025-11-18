package main

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	defaultScheme = "https"
)

// normalizeOrigin converts a user-supplied URL or domain into a canonical origin
// string consisting of "<scheme>://<host>". Paths, queries, ports, fragments,
// and casing differences are removed. If the input lacks a scheme, https is
// assumed. Subdomains are preserved so that different subdomains remain unique.
func normalizeOrigin(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("domain cannot be empty")
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = defaultScheme + "://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to parse domain %q: %w", input, err)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("invalid domain %q: missing host", input)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "" {
		scheme = defaultScheme
	}

	host = strings.TrimSuffix(strings.ToLower(host), ".")

	return fmt.Sprintf("%s://%s", scheme, host), nil
}

// domainsEqual compares two domain strings by normalizing them, ignoring any
// errors from normalization by falling back to a case-insensitive comparison
// without trailing slashes.
func domainsEqual(a, b string) bool {
	na, errA := normalizeOrigin(a)
	nb, errB := normalizeOrigin(b)
	if errA == nil && errB == nil {
		return na == nb
	}

	// Fallback: compare trimmed strings case-insensitively
	trimA := strings.TrimRight(strings.ToLower(strings.TrimSpace(a)), "/")
	trimB := strings.TrimRight(strings.ToLower(strings.TrimSpace(b)), "/")
	return trimA == trimB
}

// prepareDomainInput trims the user-provided identifier, ensures it is a domain
// (not an email address), and returns both the trimmed display value and the
// normalized domain used for API calls.
func prepareDomainInput(input string) (string, string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", fmt.Errorf("domain cannot be empty")
	}
	if looksLikeEmail(trimmed) {
		return "", "", fmt.Errorf("expected a domain but received an email address: %s", trimmed)
	}

	normalized, err := normalizeOrigin(trimmed)
	if err != nil {
		return "", "", err
	}
	return trimmed, normalized, nil
}

// normalizeEmailInput trims and validates an email identifier.
func normalizeEmailInput(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("alias email cannot be empty")
	}
	if !looksLikeEmail(trimmed) {
		return "", fmt.Errorf("expected an alias email address, got %s", trimmed)
	}
	return trimmed, nil
}

func looksLikeEmail(input string) bool {
	return strings.Count(input, "@") == 1 && !strings.ContainsAny(input, " \t")
}
