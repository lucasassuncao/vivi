package vault

import "context"

// Health reads sys/health and fills in the parts of ServerInfo describing the
// server rather than the connection. The standby flag earns the request:
// reading from one is a normal way to see data behind the leader.
func (a *apiClient) Health(ctx context.Context) (ServerInfo, error) {
	a.mu.RLock()
	info := a.info
	a.mu.RUnlock()

	h, err := a.c.Sys().HealthWithContext(ctx)
	if err != nil {
		return info, wrap("read health", "sys/health", err)
	}
	if h == nil {
		return info, &Error{Op: "read health", Path: "sys/health", Kind: ErrNotFound}
	}

	info.Version = h.Version
	info.Sealed = h.Sealed
	// A performance standby serves reads locally and forwards writes, so it is
	// a standby for the purpose this flag is read for: what you see may be
	// behind the leader.
	info.Standby = h.Standby || h.PerformanceStandby

	a.mu.Lock()
	a.info = info
	a.mu.Unlock()

	return info, nil
}
