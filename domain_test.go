package main

import "testing"

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "https://example.com"},
		{"HTTPS://Example.COM", "https://example.com"},
		{"http://sub.example.com/path", "http://sub.example.com"},
		{" example.com/login ", "https://example.com"},
		{"https://example.com:443", "https://example.com"},
		{"ftp://example.com", "ftp://example.com"},
	}

	for _, tt := range tests {
		got, err := normalizeOrigin(tt.input)
		if err != nil {
			t.Fatalf("normalizeOrigin(%q) returned error: %v", tt.input, err)
		}
		if got != tt.expected {
			t.Fatalf("normalizeOrigin(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDomainsEqual(t *testing.T) {
	if !domainsEqual("https://Example.com", "https://example.com/") {
		t.Fatalf("domainsEqual should treat casing and trailing slash as equivalent")
	}

	if !domainsEqual("https://example.com", "Example.com") {
		t.Fatalf("domainsEqual should assume protocol is https:// if not provided, and treat casing as equivalent")
	}

	if !domainsEqual("https://example.com", "https://example.com/signup") {
		t.Fatalf("domainsEqual should treat path as equivalent")
	}

	if domainsEqual("https://one.example.com", "https://two.example.com") {
		t.Fatalf("domainsEqual should keep subdomains distinct")
	}

	if domainsEqual("ftp://example.com", "https://example.com") {
		t.Fatalf("domainsEqual should treat different protocols as distinct")
	}

	if domainsEqual("ftp://example.com", "example.com") {
		t.Fatalf("domainsEqual should assume protocol is https:// if not provided, and treat different protocols as distinct")
	}

	if !domainsEqual("https://example.com:443", "https://example.com/signup") {
		t.Fatalf("domainsEqual should treat ports as equivalent")
	}
}
