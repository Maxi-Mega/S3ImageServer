package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"strconv"
	"time"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run(eventChan <-chan event) {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case evt := <-eventChan:
			// eventMsg := []byte(evt.String())
			eventMsg := evt.Json()
			for client := range h.clients {
				select {
				case client.send <- eventMsg:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

func (c *Client) writer() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(newline)
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		printError(fmt.Errorf("failed to upgrade WS connection: %v", err), false)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writer()
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	deleteCookies(w, r)
	tmpl, err := getIndexWsTemplate()
	if err != nil {
		prettier(w, err.Error(), nil, http.StatusInternalServerError)
		return
	}
	executeTemplate(w, tmpl, templateData{
		Version:                version,
		BasePath:               config.BasePath,
		WindowTitle:            config.WindowTitle,
		ScaleInitialPercentage: config.ScaleInitialPercentage,
		BucketName:             config.S3.BucketName,
		PrefixName:             "config.S3.KeyPrefix",
		Previews:               imagesCache.toEventObjects(),
		// PreviewsWithTime:       imagesCache, TODO: add time to EventObject ?
		PreviewFilename:       config.PreviewFilename,
		FullProductExtension:  config.FullProductExtension,
		KeyPrefix:             "config.S3.KeyPrefix",
		ImageGroups:           config.ImageGroups,
		ImageTypes:            config.imageTypes,
		MaxImagesDisplayCount: config.MaxImagesDisplayCount,
		RetentionPeriod:       config.RetentionPeriod.Seconds(),
		PollingPeriod:         config.PollingPeriod.Seconds(),
	})
}

func reloadHandler(w http.ResponseWriter, r *http.Request, eventChan chan event) {
	printInfo("Reload ...")
	pollMutex.Lock()
	defer pollMutex.Unlock()
	imagesCacheMutex.Lock()
	defer imagesCacheMutex.Unlock()
	timersMutex.Lock()
	defer timersMutex.Unlock()
	geonamesCacheMutex.Lock()
	defer geonamesCacheMutex.Unlock()
	fullProductLinksCacheMutex.Lock()
	defer fullProductLinksCacheMutex.Unlock()

	// delete all cache in the filesystem
	err := clearDir(config.CacheDir)
	if err != nil {
		printError(fmt.Errorf("failed to clear the cache on disk: %v", err), false)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Failed to reload the cache, see the server's console for more details")
		return
	}

	// clear all caches in ram
	imagesCache = S3Images{}
	for timerKey, timer := range timers {
		timer.Stop()
		delete(timers, timerKey)
	}
	geonamesCache = map[string]Geonames{}
	fullProductLinksCache = map[string][]string{}

	// send a reset signal to all the clients through websocket connections
	eventChan <- event{
		EventType: eventReset,
		EventDate: time.Now().String(),
	}

	printInfo("Reload done !")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Reload done !")
}

func startWSServer(port uint16, eventChan chan event) error {
	hub := newHub()
	go hub.run(eventChan)

	http.HandleFunc("/", websocketHandler)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	http.HandleFunc("/image/", imageHandler)
	http.HandleFunc("/images", imagesListHandler)
	http.HandleFunc("/infos/", infosHandler)
	http.HandleFunc("/cache/", cacheHandler)
	http.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		reloadHandler(w, r, eventChan)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent) // for ping
	})

	printInfo("Starting web socket server on port ", port, " ...")
	return http.ListenAndServe(":"+strconv.FormatUint(uint64(port), 10), nil)
}
