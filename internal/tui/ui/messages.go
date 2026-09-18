package ui

import tea "charm.land/bubbletea/v2"

// How a tab asks for something it does not own. There is one way and it is a
// message: a tab returns a command that emits one of these, and the parent
// folds it. A tab never writes the parent and never touches another tab.

// BannerMsg puts a transient line in the footer. Err picks the colour and is
// the whole of the difference between notify and fail.
//
// Cause travels as an error and not as a sentence because reading a Vault
// failure is the parent's job: a tab that spelled out "permission denied"
// itself would need this package to know what vault.ErrForbidden is.
type BannerMsg struct {
	Text  string
	Err   bool
	Cause error
}

// FocusMsg moves the keyboard to a pane.
type FocusMsg struct{ Focus Focus }

// ScrollTopMsg sends the detail pane back to its first line. A tab whose
// selection moved asks for this rather than reaching for the viewport, which is
// the parent's: the pane is the frame, and the frame is not a tab's to scroll.
type ScrollTopMsg struct{}

// CopyMsg asks for text to reach the system clipboard. The clipboard is the
// parent's because a component writing it directly could not be tested without
// a real one, which is why Model.clip is a field in the first place.
type CopyMsg struct {
	// Op names the action for the activity log, What names it for the banner.
	Op, What string
	Text     string
}

// ScrollToLineMsg brings one line of the detail pane into view, which is what
// a selection that has scrolled out of sight needs: it is worse than no
// selection, because the keys still act on it.
type ScrollToLineMsg struct{ Line int }

// ShowSecretsMsg asks the shell to bring the Secrets tab to the front. A path
// is an address, so ":goto" works from wherever the user is, and the tab that
// answers it cannot switch to itself.
type ShowSecretsMsg struct{}

// OpenCmdlineMsg asks the shell for its command line. The palette is the
// shell's, and ":" reaches it from inside a tab's own modal.
type OpenCmdlineMsg struct{}

// RecordedMsg is a write this session performed, on its way to the Activity
// tab. It travels through the parent because a tab does not name another tab,
// and it carries what happened and never what was written.
type RecordedMsg struct {
	Op, Path, Detail string
	Err              error
}

// JumpToPolicyMsg is the Auth tab reaching a policy by name. It goes through
// the parent because a tab importing a tab is the coupling this whole change
// exists to remove: Auth says what it wants, and the parent knows who answers.
type JumpToPolicyMsg struct{ Name string }

// Emit turns messages into a command. A key often has more than one effect -
// the cursor moved and the pane must go back to its first line - and this is
// how a tab answers with both without knowing what either one costs.
func Emit(msgs ...tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(msgs))
	for _, msg := range msgs {
		cmds = append(cmds, func() tea.Msg { return msg })
	}
	return tea.Batch(cmds...)
}
