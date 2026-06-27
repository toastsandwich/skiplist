package skiplist

import "sync"

type pool[T comparable] struct {
	new func() T
	p   sync.Pool
}

func (p *pool[T]) Get() T {
	v, ok := p.p.Get().(T)
	if ok {
		return v
	}

	if p.new == nil {
		var t T
		return t
	}

	return p.new()
}

func (p *pool[T]) Put(t T) {
	var empty T
	if t != empty {
		p.p.Put(t)
	}
}
