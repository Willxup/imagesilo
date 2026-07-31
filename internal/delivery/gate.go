package delivery

type Gate struct {
	slots chan struct{}
}

func NewGate(concurrency int) *Gate {
	if concurrency <= 0 {
		return &Gate{}
	}
	return &Gate{slots: make(chan struct{}, concurrency)}
}

func (g *Gate) TryAcquire() (func(), bool) {
	if g.slots == nil {
		return func() {}, true
	}
	select {
	case g.slots <- struct{}{}:
		return func() { <-g.slots }, true
	default:
		return nil, false
	}
}
