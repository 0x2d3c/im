package v0

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WebsocketMgr manages WebSocket connections and messages.
type WebsocketMgr struct {
	log        *slog.Logger       // Logger for handling logs.
	users      map[string]*User   // A map of users, indexed by user ID.
	messageCh  chan *Message      // A channel for sending and receiving messages.
	upgrader   websocket.Upgrader // Upgrader for upgrading HTTP connections to WebSocket.
	messageBuf sync.Pool          // Pool for managing reusable Message objects.
	done       chan struct{}      // A channel to signal the completion of the messageDispatcher.

	sync.RWMutex // Read-Write Mutex for concurrent access protection.
}

// User represents a user and their associated WebSocket connections.
type User struct {
	ID      string                     // User's unique identifier.
	devices map[string]*websocket.Conn // A map of user devices, indexed by device name.

	sync.RWMutex // Read-Write Mutex for concurrent access protection.
}

// Message represents a WebSocket message.
type Message struct {
	At        int64           // Timestamp of the message.
	Device    string          // Device identifier.
	Sender    string          // Sender's user ID.
	MBytes    json.RawMessage // Message content as json.
	Receivers []string        // User IDs of message receivers.
}

// Initialization of a global WebSocket manager.
var (
	wsMgr = &WebsocketMgr{
		log:        slog.Default(),
		upgrader:   websocket.Upgrader{},
		users:      make(map[string]*User),
		messageCh:  make(chan *Message),
		messageBuf: sync.Pool{New: func() interface{} { return &Message{} }},
	}
)

// Initialization function, starting the messageDispatcher goroutine.
func init() {
	go wsMgr.messageDispatcher()
}

// HandleWebSocket upgrades an HTTP connection to a WebSocket connection.
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsMgr.upgrader.Upgrade(w, r, nil)
	if err != nil {
		wsMgr.log.Error("websocket upgrade", slog.String("err", err.Error()))
		return
	}
	defer conn.Close()

	values := r.URL.Query()

	// Join the user and device to the WebSocket manager.
	wsMgr.userDeviceJoin(values.Get("user"), values.Get("device"), conn)
}

// deviceJoin adds a device to a user's WebSocket connections.
func (user *User) deviceJoin(device string, conn *websocket.Conn) {
	user.RLock()
	defer user.RUnlock()

	if old, ok := user.devices[device]; ok {
		old.Close()
	}
	user.devices[device] = conn

	user.handleConnection(conn, device)
}

// deviceLeft removes a device from a user's WebSocket connections.
func (user *User) deviceLeft(device string) {
	user.RLock()
	defer user.RUnlock()

	if conn, ok := user.devices[device]; ok {
		conn.Close()

		delete(user.devices, device)
	}

	if len(user.devices) == 0 {
		wsMgr.userLeft(user.ID)
	}
}

// handleConnection handles WebSocket communication for a user's device.
func (user *User) handleConnection(conn *websocket.Conn, device string) {
	defer func() {
		user.deviceLeft(device)
	}()

	for {
		message := wsMgr.getMessageFromPool()

		if err := conn.ReadJSON(message); err != nil {
			wsMgr.log.Error("read data from websocket", slog.String("err", err.Error()))

			break
		}

		message.Device = device
		message.Sender = user.ID

		wsMgr.messageCh <- message
	}
}

// userDeviceJoin joins a user and their device to the WebSocket manager.
func (mgr *WebsocketMgr) userDeviceJoin(userID, device string, conn *websocket.Conn) {
	user, exists := wsMgr.users[userID]
	if !exists {
		user = &User{ID: userID, devices: make(map[string]*websocket.Conn)}
		mgr.RLock()
		wsMgr.users[userID] = user
		mgr.RUnlock()
	}

	user.deviceJoin(device, conn)
}

// userLeft removes a user from the WebSocket manager.
func (mgr *WebsocketMgr) userLeft(userID string) {
	mgr.RLock()
	delete(wsMgr.users, userID)
	mgr.RUnlock()
}

// getMessageFromPool retrieves a reusable Message object from the message buffer.
func (mgr *WebsocketMgr) getMessageFromPool() *Message {
	return mgr.messageBuf.Get().(*Message)
}

// putMessageToPool returns a Message object to the message buffer for reuse.
func (mgr *WebsocketMgr) putMessageToPool(message *Message) {
	message.At = 0
	message.Sender = ""
	message.MBytes = nil
	message.Receivers = nil
	mgr.messageBuf.Put(message)
}

// messageDispatcher distributes messages to connected users.
func (mgr *WebsocketMgr) messageDispatcher() {
	for {
		select {
		case message := <-mgr.messageCh:
			for _, user := range wsMgr.users {
				for device, conn := range user.devices {
					if message.Sender == user.ID && message.Device == device {
						continue
					}
					if err := conn.WriteJSON(message); err != nil {
						wsMgr.log.Error("dropped message", slog.String("err", err.Error()), slog.Any("data", message))
					}
				}
			}
		case <-wsMgr.done:
			return
		}
	}
}

// Quit gracefully shuts down the WebSocket manager.
func (mgr *WebsocketMgr) Quit() {
	close(mgr.done)
}
