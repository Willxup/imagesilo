package delivery

import "sync"

type Index struct {
	mu   sync.RWMutex
	byID map[string]Target
}

func NewIndex() *Index {
	return &Index{byID: make(map[string]Target)}
}

func (i *Index) Replace(targets map[string]Target) {
	i.mu.Lock()
	i.byID = targets
	i.mu.Unlock()
}

func (i *Index) Add(id string, target Target) {
	i.mu.Lock()
	i.byID[id] = target
	i.mu.Unlock()
}

func (i *Index) UpdateVisibility(id, visibility string) bool {
	i.mu.Lock()
	target, ok := i.byID[id]
	if ok {
		target.Visibility = visibility
		i.byID[id] = target
	}
	i.mu.Unlock()
	return ok
}

func (i *Index) Get(id string) (Target, bool) {
	i.mu.RLock()
	target, ok := i.byID[id]
	i.mu.RUnlock()
	return target, ok
}

func (i *Index) Len() int {
	i.mu.RLock()
	length := len(i.byID)
	i.mu.RUnlock()
	return length
}
