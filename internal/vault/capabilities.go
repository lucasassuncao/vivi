package vault

import (
	"context"
	"slices"
)

// Capabilities asks the server what this token may do at a path - the only
// honest answer to "may I write here", since everything else infers from policy
// documents. A raw API path, not a browsing path: see app.PolicyPath.
func (a *apiClient) Capabilities(ctx context.Context, path string) ([]string, error) {
	caps, err := a.c.Sys().CapabilitiesSelfWithContext(ctx, path)
	if err != nil {
		return nil, wrap("read capabilities", "sys/capabilities-self", err)
	}
	slices.Sort(caps)
	return caps, nil
}
