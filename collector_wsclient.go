package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func DoWebsocket() {
	if Settings.CollectorServer == "" {
		log.Printf("collectorServer is not set")
		return
	}

	client := wsclient{}
	if client.Name == "" {
		client.Name = Settings.CollectorName
	}
	if client.Name == "" {
		client.Name = g_GullfireId
	}	

	for {
		err := client.DoWebsocketOnce()
		if err != nil {
			log.Printf("failed to send stats to the RTCollector. %v\n", err)
		}

		// retry in 10 seconds
		log.Printf("retry to connect to the RTColelctor in 10 seconds\n")
		time.Sleep(time.Second * 10)
	}
}

type StatsItem struct {
	Stats       Stats
	ClientStats []ClientStatsJson
}

type messageItem struct {
	Name    string
	Product string
	Version string

	StatsItem
}

type wsclient struct {
	Name string
}

func (client *wsclient) readPump(ws *websocket.Conn) {
	defer ws.Close()

	ws.SetPongHandler(func(pongData string) error {
		if Settings.debug {
			log.Printf("RTCollector Pong: %v\n", pongData)
		}
		return nil
	})

	for {
		//mtype, buf, err := ws.ReadMessage()
		_, _, err := ws.ReadMessage()
		if err != nil {
			return
		}

		/*if mtype == websocket.TextMessage {
			//log.Printf("%s: %s\n", name, string(buf))
			//log.Printf("%s\n", string(buf))

			var m messageItem
			err := json.Unmarshal(buf, &m)
			if err == nil {
				WriteCCByName(m.Name, m.Item)
			} else {
				log.Println(err)
			}
		} else {
			log.Printf("receive type=%d, len=%d\n", mtype, len(buf))
		}*/
	}
}

func (client *wsclient) writePump(ws *websocket.Conn) {
	defer ws.Close()

	pingTick := time.Tick(time.Second * 30)
	interval := time.Tick(time.Second * 2)
	counter := 0
	pingCounter := 0
	for {
		select {
		case <-pingTick:
			pingCounter++
			pingData := fmt.Sprintf("%d", pingCounter)

			err := ws.WriteMessage(websocket.PingMessage, []byte(pingData))
			if err != nil {
				return
			}
			if Settings.debug {
				log.Printf("RTCollector Ping: %v\n", pingData)
			}
		case <-interval:
			counter++

			message := messageItem{
				Name:    client.Name,
				Product: "Gullfire",
				Version: "1.0",

				StatsItem: StatsItem{
					Stats:       g_stats,
					ClientStats: GetStats(),
				},
			}

			buf, err := json.Marshal(message)
			if err == nil {
				err := ws.WriteMessage(websocket.TextMessage, buf)
				if err != nil {
					if Settings.debug {
						log.Printf("%v", err)
					}
					return
				}

				log.Printf("sent stats to the RTCollector %d", counter)
			} else {
				if Settings.debug {
					log.Printf("%v", err)
				}
			}
		}
	}

	//wsConn.WriteMessage(websocket.TextMessage, []byte(name))
}

func (client *wsclient) DoWebsocketOnce() error {
	//u, err := url.Parse("http://localhost:90/ws")

	var strUrl string
	if strings.HasSuffix(Settings.CollectorServer, "/") {
		strUrl = fmt.Sprintf("%sws", Settings.CollectorServer)
	} else {
		strUrl = fmt.Sprintf("%s/ws", Settings.CollectorServer)
	}
	if !strings.HasPrefix(strUrl, "http://") {
		strUrl = "http://" + strUrl
	}

	u, err := url.Parse(strUrl)
	if err != nil {
		return err
	}

	dialAddress := u.Host
	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		dialAddress = net.JoinHostPort(u.Host, "80")
	}
	rawConn, err := net.Dial("tcp", dialAddress)
	if err != nil {
		return err
	}

	wsHeaders := http.Header{
	//"Origin":                   {"http://localhost:90"},
	// your milage may differ
	//"Sec-WebSocket-Extensions": {"permessage-deflate; client_max_window_bits, x-webkit-deflate-frame"},
	}

	wsConn, resp, err := websocket.NewClient(rawConn, u, wsHeaders, 1024, 1024)
	if err != nil {
		return fmt.Errorf("websocket.NewClient Error: %s\nResp:%+v", err, resp)
	}

	if Settings.debug {
		log.Printf("connected to the RTCollector through %v -> %v\n", wsConn.LocalAddr(), wsConn.RemoteAddr())
	}

	go client.readPump(wsConn)
	client.writePump(wsConn)

	return nil
}
