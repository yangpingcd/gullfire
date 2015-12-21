package main

import (
	"fmt"
	"testing"
	"time"
)

func init() {
}

func TestBandwidth(*testing.T) {
	for i := 0; i < 1; i++ {
		g_upBandwidthManager.Update(1000000)
		time.Sleep(time.Second)

		fmt.Printf("%d: bandwidth=%d\n", i, g_upBandwidthManager.CalcBandwidth())
	}

	bandwidth := g_upBandwidthManager.CalcBandwidth()
	fmt.Printf("bandwidth=%d\n", bandwidth)
}
