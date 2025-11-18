package main

import "testing"

func TestSelectPreferredAliasUnknownState(t *testing.T) {
	aliases := []MaskedEmailInfo{
		{Email: "unknown@example.com", State: AliasState("mystery")},
		{Email: "enabled@example.com", State: AliasEnabled},
	}

	selected := selectPreferredAlias(aliases)
	if selected == nil {
		t.Fatalf("expected alias to be selected")
	}
	if selected.Email != "enabled@example.com" {
		t.Fatalf("expected enabled alias to be selected, got %s", selected.Email)
	}
}

func TestAliasMatchesSearch(t *testing.T) {
	alias := MaskedEmailInfo{
		Email:       "user@example.com",
		Description: "Shopping account",
		ForDomain:   "https://example.com",
		ID:          "alias-id",
	}

	if !aliasMatchesSearch(alias, "shopping") {
		t.Fatalf("expected description match to be detected")
	}

	if !aliasMatchesSearch(alias, "alias-id") {
		t.Fatalf("expected ID match to be detected")
	}

	if aliasMatchesSearch(alias, "nomatch") {
		t.Fatalf("did not expect match for unrelated text")
	}
}

func TestFilterAliasesForList(t *testing.T) {
	aliases := []MaskedEmailInfo{
		{ID: "1", Email: "one@example.com", ForDomain: "https://example.com", State: AliasEnabled},
		{ID: "2", Email: "two@example.com", ForDomain: "https://other.com", Description: "Example login", State: AliasEnabled},
		{ID: "3", Email: "three@example.com", ForDomain: "https://third.com", State: AliasEnabled},
		{ID: "5", Email: "sub@example.com", ForDomain: "https://sub.example.com", State: AliasEnabled},
		{ID: "4", Email: "deleted@example.com", ForDomain: "https://example.com", State: AliasDeleted},
	}

	matching, related := filterAliasesForList(aliases, "https://example.com", "example")

	if len(matching) != 1 || matching[0].Email != "one@example.com" {
		t.Fatalf("expected single primary match for forDomain, got %+v", matching)
	}

	if len(related) != 3 {
		t.Fatalf("expected related matches based on description/email search, got %+v", related)
	}

	foundSubdomain := false
	for _, alias := range related {
		if alias.Email == "sub@example.com" {
			foundSubdomain = true
			break
		}
	}
	if !foundSubdomain {
		t.Fatalf("expected subdomain alias to appear in related matches, got %+v", related)
	}
}
