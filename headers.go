package main

import (
	"fmt"
)

type CodeHeader struct {
	code   int
	header []byte
}

var (
	//okResponseHeader                  = CodeHeader{200, []byte("HTTP/1.1 200 OK\r\nServer: proxy2\r\nCache-Control: public, max-age=31536000\r\nETag: W/\"CacheForever\"\r\n")}

	internalServerErrorResponseHeader = CodeHeader{500, []byte("HTTP/1.1 500 Internal Server Error\r\nServer: gullfire\r\n")}
	notAllowedResponseHeader          = CodeHeader{405, []byte("HTTP/1.1 405 Method Not Allowed\r\nServer: gullfire\r\n")}
	okResponseHeader                  = CodeHeader{200, []byte("HTTP/1.1 200 OK\r\nServer: gullfire\r\nCache-Control: no-cache\r\n")}
	serviceUnavailableResponseHeader  = CodeHeader{503, []byte("HTTP/1.1 503 Service Unavailable\r\nServer: gullfire\r\n")}
)

func initHeaders() {
	internalServerErrorResponseHeader.header =
		[]byte(fmt.Sprintf("HTTP/1.1 500 Internal Server Error\r\nServer: gullfire-%s\r\n", g_GullfireId))
	notAllowedResponseHeader.header =
		[]byte(fmt.Sprintf("HTTP/1.1 405 Method Not Allowed\r\nServer: gullfire-%s\r\n", g_GullfireId))
	okResponseHeader.header =
		//[]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nServer: gullfire-%s\r\nCache-Control: public, max-age=31536000\r\nETag: W/\"CacheForever\"\r\n", g_GullfireId))
		[]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nServer: gullfire-%s\r\nCache-Control: no-cache\r\n", g_GullfireId))
	serviceUnavailableResponseHeader.header =
		[]byte(fmt.Sprintf("HTTP/1.1 503 Service Unavailable\r\nServer: gullfire-%s\r\n", g_GullfireId))
}
