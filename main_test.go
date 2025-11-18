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
