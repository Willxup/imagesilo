package delivery

type Gate struct {
	slots chan struct{}
}

func NewGate(concurrency int) *Gate {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Gate{slots: make(chan struct{}, concurrency)}
}

func (g *Gate) TryAcquire() (func(), bool) {
	select {
	case g.slots <- struct{}{}:
		return func() { <-g.slots }, true
	default:
		return nil, false
	}
}
