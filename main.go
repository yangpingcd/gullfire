package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "net/http/pprof"
	"os"

	"github.com/kardianos/service"
	"github.com/vharitonsky/iniflags"
	"gopkg.in/yangpingcd/lumberjack.v2"
)

var (
	g_GullfireId     = ""
	perIpConnTracker = createPerIpConnTracker()
	//upstreamClient   http.Client

	upstreamClients []UpstreamClient = make([]UpstreamClient, 0)
)

func initGullfireId() {
	g_GullfireId = Settings.id
	if g_GullfireId == "" {
		g_GullfireId, _ = os.Hostname()
	}
}

func initUpstreamClients() {
	/*defer func() {
		//
		client := NewUpstreamClient("", Settings.upstreamHost)
		upstreamClients = append(upstreamClients, client)
	}()*/

	for _, upstream := range Settings.Upstreams {
		client := NewUpstreamClient(upstream.Name, upstream.UpstreamHost)
		upstreamClients = append(upstreamClients, client)
	}

	/*if Settings.upstreamFile == "" {
		return
	}

	f, err := os.Open(Settings.upstreamFile)
	if err != nil {
		logMessage("Failed to open the upstreamFile \"%s\"", Settings.upstreamFile)
		//os.Exit(-1)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Trim(line, " ")
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			// ignore the comment which starts with #;
			continue
		}

		if pos := strings.Index(line, "="); pos >= 0 {
			serviceName := strings.Trim(line[0:pos], "")
			upstreamHost := strings.Trim(line[pos+1:], "")

			client := NewUpstreamClient(serviceName, upstreamHost)
			upstreamClients = append(upstreamClients, client)
		}
	}*/
}

var svcFlag = flag.String("service", "", "Control the system service.")
var accessLog *AccessLog

func main() {
	//flag.Parse()
	iniflags.Parse()
	initGullfireId()
	initHeaders()

	svcConfig := &service.Config{
		Name:        "gullfire",
		DisplayName: "Sliq Gullfire Service",
		Description: "Proxy cache server for Live HLS streams",
		Arguments:   []string{},
	}
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "-service=") {
			svcConfig.Arguments = append(svcConfig.Arguments, arg)
		}
	}
	/*fmt.Println(svcConfig.Arguments)
	return*/

	// intiailze the generic log file
	if Settings.LogSetting.Filename != "" {
		log.SetOutput(&lumberjack.Logger{
			Filename:   Settings.LogSetting.Filename,
			MaxSize:    Settings.LogSetting.MaxSize,
			MaxBackups: Settings.LogSetting.MaxBackups,
			MaxAge:     Settings.LogSetting.MaxAge,
			LocalTime:  Settings.LogSetting.LocalTime,
		})
	}
	log.Println("======================================================================")

	// initialize the access log file
	if true {
		accessLog = NewAccessLog(Settings.AccessLogSetting)
	}

	initUpstreamClients()
	for _, client := range upstreamClients {
		logMessage("upstreamClient \"%s\": \"%s\"", client.name, client.upstreamHost)
	}

	runtime.GOMAXPROCS(Settings.GoMaxProcs)

	//c := make(chan os.Signal, 1)
	//signal.Notify(c, os.Interrupt)
	//<-c
	//logMessage("ctrl-c is captured")

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	errs := make(chan error, 5)
	logger, err = s.Logger(errs)
	if err != nil {
		log.Fatal(err)
	}

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}

	go func() {
		for {
			err := <-errs
			if err != nil {
				log.Print(err)
			}
		}
	}()

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
	//Start(nil)
}

func Start(stop chan struct{}) error {
	quitAccessLog := make(chan struct{})
	go accessLog.StartAccessLog(quitAccessLog)

	var addr string
	for _, addr = range strings.Split(Settings.httpsListenAddrs, ",") {
		go serveHttps(addr)
	}
	for _, addr = range strings.Split(Settings.listenAddrs, ",") {
		go serveHttp(addr)
	}

	go manageCache()

	go DoWebsocket()

	quitUpdateStats := make(chan struct{})
	go updateStats(quitUpdateStats)

	// performance
	/*go func() {
		//http.Server.ListenAndServe()
		http.ListenAndServe("localhost:9000", nil)
	}()*/

	for {
		select {
		case <-stop:
			return nil
		case <-time.After(1 * time.Second):
		}
	}
}

func getAvailableMemoryPercent() float64 {
	var avail = mem_GetAvailable()
	var total = mem_GetTotal()

	if total > 0 {
		avail100 := float64(avail) * 100

		return avail100 / float64(total)
	}

	return 0
}

type KeyDuration struct {
	key      KeyType
	duration time.Duration
}
type ByDuration []KeyDuration

func (kd ByDuration) Len() int {
	return len(kd)
}
func (kd ByDuration) Swap(i, j int) {
	kd[i], kd[j] = kd[j], kd[i]
}
func (kd ByDuration) Less(i, j int) bool {
	return kd[i].duration < kd[j].duration
}

func removeOldChunks() {
	now := time.Now()

	sortKeys := []KeyDuration{}
	removing := []KeyType{}

	// method1
	/*if true {
		chunkCache.cacheMapMutex.Lock()
		for key, item := range(chunkCache.cacheMap) {
			cacheDuration := now.Sub(item.cacheTime)
			if cacheDuration.Seconds() > 3 {
				removing = append(removing, key)
			}
		}
		chunkCache.cacheMapMutex.Unlock()
	}*/

	// method2
	if true {
		for _, client := range upstreamClients {
			chunkCache := client.chunkCache

			chunkCache.tasksMutex.Lock()
			for key, task := range client.chunkCache.tasks {
				//cacheDuration := now.Sub(task.stat.cacheTime)
				cacheTime := atomic.LoadInt64(&task.stat.cacheTime)
				cacheDuration := now.Sub(time.Unix(cacheTime/1e9, cacheTime%1e9))

				sortKeys = append(sortKeys, KeyDuration{key: key, duration: cacheDuration})
			}
			chunkCache.tasksMutex.Unlock()

			// sort descending
			sort.Sort(sort.Reverse(ByDuration(sortKeys)))

			// remove 1/10 of the cache
			for i := 0; i < len(sortKeys)/10; i++ {
				removing = append(removing, sortKeys[i].key)
			}

			count := chunkCache.RemoveTasks(removing)
			if count > 0 {
				logMessage(fmt.Sprintf("delete %d cache items because of low memory", count))
			}
		}
	}
}

// remove the expired items from the cache
func manageCache() {
	//go purgeCacheLockKey()

	lastCheckTime := time.Now()

	for {
		now := time.Now()
		if now.Sub(lastCheckTime) < time.Second*2 {
			var percent = getAvailableMemoryPercent()
			// free os memory check it again
			if percent < 10.0 {
				runtime.GC()
				debug.FreeOSMemory()
				percent = getAvailableMemoryPercent()
			}
			if percent < 10.0 {
				removeOldChunks()
				runtime.GC()
				debug.FreeOSMemory()
			}

			time.Sleep(time.Second * 1)
			//logMessage("%0.2f%% memory available", percent)
			continue
		}

		managers := make([]CacheManager, 0)
		for _, client := range upstreamClients {
			managers = append(managers, client.playlistCache)
			managers = append(managers, client.chunkCache)
			managers = append(managers, client.manifestCache)
		}

		count := 0
		//for _, cache := range([...]CacheManager {playlistCache, chunkCache, manifestCache}) {
		for _, cache := range managers {

			removing := []KeyType{}

			cache.tasksMutex.Lock()
			for key, task := range cache.tasks {

				//task.statLock.Lock()
				//cacheDuration := now.Sub(task.stat.cacheTime)
				//task.statLock.Unlock()

				cacheTime := atomic.LoadInt64(&task.stat.cacheTime)
				cacheDuration := now.Sub(time.Unix(cacheTime/1e9, cacheTime%1e9))

				if cacheDuration.Seconds() > float64(cache.cacheDuration) || cacheDuration < 0 {
					removing = append(removing, key)
				}
			}
			cache.tasksMutex.Unlock()

			count += cache.RemoveTasks(removing)
		}

		if count > 0 {
			logMessage(fmt.Sprintf("delete %d cache items", count))
		}

		lastCheckTime = time.Now()
	}
}

func serveHttps(addr string) {
	if addr == "" {
		return
	}
	cert, err := tls.LoadX509KeyPair(Settings.httpsCertFile, Settings.httpsKeyFile)
	if err != nil {
		logFatal("Cannot load certificate: [%s]", err)
	}
	c := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	ln := tls.NewListener(listen(addr), c)
	logMessage("Listening https on [%s]", addr)
	serve(ln)
}

func serveHttp(addr string) {
	if addr == "" {
		return
	}
	ln := listen(addr)
	logMessage("Listening http on [%s]", addr)
	serve(ln)
}

func listen(addr string) net.Listener {
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		logFatal("Cannot listen [%s]: [%s]", addr, err)
	}
	return ln
}

func serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				logMessage("Cannot accept connections due to temporary network error: [%s]", err)
				time.Sleep(time.Second)
				continue
			}
			logFatal("Cannot accept connections due to permanent error: [%s]", err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	handler := connHandler{}
	handler.handleConnection(conn)
}

func getRequestHost(req *http.Request, upstreamHost string) string {
	if Settings.useClientRequestHost {
		return req.Host
	}
	//return *upstreamHost
	return upstreamHost
}

func logRequestError(req *http.Request, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logMessage("%s - %s - %s - %s. %s", req.RemoteAddr, req.RequestURI, req.Referer(), req.UserAgent(), msg)
}

func logMessage(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s\n", msg)
}

func logFatal(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Fatalf("%s\n", msg)
}

func ip4ToUint32(ip4 net.IP) uint32 {
	return (uint32(ip4[0]) << 24) | (uint32(ip4[1]) << 16) | (uint32(ip4[2]) << 8) | uint32(ip4[3])
}

type PerIpConnTracker struct {
	mutex          sync.Mutex
	perIpConnCount map[uint32]int
}

func (ct *PerIpConnTracker) registerIp(ipUint32 uint32) int {
	ct.mutex.Lock()
	ct.perIpConnCount[ipUint32] += 1
	connCount := ct.perIpConnCount[ipUint32]
	ct.mutex.Unlock()
	return connCount
}

func (ct *PerIpConnTracker) unregisterIp(ipUint32 uint32) {
	ct.mutex.Lock()
	ct.perIpConnCount[ipUint32] -= 1
	ct.mutex.Unlock()
}

func createPerIpConnTracker() *PerIpConnTracker {
	return &PerIpConnTracker{
		perIpConnCount: make(map[uint32]int),
	}
}
