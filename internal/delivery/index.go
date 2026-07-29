package delivery

import "sync"

type Index struct {
	mu      sync.RWMutex
	byID    map[string]Target
	aliases map[string]string
}

func NewIndex() *Index {
	return &Index{byID: make(map[string]Target), aliases: make(map[string]string)}
}

func (i *Index) Replace(targets map[string]Target) {
	i.ReplaceAll(targets, make(map[string]string))
}

func (i *Index) ReplaceAll(targets map[string]Target, aliases map[string]string) {
	i.mu.Lock()
	i.byID = targets
	i.aliases = aliases
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

func (i *Index) AddAlias(path, imageID string) {
	i.mu.Lock()
	i.aliases[path] = imageID
	i.mu.Unlock()
}

func (i *Index) RemoveAlias(path string) {
	i.mu.Lock()
	delete(i.aliases, path)
	i.mu.Unlock()
}

func (i *Index) Get(id string) (Target, bool) {
	i.mu.RLock()
	target, ok := i.byID[id]
	i.mu.RUnlock()
	return target, ok
}

func (i *Index) GetAlias(path string) (Target, bool) {
	i.mu.RLock()
	id, ok := i.aliases[path]
	if ok {
		var target Target
		target, ok = i.byID[id]
		i.mu.RUnlock()
		return target, ok
	}
	i.mu.RUnlock()
	return Target{}, false
}

func (i *Index) ResolveAlias(path string) (string, bool) {
	i.mu.RLock()
	id, ok := i.aliases[path]
	i.mu.RUnlock()
	return id, ok
}

func (i *Index) Len() int {
	i.mu.RLock()
	length := len(i.byID)
	i.mu.RUnlock()
	return length
}

func (i *Index) AliasLen() int {
	i.mu.RLock()
	length := len(i.aliases)
	i.mu.RUnlock()
	return length
}
