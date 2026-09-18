package vault

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Some auth backends return the granted policies as one comma-separated string,
// and they are not consistent about the spaces around the commas. An untrimmed
// name matches nothing when "p" goes looking for the policy it names.
func TestPoliciesOfTrimsAndDropsBlanks(t *testing.T) {
	cases := []struct {
		name string
		data map[string]any
		want []string
	}{
		{"spaced string", map[string]any{"policies": "app-admin, default"}, []string{"app-admin", "default"}},
		{"trailing comma", map[string]any{"policies": "app-admin,"}, []string{"app-admin"}},
		{"blank entries", map[string]any{"policies": "a, ,b"}, []string{"a", "b"}},
		{"empty string", map[string]any{"policies": ""}, nil},
		{"only separators", map[string]any{"policies": " , "}, nil},
		{"list form is untouched", map[string]any{"token_policies": []any{"x", "y"}}, []string{"x", "y"}},
		{"token_policies wins", map[string]any{
			"token_policies": []any{"first"},
			"policies":       "second",
		}, []string{"first"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := policiesOf(tc.data)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got %#v, want %#v", got, tc.want)
					break
				}
			}
		})
	}
}

// lookupSelfServer answers auth/token/lookup-self with the given body and
// nothing else, so a test can hand TokenInfo a response of its own shaping.
func lookupSelfServer(t *testing.T, body string) Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/token/lookup-self" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{Address: srv.URL, Token: "t"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return c
}

// TokenInfo maps seven fields out of lookup-self by string key, and a misspelt
// key produces a zero value rather than an error. This pins the spelling.
func TestTokenInfoMapsTheLookupSelfFields(t *testing.T) {
	c := lookupSelfServer(t, `{"data":{
		"accessor":"acc-1",
		"display_name":"token-ci",
		"entity_id":"ent-1",
		"namespace_path":"team/",
		"policies":["default","app-admin"],
		"renewable":true,
		"ttl":600
	}}`)

	before := time.Now()
	info, err := c.TokenInfo(context.Background())
	if err != nil {
		t.Fatalf("token info: %v", err)
	}

	if info.Accessor != "acc-1" || info.DisplayName != "token-ci" || info.EntityID != "ent-1" || info.Namespace != "team/" {
		t.Errorf("string fields came back wrong: %+v", info)
	}
	if len(info.Policies) != 2 || info.Policies[1] != "app-admin" {
		t.Errorf("policies: got %v", info.Policies)
	}
	if !info.Renewable {
		t.Error("renewable was true in the response")
	}
	if info.TTL != 10*time.Minute {
		t.Errorf("ttl: got %v, want 10m", info.TTL)
	}
	if info.ExpiresAt.Before(before.Add(10*time.Minute)) || info.ExpiresAt.After(time.Now().Add(10*time.Minute)) {
		t.Errorf("expires at %v is not ttl from now", info.ExpiresAt)
	}
}

// A zero ttl is a token that never expires, which the header shows as "no
// expiry". That must come from the server saying 0, never from the field being
// absent: the two cases have to be told apart, and this is where they are.
func TestTokenInfoTellsNoExpiryFromNoTTL(t *testing.T) {
	c := lookupSelfServer(t, `{"data":{"accessor":"acc","policies":["root"],"ttl":0}}`)
	info, err := c.TokenInfo(context.Background())
	if err != nil {
		t.Fatalf("a zero ttl is a valid answer: %v", err)
	}
	if info.TTL != 0 || !info.ExpiresAt.IsZero() {
		t.Errorf("a zero ttl must mean no expiry, got ttl=%v expires=%v", info.TTL, info.ExpiresAt)
	}

	c = lookupSelfServer(t, `{"data":{"accessor":"acc","policies":["root"]}}`)
	if _, err := c.TokenInfo(context.Background()); err == nil {
		t.Fatal("a response with no ttl was accepted, and would render as a token that never expires")
	}
}

// Vault answers a missing policy with a 404 the SDK turns into ("", nil). Read
// literally that is a policy granting nothing, which is the one thing
// app.ParsePolicy promises to keep apart from "could not be read".
func TestPolicyTellsAMissingPolicyFromAnEmptyOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/gone") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"policy":"path \"kv/*\" { capabilities = [\"read\"] }"}}`))
	}))
	t.Cleanup(srv.Close)

	c := clientFor(t, srv)

	if _, err := c.Policy(context.Background(), "gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing policy must be ErrNotFound, got %v", err)
	}
	if hcl, err := c.Policy(context.Background(), "here"); err != nil || hcl == "" {
		t.Fatalf("an existing policy must come back whole: %q, %v", hcl, err)
	}
}

// One read per role under a single deadline: once the context is done every
// remaining read fails instantly, and the list would come back whole and wrong,
// each entry blaming the role for a deadline the request hit.
func TestAuthRolesReportsACancelledContextRatherThanUnreadableRoles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("list") == "true" {
			_, _ = w.Write([]byte(`{"data":{"keys":["a","b","c"]}}`))
			return
		}
		// The session ends while the per-role reads are still going.
		cancel()
		_, _ = w.Write([]byte(`{"data":{"token_policies":["p"]}}`))
	}))
	t.Cleanup(srv.Close)

	c := clientFor(t, srv)

	roles, err := c.AuthRoles(ctx, AuthMethod{Path: "approle/", Type: "approle"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled read must fail the listing, got %d roles and %v", len(roles), err)
	}
}

// clientFor points a client at a test server, for the cases that need more than
// the one endpoint lookupSelfServer answers.
func clientFor(t *testing.T, srv *httptest.Server) Client {
	t.Helper()
	c, err := New(Config{Address: srv.URL, Token: "t"})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return c
}
