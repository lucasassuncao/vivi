package app

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/hcl"
)

// Rule is one path block of a policy document.
type Rule struct {
	// Path is written as the policy wrote it, glob and all.
	Path         string
	Capabilities []string
}

// Grant is a policy that reaches a path, and what it allows there.
type Grant struct {
	Policy string
	Rule   Rule
	// Exact is true when the rule names the path outright rather than through
	// a glob. An exact rule is the one to edit; a glob is the one to be
	// careful about.
	Exact bool
}

// ParsePolicy pulls the path blocks out of a policy document, using the same
// HCL package Vault does. A document that will not decode is an error and not
// an empty result: "grants nothing" and "could not be read" are different.
func ParsePolicy(document string) ([]Rule, error) {
	var doc struct {
		Path []map[string]struct {
			Capabilities []string `hcl:"capabilities"`
		} `hcl:"path"`
	}
	if err := hcl.Decode(&doc, document); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}

	var rules []Rule
	for _, block := range doc.Path {
		for path, body := range block {
			rules = append(rules, Rule{Path: path, Capabilities: body.Capabilities})
		}
	}
	slices.SortFunc(rules, func(a, b Rule) int { return strings.Compare(a.Path, b.Path) })
	return rules, nil
}

// Matches reports whether a rule reaches a path, and whether by naming it. All
// of Vault's globs: a trailing "*" matches any suffix including none, "+" one
// segment, and a "*" anywhere else is a literal asterisk.
func (r Rule) Matches(path string) (matched, exact bool) {
	pattern := r.Path

	prefix := strings.HasSuffix(pattern, "*")
	if prefix {
		pattern = strings.TrimSuffix(pattern, "*")
	}

	if !strings.Contains(pattern, "+") {
		if prefix {
			return strings.HasPrefix(path, pattern), false
		}
		return path == pattern, path == pattern
	}

	return matchSegments(strings.Split(pattern, "/"), path, prefix), false
}

// matchSegments walks a pattern containing "+" against a path, segment by
// segment. A trailing "*" on the pattern lets the path carry on past the end.
func matchSegments(pattern []string, path string, prefix bool) bool {
	segments := strings.Split(path, "/")
	if len(segments) < len(pattern) {
		return false
	}
	if len(segments) > len(pattern) && !prefix {
		return false
	}

	for i, want := range pattern {
		if want == "+" {
			continue
		}
		// The final segment of a prefix pattern is the part before the "*",
		// so it only has to start the path's segment.
		if prefix && i == len(pattern)-1 {
			if !strings.HasPrefix(segments[i], want) {
				return false
			}
			continue
		}
		if segments[i] != want {
			return false
		}
	}
	return true
}

// GrantsFor is every policy that reaches a path, most specific first: exact
// matches above globs, longer patterns above shorter. unreadable names the
// documents that would not decode, sorted; only the caller can say so on screen.
func GrantsFor(policies map[string]string, path string) (grants []Grant, unreadable []string) {
	for name, document := range policies {
		rules, err := ParsePolicy(document)
		if err != nil {
			unreadable = append(unreadable, name)
			continue
		}
		for _, rule := range rules {
			if matched, exact := rule.Matches(path); matched {
				grants = append(grants, Grant{Policy: name, Rule: rule, Exact: exact})
			}
		}
	}

	slices.Sort(unreadable)
	slices.SortFunc(grants, func(a, b Grant) int {
		if a.Exact != b.Exact {
			if a.Exact {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(len(b.Rule.Path), len(a.Rule.Path)); c != 0 {
			return c
		}
		if c := strings.Compare(a.Policy, b.Policy); c != 0 {
			return c
		}
		return strings.Compare(a.Rule.Path, b.Rule.Path)
	})
	return grants, unreadable
}
