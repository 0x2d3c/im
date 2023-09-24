package v1

import (
	"encoding/json"
	"hash/fnv"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"

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
	users     *atomic.Value // Atomic value to store user connections.
	messageCh chan *Message // Channel for handling messages within this user pool.
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
		v := &atomic.Value{}
		v.Store(make(map[string]*websocket.Conn))
		wsMgr.users[i] = &UserPool{users: v, messageCh: make(chan *Message)}

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

func (pool *UserPool) userDeviceJoin(userID, device string, conn *websocket.Conn) {
	users := pool.users.Load().(map[string]*websocket.Conn)

	userCopies := make(map[string]*websocket.Conn)
	for k, v := range users {
		if k == userID+device {
			v.Close()
			continue
		}
		userCopies[k] = v
	}

	userCopies[userID+device] = conn

	pool.users.Store(userCopies)

	pool.handleConnection(userID, conn, device)
}

func (pool *UserPool) handleConnection(userID string, conn *websocket.Conn, device string) {
	defer func() {
		pool.userDeviceLeft(userID, device)
	}()

	for {
		message := wsMgr.getMessageFromPool()

		if err := conn.ReadJSON(message); err != nil {
			wsMgr.log.Error("read data from websocket", slog.String("err", err.Error()))
			break
		}

		message.Device = device
		message.Sender = userID

		for i := range message.Receivers {
			wsMgr.users[hashIndex(message.Receivers[i])].messageCh <- message
		}

		wsMgr.users[hashIndex(userID)].messageCh <- message
	}
}

func (pool *UserPool) userDeviceLeft(userID, device string) {
	users := pool.users.Load().(map[string]*websocket.Conn)

	userCopies := make(map[string]*websocket.Conn)

	for k, v := range users {
		if k == userID+device {
			v.Close()
			continue
		}
		userCopies[k] = v
	}

	pool.users.Store(userCopies)
}

func (mgr *WebsocketMgr) getMessageFromPool() *Message {
	return mgr.messageBuf.Get().(*Message)
}

func (mgr *WebsocketMgr) messageDispatcher(pool *UserPool) {
	for {
		select {
		case message := <-pool.messageCh:
			users := pool.users.Load().(map[string]*websocket.Conn)
			for k, conn := range users {
				if k == message.Sender+message.Device {
					continue
				}
				if err := conn.WriteJSON(message); err != nil {
					wsMgr.log.Error("dropped message", slog.String("err", err.Error()), slog.Any("data", message))
				}
			}
		case <-mgr.done:
			return
		}
	}
}
