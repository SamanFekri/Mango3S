package api

import (
	"context"
	"time"

	"mango3s/internal/monitor"
)

// Server monitor (Explorer › connection › Server monitor): live rates from
// serverStatus, replica set state, and running operations with killOp.

// MonitorEvent is pushed as "monitor:sample" while a Monitor tab is open.
type MonitorEvent struct {
	ConnID string         `json:"connId"`
	Sample monitor.Sample `json:"sample"`
}

// StartMonitor samples connID every intervalSecs (1–5) and returns the history
// kept so far (up to 15 minutes). Samples then arrive as "monitor:sample".
func (a *App) StartMonitor(connID string, intervalSecs int) (monitor.Snapshot, error) {
	snap, err := a.monitor.Start(connID, time.Duration(intervalSecs)*time.Second)
	return snap, a.encode("StartMonitor", err)
}

// StopMonitor stops sampling connID (its Monitor tab closed).
func (a *App) StopMonitor(connID string) { a.monitor.Stop(connID) }

// GetServerOverview reads the server description and replica set status.
func (a *App) GetServerOverview(connID string) (monitor.Overview, error) {
	return call(a, "GetServerOverview", "", func(ctx context.Context) (monitor.Overview, error) {
		sess, err := a.conns.Session(connID)
		if err != nil {
			return monitor.Overview{}, err
		}
		return monitor.ReadOverview(ctx, sess.Client)
	})
}

// ListCurrentOps lists the operations running now, longest first.
func (a *App) ListCurrentOps(connID string, f monitor.OpsFilter) ([]monitor.Op, error) {
	return call(a, "ListCurrentOps", "", func(ctx context.Context) ([]monitor.Op, error) {
		return a.monitor.CurrentOps(ctx, connID, f)
	})
}

// KillOperation stops a running operation; confirm must repeat its ID.
// Refused on read-only connections.
func (a *App) KillOperation(connID, opid, confirm string) error {
	_, err := call(a, "KillOperation", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.monitor.KillOp(ctx, connID, opid, confirm)
	})
	return err
}
