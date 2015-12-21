package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type Stats struct {
	RequestsCount     int64
	ActiveConnections int64
	//UncacheableCount	  int64
	//CacheHitsCount        int64
	//CacheMissesCount      int64
	BytesReadFromUpstream int64
	BytesSentToClients    int64

	UpBandwidth   int64
	DownBandwidth int64
	PeriodHits    int64
	PeriodMisses  int64
}

var g_stats Stats

//
func updateStats(quit chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			upBandwidth := g_upBandwidthManager.CalcBandwidth()
			atomic.StoreInt64(&g_stats.UpBandwidth, int64(upBandwidth))

			downBandwidth := g_downBandwidthManager.CalcBandwidth()
			atomic.StoreInt64(&g_stats.DownBandwidth, int64(downBandwidth))

			hits, misses := g_hitratioManager.CalcHitratio()
			atomic.StoreInt64(&g_stats.PeriodHits, hits)
			atomic.StoreInt64(&g_stats.PeriodMisses, misses)

			logMessage("upBandwidth=%d, downBandwidth=%d, periodHits=%d, periodMisses=%d", upBandwidth, downBandwidth, hits, misses)

		case <-quit:
			ticker.Stop()
			return
		}
	}
}

func UsageToString(size int64) string {
	if size >= 1024*1024 {
		return fmt.Sprintf("%1.1f MB", float32(size)/1024/1024)
	} else if size >= 1024 {
		return fmt.Sprintf("%1.1f KB", float32(size)/1024)
	} else {
		return fmt.Sprintf("%d B", size)
	}
}

func (s *Stats) WriteToStream(w io.Writer) {
	fmt.Fprintf(w, "Command-line flags:\n")
	flag.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(w, "  %s=%v\n", f.Name, f.Value)
	})
	fmt.Fprintf(w, "\n")

	//requestsCount := s.CacheHitsCount + s.CacheMissesCount + s.IfNoneMatchHitsCount
	//requestsCount := s.UncacheableCount + s.CacheHitsCount + s.CacheMissesCount + s.IfNoneMatchHitsCount
	requestsCount := s.RequestsCount

	var cacheHitRatio float64
	if requestsCount > 0 {
		//cacheHitRatio = float64(s.CacheHitsCount+s.IfNoneMatchHitsCount) / float64(requestsCount) * 100.0
	}

	fmt.Fprintf(w, "Requests count: %d\n", requestsCount)
	fmt.Fprintf(w, "Cache hit ratio: %.3f%%\n", cacheHitRatio)
	/*fmt.Fprintf(w, "Uncacheable: %d\n", s.UncacheableCount)
	fmt.Fprintf(w, "Cache hits: %d\n", s.CacheHitsCount)
	fmt.Fprintf(w, "Cache misses: %d\n", s.CacheMissesCount)
	fmt.Fprintf(w, "If-None-Match hits: %d\n", s.IfNoneMatchHitsCount)
	fmt.Fprintf(w, "Read from upstream: %s\n", UsageToString(s.BytesReadFromUpstream))*/
	fmt.Fprintf(w, "Sent to clients: %s\n", UsageToString(s.BytesSentToClients))
	fmt.Fprintf(w, "Upstream traffic saved: %s\n", UsageToString(s.BytesSentToClients-s.BytesReadFromUpstream))
	//fmt.Fprintf(w, "Upstream requests saved: %d\n", s.CacheHitsCount+s.IfNoneMatchHitsCount)

	//
	fmt.Fprintf(w, "\n")

	items := GetStats()
	for _, item := range items {
		fmt.Fprintf(w, "%s PlaylistCache stats (playlist.m3u8)\n", item.Name)
		item.PlaylistStats.WriteToStream(w)
		fmt.Fprintf(w, "%s ManifestCache stats (chunklist*.m3u8)\n", item.Name)
		item.ManifestStats.WriteToStream(w)
		fmt.Fprintf(w, "%s ChunkCache stats (*.ts)\n", item.Name)
		item.ChunkStats.WriteToStream(w)
	}
}

type CacheStatsJson struct {
	CacheCap     int64
	Duration     int64
	CacheObjects int

	CacheUsed   int64
	CacheMisses int64
	CacheHits   int64
}

func (stats CacheStatsJson) WriteToStream(w io.Writer) {
	fmt.Fprintf(w, "  Capacity: %d MB\n", stats.CacheCap)
	fmt.Fprintf(w, "  Duration: %d seconds\n", stats.Duration)

	fmt.Fprintf(w, "  Objects: %d\n", stats.CacheObjects)

	fmt.Fprintf(w, "  Used: %s\n", UsageToString(stats.CacheUsed))
	fmt.Fprintf(w, "  Misses: %d\n", stats.CacheMisses)
	fmt.Fprintf(w, "  Hits: %d\n", stats.CacheHits)
}

func cacheStatsToJson(cache CacheManager, json *CacheStatsJson) {

	json.CacheCap = atomic.LoadInt64(&cache.cacheCap)
	json.Duration = atomic.LoadInt64(&cache.cacheDuration)

	cache.tasksMutex.Lock()
	json.CacheObjects = len(cache.tasks)
	cache.tasksMutex.Unlock()

	json.CacheUsed = atomic.LoadInt64(&cache.stat.cacheUsed)
	json.CacheMisses = atomic.LoadInt64(&cache.stat.cacheMisses)
	json.CacheHits = atomic.LoadInt64(&cache.stat.cacheHits)
}

type ClientStatsJson struct {
	Name string

	PlaylistStats CacheStatsJson
	ManifestStats CacheStatsJson
	ChunkStats    CacheStatsJson
}

func GetStats() []ClientStatsJson {
	items := make([]ClientStatsJson, 0)

	for _, client := range upstreamClients {
		item := ClientStatsJson{
			Name: client.name,

			PlaylistStats: CacheStatsJson{},
			ManifestStats: CacheStatsJson{},
			ChunkStats:    CacheStatsJson{},
		}

		cacheStatsToJson(client.playlistCache, &item.PlaylistStats)
		cacheStatsToJson(client.manifestCache, &item.ManifestStats)
		cacheStatsToJson(client.chunkCache, &item.ChunkStats)

		items = append(items, item)
	}

	return items
}

func (s *Stats) WriteJsonToStream(w io.Writer) {

	stats := struct {
		Settings    AppSettings
		Stats       Stats
		ClientStats []ClientStatsJson
	}{
		Settings:    Settings,
		Stats:       *s,
		ClientStats: GetStats(),
	}

	//buf, _ := json.Marshal(stats)
	buf, _ := json.MarshalIndent(stats, "", "    ")

	w.Write(buf)
}
