package app_test

import (
	"testing"

	"github.com/lucasassuncao/vivi/internal/app"
)

func TestParsePolicyReadsPathBlocks(t *testing.T) {
	rules, err := app.ParsePolicy(`
# a comment
path "kv/data/app/*" {
  capabilities = ["create", "read", "update"]
}
path "kv/metadata/app/prod/db" {
  capabilities = ["list"]
}
`)
	if err != nil {
		t.Fatalf("a well-formed document did not parse: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("parsed %d rules, want 2: %+v", len(rules), rules)
	}
	if rules[0].Path != "kv/data/app/*" || len(rules[0].Capabilities) != 3 {
		t.Errorf("first rule is %+v", rules[0])
	}
}

// A document Vault would reject used to yield nothing, which the access panel
// drew the same way as a policy that does not reach this path. "Names it" and
// "could not be read" are different answers, and only one of them is true.
func TestParsePolicyReportsADocumentItCannotRead(t *testing.T) {
	rules, err := app.ParsePolicy(`path "unclosed" {`)
	if err == nil {
		t.Fatal("a broken document parsed without complaint")
	}
	if len(rules) != 0 {
		t.Errorf("a broken document produced %d rules", len(rules))
	}
}

// An empty document is not a broken one: a policy may legitimately declare no
// path blocks, and reporting that as a failure would put a warning on the panel
// for a policy that is doing nothing wrong.
func TestParsePolicyAcceptsADocumentWithNoRules(t *testing.T) {
	rules, err := app.ParsePolicy("# nothing here\n")
	if err != nil {
		t.Fatalf("an empty policy is not an error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("parsed %d rules out of an empty document", len(rules))
	}
}

func TestRuleMatchingFollowsVaultsGlobs(t *testing.T) {
	for _, tc := range []struct {
		pattern, path   string
		want, wantExact bool
	}{
		{"kv/data/app/prod/db", "kv/data/app/prod/db", true, true},
		{"kv/data/app/prod/db", "kv/data/app/prod/dbx", false, false},
		{"kv/data/app/*", "kv/data/app/prod/db", true, false},
		{"kv/data/app/*", "kv/data/other/db", false, false},
		{"kv/data/*", "kv/data/", true, false},
		{"kv/data/+/db", "kv/data/prod/db", true, false},
		{"kv/data/+/db", "kv/data/prod/deep/db", false, false},
		{"kv/data/+/db", "kv/data/prod/other", false, false},
		{"secret/+/*", "secret/team/a/b", true, false},
	} {
		got, exact := app.Rule{Path: tc.pattern}.Matches(tc.path)
		if got != tc.want || exact != tc.wantExact {
			t.Errorf("%q vs %q: matched=%v exact=%v, want %v/%v",
				tc.pattern, tc.path, got, exact, tc.want, tc.wantExact)
		}
	}
}

// Most specific first, because that is the order somebody reads them in: which
// rule is this really, before what else happens to cover it.
func TestGrantsAreOrderedMostSpecificFirst(t *testing.T) {
	grants, _ := app.GrantsFor(map[string]string{
		"broad":  `path "kv/data/*" { capabilities = ["read"] }`,
		"narrow": `path "kv/data/app/prod/db" { capabilities = ["read"] }`,
		"middle": `path "kv/data/app/*" { capabilities = ["read"] }`,
		"other":  `path "kv/data/other/*" { capabilities = ["read"] }`,
	}, "kv/data/app/prod/db")

	var order []string
	for _, g := range grants {
		order = append(order, g.Policy)
	}
	want := []string{"narrow", "middle", "broad"}
	if len(order) != len(want) {
		t.Fatalf("grants: %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("grants: %v, want %v", order, want)
			break
		}
	}
	if !grants[0].Exact {
		t.Error("the rule naming the path outright should be marked exact")
	}
	if grants[1].Exact {
		t.Error("a glob should not be marked exact")
	}
}

func TestGrantsForNamesThePoliciesItCouldNotRead(t *testing.T) {
	grants, unreadable := app.GrantsFor(map[string]string{
		"reader": `path "kv/data/app/prod/db" { capabilities = ["read"] }`,
		"broken": `path "unclosed" {`,
		"silent": `path "kv/data/other/*" { capabilities = ["read"] }`,
	}, "kv/data/app/prod/db")

	if len(grants) != 1 || grants[0].Policy != "reader" {
		t.Fatalf("grants are %+v, want the one from reader", grants)
	}
	if len(unreadable) != 1 || unreadable[0] != "broken" {
		t.Fatalf("unreadable is %v, want [broken]: a policy that does not match "+
			"is not a policy that would not parse", unreadable)
	}
}
