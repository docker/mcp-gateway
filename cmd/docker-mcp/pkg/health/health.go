package health

import "sync/atomic"

type State struct {
	healthy atomic.Bool
}

func (h *State) IsHealthy() bool {
	return h.healthy.Load()
}

func (h *State) SetHealthy() {
	h.healthy.Store(true)
}
