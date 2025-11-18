package main

import "testing"

func TestAliasMatchesDomain(t *testing.T) {
	target := "https://example.com"

	if !aliasMatchesDomain(MaskedEmailInfo{
		ForDomain: target,
	}, target) {
		t.Fatalf("expected direct forDomain match")
	}

	if aliasMatchesDomain(MaskedEmailInfo{
		ForDomain: "https://other.com",
	}, target) {
		t.Fatalf("did not expect different domain to match")
	}

	if !aliasMatchesDomain(MaskedEmailInfo{
		ForDomain:   "",
		Description: "https://example.com",
	}, target) {
		t.Fatalf("expected description fallback to match")
	}

	if aliasMatchesDomain(MaskedEmailInfo{
		ForDomain:   "",
		Description: "https://other.com",
	}, target) {
		t.Fatalf("description fallback should not match different domains")
	}

	if !aliasMatchesDomain(MaskedEmailInfo{
		ForDomain:   "https://Example.com/signup",
	}, target) {
		t.Fatalf("expected ForDomain to match (casing and trailing slash should be ignored)")
	}
}
