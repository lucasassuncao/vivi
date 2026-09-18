package vault

import (
	"context"
	"slices"
	"strings"
	"time"
)

// Policies lists policy names, dropping none: "root" and "default" are part of
// the picture when auditing who can do what.
func (a *apiClient) Policies(ctx context.Context) ([]string, error) {
	names, err := a.c.Sys().ListPoliciesWithContext(ctx)
	if err != nil {
		return nil, wrap("list policies", "sys/policy", err)
	}
	slices.Sort(names)
	return names, nil
}

// Policy returns a policy's HCL document.
func (a *apiClient) Policy(ctx context.Context, name string) (string, error) {
	hcl, err := a.c.Sys().GetPolicyWithContext(ctx, name)
	if err != nil {
		return "", wrap("read policy", name, err)
	}
	// The SDK answers a missing policy with ("", nil), and an empty document
	// parses as a policy that grants nothing. A name can outlive the policy it
	// points at, so the two have to arrive here as different answers.
	if hcl == "" {
		return "", &Error{Op: "read policy", Path: name, Kind: ErrNotFound}
	}
	return hcl, nil
}

// AuthMethods lists the enabled auth backends.
func (a *apiClient) AuthMethods(ctx context.Context) ([]AuthMethod, error) {
	table, err := a.c.Sys().ListAuthWithContext(ctx)
	if err != nil {
		return nil, wrap("list auth methods", "sys/auth", err)
	}

	methods := make([]AuthMethod, 0, len(table))
	for path, out := range table {
		if out == nil {
			continue // the same guard the mount table gets, for the same reason
		}
		methods = append(methods, AuthMethod{
			Path:        path,
			Type:        out.Type,
			Description: out.Description,
			Accessor:    out.Accessor,
		})
	}
	slices.SortFunc(methods, func(a, b AuthMethod) int { return strings.Compare(a.Path, b.Path) })
	return methods, nil
}

// AuthRoles lists the roles under one auth method. Each backend keeps them
// somewhere different and some have no roles at all, so a method vivi cannot
// enumerate returns an empty slice: unfamiliar is not a failure.
func (a *apiClient) AuthRoles(ctx context.Context, method AuthMethod) ([]Role, error) {
	kind, ok := roleListing[method.Type]
	if !ok {
		return nil, nil
	}

	listPath := JoinPath("auth", method.Path, kind.list)
	sec, err := a.c.Logical().ListWithContext(ctx, listPath)
	if err != nil {
		return nil, wrap("list roles", listPath, err)
	}
	keys := listedKeys(sec)
	if keys == nil {
		return nil, nil
	}

	roles := make([]Role, 0, len(keys))
	for _, name := range keys {
		// One read per role under one deadline: past it every remaining read
		// fails at once, and each role would be blamed for the session ending.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name = strings.TrimSuffix(name, "/")

		// A token may list roles it cannot read, an ordinary least-privilege
		// split. The failure travels with the role: "grants nothing" and
		// "could not be read" are different answers.
		role := Role{Name: name}
		readPath := JoinPath("auth", method.Path, kind.read, name)
		detail, err := a.c.Logical().ReadWithContext(ctx, readPath)
		switch {
		case err != nil:
			role.ReadErr = wrap("read role", readPath, err)
		case detail != nil:
			role.Data = detail.Data
			role.Policies = policiesOf(detail.Data)
		}
		roles = append(roles, role)
	}

	slices.SortFunc(roles, func(a, b Role) int { return strings.Compare(a.Name, b.Name) })
	return roles, nil
}

// roleListing maps an auth backend type to the endpoints that enumerate and
// describe its roles.
var roleListing = map[string]struct{ list, read string }{
	"approle":    {list: "role", read: "role"},
	"userpass":   {list: "users", read: "users"},
	"ldap":       {list: "groups", read: "groups"},
	"okta":       {list: "groups", read: "groups"},
	"kubernetes": {list: "role", read: "role"},
	"jwt":        {list: "role", read: "role"},
	"oidc":       {list: "role", read: "role"},
	"aws":        {list: "role", read: "role"},
	"azure":      {list: "role", read: "role"},
	"gcp":        {list: "role", read: "role"},
	"cert":       {list: "certs", read: "certs"},
}

// RoleReadPath is the API path describing one role, and "" for a backend vivi
// cannot enumerate. Exported so the interface shows the CLI command without a
// second copy of roleListing, which would drift and read as authoritative.
func RoleReadPath(method AuthMethod, role string) string {
	kind, ok := roleListing[method.Type]
	if !ok {
		return ""
	}
	return JoinPath("auth", method.Path, kind.read, role)
}

// policiesOf pulls the policy list out of a role, which different backends name
// differently.
func policiesOf(data map[string]any) []string {
	for _, key := range []string{"token_policies", "policies"} {
		switch v := data[key].(type) {
		case []any:
			out := make([]string, 0, len(v))
			for _, p := range v {
				if s, ok := p.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		case string:
			// Some backends return the list as one comma-separated string, and
			// they are not consistent about the spaces. An untrimmed name
			// matches nothing when "p" goes looking for the policy.
			if out := splitAndTrim(v); len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

// TokenInfo describes the token vivi is running with.
func (a *apiClient) TokenInfo(ctx context.Context) (*TokenInfo, error) {
	sec, err := a.c.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil {
		return nil, wrap("lookup token", "auth/token/lookup-self", err)
	}
	if sec == nil || sec.Data == nil {
		return nil, &Error{Op: "lookup token", Path: "auth/token/lookup-self", Kind: ErrNotFound}
	}

	info := &TokenInfo{
		Accessor:    stringField(sec.Data, "accessor"),
		DisplayName: stringField(sec.Data, "display_name"),
		EntityID:    stringField(sec.Data, "entity_id"),
		Namespace:   stringField(sec.Data, "namespace_path"),
		Policies:    stringSlice(sec.Data["policies"]),
	}

	if r, ok := sec.Data["renewable"].(bool); ok {
		info.Renewable = r
	}

	// Vault always answers with a ttl, 0 for a token that never expires. One
	// that cannot be read must not render as "no expiry": that is the wrong
	// direction to be wrong in.
	ttl, err := numberField(sec.Data, "ttl")
	if err != nil {
		return nil, &Error{
			Op:     "lookup token",
			Path:   "auth/token/lookup-self",
			Detail: "response has no readable ttl: " + err.Error(),
		}
	}
	info.TTL = time.Duration(ttl) * time.Second
	if ttl > 0 {
		info.ExpiresAt = time.Now().Add(info.TTL)
	}

	return info, nil
}

// RenewToken extends the lease, then re-reads the token rather than trusting
// the response: the server may cap the extension, and the header counts down
// what will be honoured. The one write with no confirmation gate.
func (a *apiClient) RenewToken(ctx context.Context) (*TokenInfo, error) {
	if _, err := a.c.Auth().Token().RenewSelfWithContext(ctx, 0); err != nil {
		// classify produces ErrUnsupported for no status code, so the kind this
		// interface promises comes from asking the token instead of reading the
		// server's wording. Failure path only.
		if info, lookupErr := a.TokenInfo(ctx); lookupErr == nil && !info.Renewable {
			return nil, &Error{
				Op:     "renew token",
				Path:   "auth/token/renew-self",
				Kind:   ErrUnsupported,
				Detail: "this token is not renewable",
			}
		}
		return nil, wrap("renew token", "auth/token/renew-self", err)
	}
	return a.TokenInfo(ctx)
}

// splitAndTrim splits a comma-separated list, dropping blanks.
func splitAndTrim(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
