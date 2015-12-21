package main

import (
	"sync"
	"time"
)

type bandwidthManager struct {
	//bandwidth int

	payloadItemsMutex sync.Mutex
	payloadItems      []payloadItem
}

func CreateBandwidthManager() *bandwidthManager {
	manager := bandwidthManager{
		//bandwidth:
		payloadItemsMutex: sync.Mutex{},
		payloadItems:      make([]payloadItem, 0),
	}

	return &manager
}

type payloadItem struct {
	payloadSize int
	timestamp   time.Time
}

var bandwidthPeriod = 10 * time.Second

func (m *bandwidthManager) Update(payloadSize int) {
	m.payloadItemsMutex.Lock()
	defer m.payloadItemsMutex.Unlock()

	now := time.Now()

	if len(m.payloadItems) > 0 {
		lastItem := m.payloadItems[len(m.payloadItems)-1]
		if lastItem.timestamp.After(now) {
			// empty all the items
			m.payloadItems = make([]payloadItem, 0)
		}

		startIndex := 0
		var curItem payloadItem
		for startIndex, curItem = range m.payloadItems {
			compareTime := curItem.timestamp.Add(bandwidthPeriod)
			if compareTime.After(now) {
				break
			}
		}

		if startIndex > 0 {
			m.payloadItems = m.payloadItems[startIndex:]
		}
	}

	item := payloadItem{
		payloadSize: payloadSize,
		timestamp:   now,
	}

	m.payloadItems = append(m.payloadItems, item)
}

func (m *bandwidthManager) CalcBandwidth() int {
	m.Update(0)

	m.payloadItemsMutex.Lock()
	defer m.payloadItemsMutex.Unlock()

	//now := time.Now()

	totalSize := int64(0)
	for _, curItem := range m.payloadItems {
		totalSize += int64(curItem.payloadSize)
	}

	bandwidth := int(totalSize)
	count := len(m.payloadItems)
	if count > 1 {
		//period := m.payloadItems[count-1].timestamp.Sub(m.payloadItems[0].timestamp)
		period := bandwidthPeriod
		if period > time.Second {
			b := float32(totalSize) * float32(time.Second) / float32(period)
			bandwidth = int(b)
		}
	}
	//

	return bandwidth
}

var g_upBandwidthManager = CreateBandwidthManager()
var g_downBandwidthManager = CreateBandwidthManager()
