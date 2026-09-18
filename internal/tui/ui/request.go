package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// RequestAnswer marks the messages that settle a request's pending count. It is
// named here because the answers are built by the tabs and counted by the
// parent, so neither of them can own the marker.
type RequestAnswer interface{ AnswersRequest() }

// Requester is how a tab reaches its server. The tab decides what to call; the
// parent decides how a call is wrapped, which is the session context, the
// timeout, and the count of calls still in flight.
//
// Injected rather than asked for by message, because unlike a banner or a
// clipboard write the answer has to come back to the tab that asked. A parent
// that placed the call itself would be back to knowing what a policy is.
type Requester interface {
	Request(call func(ctx context.Context) tea.Msg) tea.Cmd
}
