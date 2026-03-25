package service

import (
	"context"
	"sort"
	"sync"
)

type skuGuard struct {
	limit int
	locks sync.Map
}

func newSKUGuard(limit int) *skuGuard {
	return &skuGuard{limit: limit}
}

func (g *skuGuard) Acquire(ctx context.Context, skuIDs []int64) (func(), error) {
	if g == nil || g.limit <= 0 || len(skuIDs) == 0 {
		return func() {}, nil
	}
	ordered := dedupeSortedSKUIDs(skuIDs)
	acquired := make([]chan struct{}, 0, len(ordered))
	for _, skuID := range ordered {
		sem := g.semaphoreFor(skuID)
		select {
		case sem <- struct{}{}:
			acquired = append(acquired, sem)
		case <-ctx.Done():
			for i := len(acquired) - 1; i >= 0; i-- {
				<-acquired[i]
			}
			return nil, ctx.Err()
		}
	}
	return func() {
		for i := len(acquired) - 1; i >= 0; i-- {
			<-acquired[i]
		}
	}, nil
}

func (g *skuGuard) semaphoreFor(skuID int64) chan struct{} {
	if sem, ok := g.locks.Load(skuID); ok {
		return sem.(chan struct{})
	}
	sem := make(chan struct{}, g.limit)
	actual, _ := g.locks.LoadOrStore(skuID, sem)
	return actual.(chan struct{})
}

func dedupeSortedSKUIDs(skuIDs []int64) []int64 {
	if len(skuIDs) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(skuIDs))
	result := make([]int64, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		if _, ok := seen[skuID]; ok {
			continue
		}
		seen[skuID] = struct{}{}
		result = append(result, skuID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
