package v1

import (
	"encoding/json"
	"hash/fnv"
	"log/slog"
	"net/http"
	"runtime"
	"sync"

	"github.com/gorilla/websocket"
)

// WebsocketMgr manages WebSocket connections and messages.
type WebsocketMgr struct {
	log        *slog.Logger  // Logger for handling logs.
	users      []*UserPool   // Slice of user pools, each corresponding to a specific set of users.
	messageCh  chan *Message // Channel for sending and receiving messages.
	upgrader   websocket.Upgrader
	messageBuf sync.Pool // Pool for managing reusable Message objects.
	done       chan struct{}
}

// UserPool represents a pool of WebSocket users.
type UserPool struct {
	users     sync.Map      // Atomic value to store user connections.
	messageCh chan *Message // Channel for handling messages within this user pool.
}

type User struct {
	devices   sync.Map      // Atomic value to store user device connections.
	messageCh chan *Message // Channel for handling messages within this user.
}

// Message represents a WebSocket message.
type Message struct {
	At        int64           // Timestamp of the message.
	Device    string          // Device identifier.
	Sender    string          // Sender's user ID.
	MBytes    json.RawMessage // Message content as json.
	Receivers []string        // User IDs of message receivers.
}

var (
	numPools = 2 * runtime.NumCPU() // Number of user pools based on CPU cores.
	wsMgr    = &WebsocketMgr{
		log:        slog.Default(),
		messageCh:  make(chan *Message),
		upgrader:   websocket.Upgrader{},
		messageBuf: sync.Pool{New: func() interface{} { return &Message{} }},
		done:       make(chan struct{}),
	}
)

func init() {
	wsMgr.users = make([]*UserPool, numPools)

	// Initialize user pools and start message dispatchers for each pool.
	for i := 0; i < numPools; i++ {
		wsMgr.users[i] = &UserPool{messageCh: make(chan *Message)}

		go wsMgr.messageDispatcher(wsMgr.users[i])
	}
}

// HandleWebSocket handles WebSocket upgrade requests.
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsMgr.upgrader.Upgrade(w, r, nil)
	if err != nil {
		wsMgr.log.Error("websocket upgrade", slog.String("err", err.Error()))
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("user")
	device := r.URL.Query().Get("device")

	userPool := wsMgr.users[hashIndex(userID)]

	userPool.userDeviceJoin(userID, device, conn)
}

// hashIndex calculates the pool index based on the user ID.
func hashIndex(id string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(id))
	return h.Sum32() % uint32(numPools)
}

func (pool *UserPool) userDeviceJoin(userID, deviceType string, conn *websocket.Conn) {
	pool.users.Range(func(uid, user any) bool {
		if uid != userID {
			return true
		}

		user.(*User).devices.Store(deviceType, conn)

		return false
	})

	user, ok := pool.users.Load(userID)

	if !ok {
		u := &User{
			devices:   sync.Map{},
			messageCh: make(chan *Message),
		}

		u.devices.Store(deviceType, conn)

		pool.users.Store(userID, u)
	} else {

		user.(*User).devices.Store(deviceType, conn)

		pool.users.Store(userID, user)
	}

	pool.handleConnection(userID, conn, deviceType)
}

func (pool *UserPool) handleConnection(userID string, conn *websocket.Conn, deviceType string) {
	defer func() {
		pool.userDeviceLeft(userID, deviceType)
	}()

	for {
		message := wsMgr.getMessageFromPool()

		if err := conn.ReadJSON(message); err != nil {
			wsMgr.log.Error("read data from websocket", slog.String("err", err.Error()))
			break
		}

		message.Sender = userID
		message.Device = deviceType

		for i := range message.Receivers {
			wsMgr.users[hashIndex(message.Receivers[i])].messageCh <- message
		}

		wsMgr.users[hashIndex(userID)].messageCh <- message
	}
}

func (pool *UserPool) userDeviceLeft(userID, deviceType string) {
	pool.users.Range(func(uid, user any) bool {
		if uid != userID {
			return true
		}

		user.(*User).devices.Delete(deviceType)

		return false
	})
}

func (mgr *WebsocketMgr) getMessageFromPool() *Message {
	return mgr.messageBuf.Get().(*Message)
}

func (mgr *WebsocketMgr) messageDispatcher(pool *UserPool) {
	for {
		select {
		case message := <-pool.messageCh:
			pool.users.Range(func(uid, user any) bool {
				for i := 0; i < len(message.Receivers); i++ {
					if uid == message.Receivers[i] {
						user.(*User).devices.Range(func(device, conn any) bool {
							if err := conn.(*websocket.Conn).WriteJSON(message); err != nil {
								wsMgr.log.Error("dropped message", slog.String("err", err.Error()), slog.Any("data", message))
							}
							return true
						})
					}
				}
				if uid == message.Sender {
					user.(*User).devices.Range(func(device, conn any) bool {
						if device == message.Device {
							return true
						}
						if err := conn.(*websocket.Conn).WriteJSON(message); err != nil {
							wsMgr.log.Error("dropped message", slog.String("err", err.Error()), slog.Any("data", message))
						}
						return true
					})
				}
				return true
			})
		case <-mgr.done:
			return
		}
	}
}
