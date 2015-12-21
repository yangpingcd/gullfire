package main

import (
	"sync"
	"sync/atomic"
	"time"
)

type hitratioManager struct {
	hits   int64
	misses int64

	payloadItemsMutex sync.Mutex
	payloadItems      []hitratioPayloadItem
}

func CreateHitratioManager() *hitratioManager {
	manager := hitratioManager{
		hits:   0,
		misses: 0,

		payloadItemsMutex: sync.Mutex{},
		payloadItems:      make([]hitratioPayloadItem, 0),
	}

	return &manager
}

type hitratioPayloadItem struct {
	//payloadSize int
	hits   int64
	misses int64

	timestamp time.Time
}

var hitratioPeriod = 60 * time.Second

func (m *hitratioManager) Update() {
	m.payloadItemsMutex.Lock()
	defer m.payloadItemsMutex.Unlock()

	now := time.Now()

	if len(m.payloadItems) > 0 {
		lastItem := m.payloadItems[len(m.payloadItems)-1]
		if lastItem.timestamp.After(now) {
			// empty all the items
			m.payloadItems = make([]hitratioPayloadItem, 0)
		}

		startIndex := 0
		var curItem hitratioPayloadItem
		for startIndex, curItem = range m.payloadItems {
			compareTime := curItem.timestamp.Add(hitratioPeriod)
			if compareTime.After(now) {
				break
			}
		}

		if startIndex > 0 {
			m.payloadItems = m.payloadItems[startIndex:]
		}
	}

	hits := atomic.SwapInt64(&m.hits, 0)
	misses := atomic.SwapInt64(&m.misses, 0)
	item := hitratioPayloadItem{
		hits:      hits,
		misses:    misses,
		timestamp: now,
	}

	m.payloadItems = append(m.payloadItems, item)
}

func (m *hitratioManager) CalcHitratio() (hits int64, misses int64) {
	m.Update()

	m.payloadItemsMutex.Lock()
	defer m.payloadItemsMutex.Unlock()

	//now := time.Now()

	hits = int64(0)
	misses = int64(0)
	for _, curItem := range m.payloadItems {
		hits += curItem.hits
		misses += curItem.misses
	}

	/*hitratio := float64(0)
	if totalHits+totalMisses > 0 {
		hitratio = float64(totalHits) * 100 / float64(totalHits+totalMisses)
	}*/

	/*count := len(m.payloadItems)
	if count > 1 {
		//period := m.payloadItems[count-1].timestamp.Sub(m.payloadItems[0].timestamp)
		period := bandwidthPeriod
		if period > time.Second {
			b := float32(totalSize) * float32(time.Second) / float32(period)
			bandwidth = int(b)
		}
	}
	//*/

	//return hitratio
	return
}

var g_hitratioManager = CreateHitratioManager()
