package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type connHandler struct {
	logItem LogItem
}

func (handler *connHandler) handleConnection(conn net.Conn) {
	atomic.AddInt64(&g_stats.ActiveConnections, 1)
	defer func() {
		atomic.AddInt64(&g_stats.ActiveConnections, -1)
		conn.Close()
	}()

	//clientAddr := conn.RemoteAddr().(*net.TCPAddr).IP.To4()
	/*ipUint32 := ip4ToUint32(clientAddr)
	if perIpConnTracker.registerIp(ipUint32) > *maxConnsPerIp {
		logMessage("Too many concurrent connections (more than %d) from ip=%s. Denying new connection from the ip", *maxConnsPerIp, clientAddr)
		perIpConnTracker.unregisterIp(ipUint32)
		//return
	}
	defer perIpConnTracker.unregisterIp(ipUint32)*/

	localAddrStr := conn.LocalAddr().String()
	if pos := strings.Index(localAddrStr, ":"); pos >= 0 {
		handler.logItem.s_ip = localAddrStr[:pos]
		port, err := strconv.Atoi(localAddrStr[pos+1:])
		if err == nil {
			handler.logItem.s_port = port
		}
	} else {
		handler.logItem.s_ip = localAddrStr
		handler.logItem.s_port = 80
	}

	clientAddrStr := conn.RemoteAddr().String()
	if pos := strings.Index(clientAddrStr, ":"); pos >= 0 {
		handler.logItem.c_ip = clientAddrStr[:pos]
	} else {
		handler.logItem.c_ip = clientAddrStr
	}

	r := bufio.NewReaderSize(conn, Settings.readBufferSize)
	w := bufio.NewWriterSize(conn, Settings.writeBufferSize)
	for {
		req, err := http.ReadRequest(r)
		if err != nil {
			if err != io.EOF {
				logMessage("Error when reading http request from %s: [%s]", clientAddrStr, err)
			}
			return
		}
		req.RemoteAddr = clientAddrStr
		ok := handler.handleRequest(req, w)
		w.Flush()
		if !ok || !req.ProtoAtLeast(1, 1) || req.Header.Get("Connection") == "close" {
			//if !ok || !req.ProtoAtLeast(1, 0) || req.Header.Get("Connection") == "close" {
			return
		}
	}
}

//var chunkRegExp, _ = regexp.Compile(`(media_).*_(\d*)(\.ts)`)

func (handler *connHandler) handleRequest(req *http.Request, w io.Writer) bool {
	timeStartHandle := time.Now()
	/*defer func() {
		timeEndHandle := time.Now()
		elapse := timeEndRequest.Sub(timeStartHandle)
		logMessage(fmt.Sprintf("spent %v to handleRequest %v", elapse, req.RequestURI))
	}()*/

	//var logItem LogItem = LogItem{}
	var logItem *LogItem = &handler.logItem

	logItem.date = time.Now().UTC()
	//logItem.cs_uri_stem = req.req.RequestURI
	logItem.cs_uri_stem = req.URL.Path
	logItem.cs_uri_query = req.URL.RawQuery
	logItem.cs_method = req.Method
	/*if pos := strings.Index(req.RemoteAddr, ":"); pos >= 0 {
		logItem.s_ip = req.RemoteAddr[:pos]
		port, err := strconv.Atoi(req.RemoteAddr[pos+1:])
		if err == nil {
			logItem.s_port = port
		}
	} else {
		logItem.s_ip = req.RemoteAddr
		logItem.s_port = 80
	}*/

	//req.Header.

	logItem.cs_User_Agent = req.UserAgent()
	logItem.cs_Referer = req.Referer()
	logItem.sc_status = 200
	logItem.sc_substatus = 0
	logItem.sc_bytes = 0
	logItem.cs_bytes = 0
	//logItem.cs_bytes = req.

	defer func(startTime time.Time, logItem *LogItem) {
		elapse := time.Now().Sub(startTime)
		logItem.time_taken = int(elapse.Nanoseconds() / 1000000)
		accessLog.AddItem(*logItem)
	}(timeStartHandle, logItem)

	atomic.AddInt64(&g_stats.RequestsCount, 1)

	logMessage("request %s from %s", req.RequestURI, req.RemoteAddr)

	if req.Method != "GET" {
		w.Write(notAllowedResponseHeader.header)
		w.Write([]byte("\r\n"))

		logItem.sc_status = notAllowedResponseHeader.code
		return false
	}

	if req.RequestURI == "/" {
		body := fmt.Sprintf("Welcome to gullfire %s", g_GullfireId)

		headerExtra := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
		w.Write(okResponseHeader.header)
		w.Write(headerExtra)
		w.Write([]byte(body))

		logItem.sc_bytes += len(okResponseHeader.header) + len(headerExtra)
		logItem.sc_bytes += len(body)

		logItem.sc_status = okResponseHeader.code
		return false
	}
	if req.RequestURI == "/favicon.ico" {
		headerExtra := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", 0))
		w.Write(serviceUnavailableResponseHeader.header)
		w.Write(headerExtra)

		logItem.sc_bytes += len(serviceUnavailableResponseHeader.header) + len(headerExtra)

		logItem.sc_status = serviceUnavailableResponseHeader.code
		return false
	}

	if req.RequestURI == Settings.statsRequestPath {
		var bodyBuffer bytes.Buffer
		bodyWriter := bufio.NewWriter(&bodyBuffer)
		g_stats.WriteToStream(bodyWriter)
		bodyWriter.Flush()
		body := bodyBuffer.Bytes()

		headerExtra := []byte(fmt.Sprintf("Content-Type: text/plain\r\nContent-Length: %d\r\n\r\n", len(body)))
		w.Write(okResponseHeader.header)
		w.Write(headerExtra)
		w.Write(body)

		logItem.sc_bytes += len(okResponseHeader.header) + len(headerExtra)
		logItem.sc_bytes += len(body)

		logItem.sc_status = okResponseHeader.code
		return false
	}

	if req.RequestURI == Settings.statsJsonRequestPath {
		var bodyBuffer bytes.Buffer
		bodyWriter := bufio.NewWriter(&bodyBuffer)
		g_stats.WriteJsonToStream(bodyWriter)
		bodyWriter.Flush()
		body := bodyBuffer.Bytes()

		headerExtra := []byte(fmt.Sprintf("Content-Type: application/json\r\nContent-Length: %d\r\n\r\n", len(body)))
		w.Write(okResponseHeader.header)
		w.Write(headerExtra)
		w.Write(body)

		logItem.sc_bytes += len(okResponseHeader.header) + len(headerExtra)
		logItem.sc_bytes += len(body)

		logItem.sc_status = okResponseHeader.code
		return false
	}

	for _, client := range upstreamClients {
		match := "/" + client.name + "/"
		if strings.HasPrefix(req.RequestURI, match) {
			return client.Handle(req, w, logItem)
		}
	}

	// find the default client
	for _, client := range upstreamClients {
		if client.name == "" {
			return client.Handle(req, w, logItem)
		}
	}

	// no clients to handle this
	if true {
		headerExtra := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", 0))
		w.Write(serviceUnavailableResponseHeader.header)
		w.Write(headerExtra)

		logItem.sc_bytes += len(serviceUnavailableResponseHeader.header) + len(headerExtra)

		logItem.sc_status = serviceUnavailableResponseHeader.code
		return false
	}
	return false
}
