package secrets

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// Every Vault call is a tea.Cmd returning one of these messages. Nothing in the
// update loop blocks on the network: a slow server slows loading, never typing.
// ui.RequestAnswer marks the ones that settle request's pending count. It is
// named in ui because a tab builds its own answers and the parent counts them.

func (mountsMsg) AnswersRequest()       {}
func (childrenMsg) AnswersRequest()     {}
func (secretMsg) AnswersRequest()       {}
func (versionsMsg) AnswersRequest()     {}
func (writeMsg) AnswersRequest()        {}
func (opMsg) AnswersRequest()           {}
func (diffMsg) AnswersRequest()         {}
func (capabilitiesMsg) AnswersRequest() {}
func (policyDocMsg) AnswersRequest()    {}

type mountsMsg struct {
	mounts []vault.Mount
	err    error
}

type childrenMsg struct {
	node    *node
	entries []vault.Entry
	err     error
}

type secretMsg struct {
	node    *node
	version int
	secret  *vault.Secret
	err     error
}

type versionsMsg struct {
	node     *node
	versions []vault.VersionInfo
	meta     *vault.Metadata
	err      error
}

// writeMsg reports a finished write. cas carries what was attempted so a
// rejected check-and-set can explain itself.
type writeMsg struct {
	node   *node
	secret *vault.Secret
	cas    int
	err    error
	// editor is set when the write came from the field editor, and is where a
	// rejection returns. A rollback is a write with no editor behind it.
	editor *editor
}

// opMsg reports a finished destructive operation.
type opMsg struct {
	op   string
	path string
	node *node
	err  error
}

// request builds the command for one server call. The client is captured now,
// so the closure holds no reference to the Model; the timeout, the session
// context and the in-flight count are the shell's, through ui.Requester.
func (m *Model) request(call func(context.Context, vault.Client) ui.RequestAnswer) tea.Cmd {
	client := m.client
	return m.req.Request(func(ctx context.Context) tea.Msg { return call(ctx, client) })
}

func (m *Model) loadMounts() tea.Cmd {
	return m.request(func(ctx context.Context, c vault.Client) ui.RequestAnswer {
		mounts, err := c.Mounts(ctx)
		return mountsMsg{mounts: mounts, err: err}
	})
}

// nodeRef is what a command needs to address a node, read off it while the
// update loop still owns it. The node pointer travels along only so the answer
// can be matched: every guard compares it, and never dereferences. See onNode.
type nodeRef struct {
	node *node

	mount     string
	path      string
	fullPath  string
	kvVersion int
}

func refTo(n *node) nodeRef {
	return nodeRef{
		node:      n,
		mount:     n.mount,
		path:      n.path,
		fullPath:  n.fullPath(),
		kvVersion: n.kvVersion,
	}
}

// onNode is request for the calls that address a node. A nil node is refused
// here rather than dereferenced off the loop, and the node is copied into a
// nodeRef here too: reading it in the closure raced setChildren and markDenied.
func (m *Model) onNode(n *node, call func(context.Context, vault.Client, nodeRef) ui.RequestAnswer) tea.Cmd {
	if n == nil {
		return nil
	}
	ref := refTo(n)
	return m.request(func(ctx context.Context, c vault.Client) ui.RequestAnswer {
		return call(ctx, c, ref)
	})
}

func (m *Model) loadChildren(n *node) tea.Cmd {
	return m.onNode(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		entries, err := c.List(ctx, r.mount, r.path)
		return childrenMsg{node: r.node, entries: entries, err: err}
	})
}

func (m *Model) loadSecret(n *node, version int) tea.Cmd {
	return m.onNode(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		sec, err := c.Read(ctx, r.mount, r.path, version)
		return secretMsg{node: r.node, version: version, secret: sec, err: err}
	})
}

func (m *Model) loadVersions(n *node) tea.Cmd {
	return m.onNode(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		versions, meta, err := c.Versions(ctx, r.mount, r.path)
		return versionsMsg{node: r.node, versions: versions, meta: meta, err: err}
	})
}

func (m *Model) writeSecret(n *node, data map[string]any, cas int, e *editor) tea.Cmd {
	return m.mutate(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		sec, err := c.Write(ctx, r.mount, r.path, data, cas)
		return writeMsg{node: r.node, secret: sec, cas: cas, err: err, editor: e}
	})
}

// operate is mutate for the calls that answer with nothing but an error: the
// four version operations, which all report through one opMsg.
func (m *Model) operate(n *node, op string, call func(context.Context, vault.Client, nodeRef) error) tea.Cmd {
	return m.mutate(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		return opMsg{op: op, path: r.fullPath, node: r.node, err: call(ctx, c, r)}
	})
}

func (m *Model) deleteVersions(n *node, versions []int) tea.Cmd {
	return m.operate(n, "delete", func(ctx context.Context, c vault.Client, r nodeRef) error {
		return c.Delete(ctx, r.mount, r.path, versions)
	})
}

func (m *Model) undeleteVersions(n *node, versions []int) tea.Cmd {
	return m.operate(n, "undelete", func(ctx context.Context, c vault.Client, r nodeRef) error {
		return c.Undelete(ctx, r.mount, r.path, versions)
	})
}

func (m *Model) destroyVersions(n *node, versions []int) tea.Cmd {
	return m.operate(n, "destroy", func(ctx context.Context, c vault.Client, r nodeRef) error {
		return c.Destroy(ctx, r.mount, r.path, versions)
	})
}

func (m *Model) deleteMetadata(n *node) tea.Cmd {
	return m.operate(n, "delete-metadata", func(ctx context.Context, c vault.Client, r nodeRef) error {
		return c.DeleteMetadata(ctx, r.mount, r.path)
	})
}

// capabilitiesMsg is the server's own answer to "what may I do here".
type capabilitiesMsg struct {
	node *node
	caps []string
	err  error
}

// policyDocMsg is one of the token's own policy documents, fetched for the
// reverse lookup rather than for the Policies tab.
type policyDocMsg struct {
	name string
	hcl  string
	err  error
}

// loadCapabilities asks the server what the token may do at a node's data path.
func (m *Model) loadCapabilities(n *node) tea.Cmd {
	return m.onNode(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		caps, err := c.Capabilities(ctx, app.PolicyPath(r.mount, r.path, r.kvVersion))
		return capabilitiesMsg{node: r.node, caps: caps, err: err}
	})
}

// loadTokenPolicies fetches the documents of this token's policies, once. A
// handful of requests, worth making eagerly on the first secret: the section
// they feed appearing seconds after the rest reads as a glitch.
func (m *Model) loadTokenPolicies() tea.Cmd {
	if m.policyDocsAsked || len(m.tokenPolicies) == 0 {
		return nil
	}
	m.policyDocsAsked = true

	cmds := make([]tea.Cmd, 0, len(m.tokenPolicies))
	for _, name := range m.tokenPolicies {
		cmds = append(cmds, m.request(func(ctx context.Context, c vault.Client) ui.RequestAnswer {
			hcl, err := c.Policy(ctx, name)
			return policyDocMsg{name: name, hcl: hcl, err: err}
		}))
	}
	return tea.Batch(cmds...)
}
