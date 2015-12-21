package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

type UpstreamClient struct {
	name         string
	upstreamHost string

	playlistCache CacheManager
	chunkCache    CacheManager
	manifestCache CacheManager
}

func NewUpstreamClient(name string, upstreamHost string) UpstreamClient {
	client := UpstreamClient{}

	client.name = name
	client.upstreamHost = upstreamHost

	prefix := "/" + name

	var err error
	cacheSize := int64(10)
	if cacheSize <= 0 {
		// todo, auto
		cacheSize = 10
	}
	client.playlistCache, err = CreateCacheManager(prefix, upstreamHost, cacheSize, 3*60) // 3 minutes
	if err != nil {
		logFatal("Failed to create the playlist cache")
	}
	//defer playlistCache.Close()

	cacheSize = Settings.ChunkCacheSize
	if cacheSize <= 0 {
		// todo, auto
		cacheSize = 100
	}
	client.chunkCache, err = CreateCacheManager(prefix, upstreamHost, cacheSize, 30) // 30 seconds
	if err != nil {
		logFatal("Failed to create the chunk cache")
	}
	//defer chunkCache.Close()

	cacheSize = Settings.ManifestCacheSize
	if cacheSize <= 0 {
		// todo, auto
		cacheSize = 10
	}
	client.manifestCache, err = CreateCacheManager(prefix, upstreamHost, cacheSize, 2) // 2 seconds
	if err != nil {
		logFatal("Failed to create the manifest cache")
	}
	//defer manifestCache.Close()

	return client
}

var chunkRegExp, _ = regexp.Compile(`(media).*(_\d*)(\.ts)`)

func (client *UpstreamClient) Handle(req *http.Request, w io.Writer, logItem *LogItem) bool {
	err := error(nil)
	uri := req.RequestURI // ReqeustURI includes query

	// only calculate hitratio for .ts
	fDoHitratio := false

	// by default use the chunkCache
	cache := client.chunkCache

	var keyBuf []byte
	if chunkRegExp.MatchString(uri) {
		fDoHitratio = true

		newuri := chunkRegExp.ReplaceAllString(uri, "$1$2$3")
		logMessage("%s to %s", uri, newuri)

		strKey := fmt.Sprintf("%s%s", getRequestHost(req, client.upstreamHost), newuri)
		keyBuf = []byte(strKey)
	} else if strings.HasSuffix(req.URL.Path, ".m3u8") {
		newuri := uri

		manifestRegExp, _ := regexp.Compile(`chunklist.*\.m3u8`)
		if manifestRegExp.MatchString(uri) {
			newuri = manifestRegExp.ReplaceAllString(uri, "chunklist.m3u8")
			cache = client.manifestCache
		} else {
			// we treat playlist.m3u8 as chunk cache because it will not be changed
			cache = client.playlistCache
		}

		strKey := fmt.Sprintf("%s%s", getRequestHost(req, client.upstreamHost), newuri)
		keyBuf = []byte(strKey)
	}
	key := CalcKey(keyBuf) //KeyType(0)

	task, taskExist := cache.AddTask(key, req)
	if fDoHitratio {
		if taskExist {
			atomic.AddInt64(&g_hitratioManager.hits, 1)
		} else {
			atomic.AddInt64(&g_hitratioManager.misses, 1)
		}
	}

	var item CacheItem

	select {
	case item = <-task.chItem:
		break
	case <-time.After(30 * time.Second):
		// timeout
		logMessage("timeout on request %s", uri)
		break
	}

	var bytesSent = 0
	defer func() {
		atomic.AddInt64(&g_stats.BytesSentToClients, int64(bytesSent))
		g_upBandwidthManager.Update(bytesSent)
		logItem.sc_bytes += int(bytesSent)
	}()

	/*item := fetchIt(req, cache, key)
	if item == nil {
		w.Write(serviceUnavailableResponseHeader)
		return false
	}*/
	// it is in the cache, but error
	if item.contentBody == nil {
		headerExtra := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", 0))
		w.Write(serviceUnavailableResponseHeader.header)
		w.Write(headerExtra)

		bytesSent += len(serviceUnavailableResponseHeader.header) + len(headerExtra)

		logItem.sc_status = serviceUnavailableResponseHeader.code
		return false
	}

	/*if item.contentType == nil {
		w.Write(internalServerErrorResponseHeader)
		return false
	}*/

	if _, err = w.Write(okResponseHeader.header); err != nil {
		return false
	}
	bytesSent += len(okResponseHeader.header)

	headerExtra := ""
	if taskExist {
		headerExtra = fmt.Sprintf("%sX-Cache: HIT from %s\r\n", headerExtra, g_GullfireId)
	} else {
		headerExtra = fmt.Sprintf("%sX-Cache: MISS from %s\r\n", headerExtra, g_GullfireId)
	}

	headerExtra = fmt.Sprintf("%sContent-Type: %s\r\nContent-Length: %d\r\n\r\n", headerExtra, item.contentType, item.Size())
	if _, err = w.Write([]byte(headerExtra)); err != nil {
		return false
	}
	bytesSent += len(headerExtra)

	var bytesWritten int64 = 0
	if bytesWritten, err = item.WriteTo(w); err != nil {
		logRequestError(req, "Cannot send file with key=[%v] to client: %s", key, err)
		return false
	}
	bytesSent += int(bytesWritten)

	return true
}
