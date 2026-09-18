package secrets

import (
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// nodeKind separates the three things that can sit in the tree, because each
// answers a different set of keys and shows a different detail panel.
type nodeKind int

const (
	kindMount nodeKind = iota
	kindDir
	kindSecret
)

// node is one row of the secrets tree. It describes itself but never talks to
// Vault: loading is the model's job, so every request takes the same
// asynchronous path and the tree stays a structure tests can build by hand.
type node struct {
	kind  nodeKind
	name  string
	mount string
	// path is relative to the mount, without leading slash. Empty on a mount.
	path string

	kvVersion int
	mountType string
	desc      string

	parent   *node
	children []*node

	expanded bool
	loaded   bool
	loading  bool

	// denied marks a path the token may not list. Such a node stays visible and
	// says so: dropping it would quietly understate the Vault.
	denied bool
	// emptyDir marks a directory that loaded and had nothing in it.
	emptyDir bool
}

// fullPath is what the header and confirmation modals show.
func (n *node) fullPath() string {
	if n.kind == kindMount {
		return n.mount + "/"
	}
	return n.mount + "/" + n.path
}

// depth is how far the node sits from a mount, for indentation.
func (n *node) depth() int {
	d := 0
	for p := n.parent; p != nil; p = p.parent {
		d++
	}
	return d
}

// isKV reports whether this subtree can be browsed at all.
func (n *node) isKV() bool {
	return n.kvVersion == vault.KV1 || n.kvVersion == vault.KV2
}

// child returns a loaded child by name and kind. The kind is half the identity:
// Vault lets ["app", "app/"] coexist, and matching on name alone picks whichever
// came first - which is how a walk to secret/app/queue lands inside secret/app.
func (n *node) child(name string, dir bool) *node {
	for _, c := range n.children {
		if c.name == name && (c.kind != kindSecret) == dir {
			return c
		}
	}
	return nil
}

// hasChild reports whether a loaded listing already holds a secret by this
// name. A directory of the same name does not count: creating secret/app under
// a listing with secret/app/ still adds something the tree has never seen.
func (n *node) hasChild(name string) bool { return n.child(name, false) != nil }

// expandable reports whether the node can hold children.
func (n *node) expandable() bool {
	return n.kind != kindSecret && n.isKV() && !n.denied
}

// rowKind separates the three things a line of the tree pane can be.
type rowKind int

const (
	rowNode rowKind = iota
	rowHeader
	rowSpacer
)

// row is one line of the tree pane. One row is one rendered line: the cursor
// and scrollStart both index this slice, so a header smuggling in its own
// newline would put them out of step with the screen.
type row struct {
	kind   rowKind
	node   *node  // set on rowNode only
	header string // set on rowHeader only
}

// tree is the mount forest plus the cursor and the filter over it.
type tree struct {
	roots  []*node
	cursor int

	// filter is the fuzzy pattern typed with "/". It matches against what has
	// already been loaded: the tree never walks the server to satisfy a
	// filter, so filtering stays instant and free.
	filter string

	// visible is the flattened rows, recomputed whenever structure or filter
	// changes. It holds the section headers as well as the nodes, because the
	// cursor and the scroll both count lines on the screen.
	visible []row

	// grouped reports whether reflow emitted section headers. It is false when
	// only one section has anything in it, and the view reads it to decide
	// whether a mount's row still has to name its own engine.
	grouped bool
}

// setMounts replaces the forest, keeping only KV mounts browsable.
func (t *tree) setMounts(mounts []vault.Mount) {
	t.roots = t.roots[:0]
	for _, m := range mounts {
		name := strings.Trim(m.Path, "/")
		t.roots = append(t.roots, &node{
			kind:      kindMount,
			name:      name,
			mount:     name,
			kvVersion: m.KVVersion,
			mountType: m.Type,
			desc:      m.Description,
		})
	}
	t.cursor = 0
	t.reflow()
}

// setChildren installs a loaded listing under a node.
func (t *tree) setChildren(n *node, entries []vault.Entry) {
	n.children = n.children[:0]
	for _, e := range entries {
		kind := kindSecret
		if e.IsDir {
			kind = kindDir
		}
		n.children = append(n.children, newChild(n, kind, e.Name))
	}
	n.loaded = true
	n.loading = false
	n.emptyDir = len(n.children) == 0
	t.reflow()
}

// newChild builds a node under a parent, inheriting the mount it lives in. It
// is not attached: a listing appends it, a create keeps it off the tree.
func newChild(parent *node, kind nodeKind, name string) *node {
	return &node{
		kind:      kind,
		name:      name,
		mount:     parent.mount,
		path:      joinRel(parent.path, name),
		kvVersion: parent.kvVersion,
		parent:    parent,
	}
}

func joinRel(base, name string) string {
	if base == "" {
		return name
	}
	return base + "/" + name
}

// markDenied records that the token may not list this node.
func (t *tree) markDenied(n *node) {
	n.denied = true
	n.loading = false
	n.loaded = true
	n.expanded = false
	t.reflow()
}

// groupLabels names the sections of the mount list, in the order they are shown.
var groupLabels = [...]string{"KV V2", "KV V1 (no history)", "NOT BROWSABLE"}

// mountGroup is the section a mount belongs to. The key is the engine, resolved
// once in setMounts: a section a mount could leave would reorder the list under
// the cursor, which is why denial stays a badge on the row instead.
func mountGroup(n *node) int {
	switch n.kvVersion {
	case vault.KV2:
		return 0
	case vault.KV1:
		return 1
	default:
		return 2
	}
}

// reflow rebuilds the visible rows from the expansion state and the filter,
// under one header per non-empty section. A lone section gets no header: it
// separates nothing from nothing.
func (t *tree) reflow() {
	t.visible = t.visible[:0]

	groups := make([][]row, len(groupLabels))
	filled := 0
	for g := range groupLabels {
		for _, r := range t.roots {
			if mountGroup(r) == g {
				groups[g] = t.appendRows(groups[g], r)
			}
		}
		if len(groups[g]) > 0 {
			filled++
		}
	}
	t.grouped = filled > 1

	for g, rows := range groups {
		// A section the filter emptied loses its header with it.
		if len(rows) == 0 {
			continue
		}
		if t.grouped {
			if len(t.visible) > 0 {
				t.visible = append(t.visible, row{kind: rowSpacer})
			}
			t.visible = append(t.visible, row{kind: rowHeader, header: groupLabels[g]})
		}
		t.visible = append(t.visible, rows...)
	}

	t.cursor = t.snap(clampIndex(t.cursor, max(0, len(t.visible)-1)), 1)
}

// appendRows flattens a node and its expanded descendants onto out.
func (t *tree) appendRows(out []row, n *node) []row {
	if t.matches(n) {
		out = append(out, row{kind: rowNode, node: n})
	}
	if !n.expanded {
		return out
	}
	for _, c := range n.children {
		out = t.appendRows(out, c)
	}
	return out
}

// snap returns the nearest row holding a node, starting at i and walking in dir,
// falling back to the other direction when that runs off the end.
func (t *tree) snap(i, dir int) int {
	if len(t.visible) == 0 {
		return 0
	}
	i = clampIndex(i, len(t.visible)-1)

	for j := i; j >= 0 && j < len(t.visible); j += dir {
		if t.visible[j].kind == rowNode {
			return j
		}
	}
	for j := i; j >= 0 && j < len(t.visible); j -= dir {
		if t.visible[j].kind == rowNode {
			return j
		}
	}
	return 0
}

// matches decides whether a node survives the filter: kept when it matches, and
// when a loaded descendant does. Otherwise filtering hides the path leading to
// the hit and the result is unreachable.
func (t *tree) matches(n *node) bool {
	if t.filter == "" {
		return true
	}
	if fuzzy.MatchFold(t.filter, n.name) || fuzzy.MatchFold(t.filter, n.fullPath()) {
		return true
	}
	for _, c := range n.children {
		if t.matches(c) {
			return true
		}
	}
	return false
}

// current returns the selected node, or nil when the tree is empty.
func (t *tree) current() *node {
	if t.cursor < 0 || t.cursor >= len(t.visible) {
		return nil
	}
	return t.visible[t.cursor].node
}

// move walks delta nodes, stepping over the rows that are not nodes and stopping
// on the last node rather than running into a trailing header.
func (t *tree) move(delta int) {
	if len(t.visible) == 0 {
		return
	}
	step := 1
	if delta < 0 {
		step, delta = -1, -delta
	}

	pos := t.cursor
	for ; delta > 0; delta-- {
		next := pos + step
		for next >= 0 && next < len(t.visible) && t.visible[next].kind != rowNode {
			next += step
		}
		if next < 0 || next >= len(t.visible) {
			break
		}
		pos = next
	}
	t.cursor = pos
}

// moveTo jumps to a line, snapping backwards to a node. Backwards is what makes
// "end" work: keySecrets asks for the last line, and the last line of a grouped
// tree is a node only by coincidence.
func (t *tree) moveTo(i int) {
	if len(t.visible) == 0 {
		return
	}
	t.cursor = t.snap(clampIndex(i, len(t.visible)-1), -1)
}

// selectNode puts the cursor on a node if it is currently visible.
func (t *tree) selectNode(target *node) {
	for i, r := range t.visible {
		if r.kind == rowNode && r.node == target {
			t.cursor = i
			return
		}
	}
}

// collapse closes the current node, or moves to its parent when it is already
// closed. Going up on a closed node is what makes "h" feel like "back".
func (t *tree) collapse() {
	n := t.current()
	if n == nil {
		return
	}
	if n.expanded {
		n.expanded = false
		t.reflow()
		return
	}
	if n.parent != nil {
		t.reflow()
		t.selectNode(n.parent)
	}
}

// invalidate drops a node's loaded children so the next expansion refetches.
func (t *tree) invalidate(n *node) {
	n.children = nil
	n.loaded = false
	n.denied = false
	n.emptyDir = false
	t.reflow()
}

// clampIndex keeps a cursor inside [0, hi].
func clampIndex(v, hi int) int { return min(max(v, 0), hi) }

// find walks a path as far as the tree goes, stopping rather than failing on an
// unloaded one. Mounts match by longest name, since a mount path may contain
// slashes; a trailing slash separates the secret app from the directory app/.
func (t *tree) find(path string) (*node, string) {
	wantDir := strings.HasSuffix(path, "/")
	path = strings.Trim(path, "/")

	var root *node
	for _, r := range t.roots {
		if path != r.name && !strings.HasPrefix(path, r.name+"/") {
			continue
		}
		if root == nil || len(r.name) > len(root.name) {
			root = r
		}
	}
	if root == nil {
		return nil, path
	}

	n := root
	rest := strings.TrimPrefix(strings.TrimPrefix(path, root.name), "/")
	for rest != "" {
		seg, tail, _ := strings.Cut(rest, "/")

		// Every segment but the last is a directory, because the path goes on
		// through it. The last is whichever the trailing slash asked for, or the
		// only one that exists: insisting would fail an unambiguous path.
		last := tail == ""
		child := n.child(seg, !last || wantDir)
		if child == nil && last {
			child = n.child(seg, !wantDir)
		}
		if child == nil {
			return n, rest
		}
		n, rest = child, tail
	}
	return n, ""
}

// revealNode opens everything above a node and puts the cursor on it. A jump
// that landed on a row folded inside a closed parent would look like nothing
// had happened.
func (t *tree) revealNode(n *node) {
	for p := n.parent; p != nil; p = p.parent {
		p.expanded = true
	}
	t.reflow()
	t.selectNode(n)
}

// Rendering. The tree draws itself here rather than in the Secrets tab: what a
// row looks like is a property of the node under it - denied, empty, not
// browsable, still loading - and those are the states this file owns.

// renderTree draws the mount forest, scrolled to keep the cursor visible.
func (m *Model) renderTree(width, height int) string {
	var b strings.Builder

	// While filtering, the pattern gets its own line at the top of the pane, so
	// the mode is visible where the results are rather than only in the footer.
	if modeIs[filtering](m) || m.tree.filter != "" {
		b.WriteString(ui.Truncate(m.renderFilterBox(), width))
		b.WriteString("\n")
		height--
	}

	if len(m.tree.visible) == 0 {
		if m.tree.filter != "" {
			return b.String() + m.st.EmptyState(width, height,
				"nothing matches", "esc clears the filter")
		}
		return b.String() + m.st.EmptyState(width, height,
			"no mounts this token can read", "check the token's policies")
	}

	start := ui.ScrollStart(m.tree.cursor, len(m.tree.visible), height)
	for i := start; i < len(m.tree.visible) && i < start+height; i++ {
		switch r := m.tree.visible[i]; r.kind {
		case rowHeader:
			b.WriteString(ui.Truncate(m.st.GroupLabel.Render(r.header), width))
		case rowSpacer:
			// The blank line is the row; there is nothing to draw on it.
		default:
			b.WriteString(m.renderTreeRow(r.node, i == m.tree.cursor, width, !m.tree.grouped))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderFilterBox is the visible sign that typing now filters.
func (m *Model) renderFilterBox() string {
	if modeIs[filtering](m) {
		return m.st.Editing.Render("/ ") + m.filterInput.View()
	}
	return m.st.Dim.Render("/ " + m.tree.filter + "  (esc clears)")
}

// renderTreeRow draws one node. showVersion is false while the tree is grouped,
// because the section header above the row already names the engine.
func (m *Model) renderTreeRow(n *node, selected bool, width int, showVersion bool) string {
	indent := strings.Repeat("  ", n.depth())

	var marker string
	switch {
	case n.kind == kindSecret:
		marker = "  "
	case n.expanded:
		marker = "▾ "
	default:
		marker = "▸ "
	}

	name := ui.Sanitize(n.name)
	if n.kind != kindSecret {
		name += "/"
	}

	var styled string
	switch {
	case n.denied:
		// Kept short: the pane is narrow, and a truncated label defeats the point
		// of showing the node. Text and not a padlock: an emoji is East Asian
		// Wide, so runewidth counts two columns where many fonts draw one.
		styled = m.st.Denied.Render(name + "  [denied]")
	case n.kind == kindMount:
		// An expanded mount with nothing under it says so. Without the label the
		// open arrow points at blank space, which reads as a failed load rather
		// than as an empty mount: a Vault in dev mode ships one of these.
		styled = m.st.Mount.Render(name) + m.st.Dim.Render(mountSuffix(n, showVersion))
		if n.emptyDir {
			styled += m.st.Dim.Render("  (empty)")
		}
	case n.kind == kindDir:
		styled = m.st.Dir.Render(name)
		if n.emptyDir {
			styled += m.st.Dim.Render("  (empty)")
		}
	default:
		styled = m.st.Secret.Render(name)
	}

	cursor := "  "
	if selected {
		cursor = m.st.Cursor.Render("● ")
	}

	return ui.Truncate(cursor+indent+marker+styled, width)
}

// mountSuffix labels a mount with what it is, so a non-KV one explains why it
// will not open. Grouped, only what the section header does not already say
// survives - but the type stays: pki and transit share a heading.
func mountSuffix(n *node, showVersion bool) string {
	switch n.kvVersion {
	case vault.KV2:
		if !showVersion {
			return ""
		}
		return "  v2"
	case vault.KV1:
		if !showVersion {
			return ""
		}
		return "  v1"
	default:
		kind := n.mountType
		if redundantType(n.name, kind) {
			kind = ""
		}
		switch {
		case !showVersion && kind == "":
			return ""
		case !showVersion:
			return "  " + kind
		case kind == "":
			return "  (not browsable)"
		default:
			return "  " + kind + " (not browsable)"
		}
	}
}

// redundantType reports whether a mount's type only repeats its own name:
// "pki/  pki" says nothing twice. A type that differs survives, since "sys/" is
// worth labelling "system". Separators are ignored, so "a-b" and "a_b" match.
func redundantType(name, mountType string) bool {
	strip := func(s string) string {
		return strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(s))
	}
	return strip(name) == strip(mountType)
}
