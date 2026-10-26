package chat

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/ws/wslog"
	"f_ccg_video_back_ddd/internal/interfaces/ws/wsorigin"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/authsession"
	"f_ccg_video_back_ddd/pkg/logctx"
	"f_ccg_video_back_ddd/pkg/res"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	chatapp "f_ccg_video_back_ddd/internal/application/chat"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	chatWSWriteWait  = 10 * time.Second
	chatWSPongWait   = 60 * time.Second
	chatWSPingPeriod = 20 * time.Second
	// chatWSReadLimit 单帧读取上限。
	chatWSReadLimit = 64 * 1024
	// chatWSSendBuffer 单连接发送缓冲区长度。
	chatWSSendBuffer = 64
	// sendTimeout 客户端 send 事件的处理超时。
	sendTimeout = 5 * time.Second
	// readTimeout 客户端 read 事件的处理超时。
	readTimeout = 3 * time.Second
)

// 客户端事件类型（入站）。
const (
	clientEventPing = "ping"
	clientEventSend = "send"
	clientEventRead = "read"
)

// 服务端事件类型（出站，属于外部契约）。
const (
	serverEventSystem  = "system"
	serverEventAck     = "ack"
	serverEventMessage = "message"
	serverEventReadAck = "read_ack"
)

// clientEvent 客户端上行事件（字段与原 chat_service.chatClientEvent 逐字一致）.
type clientEvent struct {
	Type        string             `json:"type"`
	To          uint               `json:"to,omitempty"`
	MsgType     string             `json:"msgType,omitempty"`
	Text        string             `json:"text,omitempty"`
	Emoji       string             `json:"emoji,omitempty"`
	Media       *chatapp.MediaItem `json:"media,omitempty"`
	ClientMsgID string             `json:"clientMsgId,omitempty"`
	PeerID      uint               `json:"peerId,omitempty"`
}

// WSEvent 服务端下行事件（字段与原 chat_service.ChatWSEvent 逐字一致）.
type WSEvent struct {
	Type        string               `json:"type"`
	Message     *chatapp.MessageItem `json:"message,omitempty"`
	ClientMsgID string               `json:"clientMsgId,omitempty"`
	PeerID      uint                 `json:"peerId,omitempty"`
	System      string               `json:"system,omitempty"`
}

// envelope 跨实例广播信封（serverId 用于过滤自身回环）.
type envelope struct {
	ServerID string  `json:"serverId"`
	Event    WSEvent `json:"event"`
}

// Hub 私信 WebSocket 房间中心。
//
// 原实现是包级单例 globalChatHub + 包级 upgrader；新架构改为可注入结构体，
// 由 bootstrap 创建唯一实例并注入 HTTP 处理器与 App 生命周期。
type Hub struct {
	mu       sync.RWMutex
	rooms    map[uint]*room
	serverID string
	service  *chatapp.Service
	bus      EventBus
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

// NewHub 构造私信 WebSocket 房间中心。
func NewHub(service *chatapp.Service, bus EventBus, origin *wsorigin.Checker, logger *zap.Logger) *Hub {
	return &Hub{
		rooms:    make(map[uint]*room),
		serverID: uuid.NewString(),
		service:  service,
		bus:      bus,
		logger:   loggerOrNop(logger),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return origin.Check(r) },
		},
	}
}

// Start 启动 hub。房间与 Redis 订阅器按首个连接懒创建，因此无需预热，始终返回 nil。
func (h *Hub) Start(_ context.Context) error {
	return nil
}

// Stop 关闭全部房间：停订阅、断开在连连接。
//
// 原实现没有 hub 级停止入口（进程退出时订阅器随进程销毁）；
// 这里补齐以便 App.Stop 显式回收 Redis 订阅与 WebSocket 连接。
func (h *Hub) Stop(_ context.Context) error {
	h.mu.Lock()
	rooms := make([]*room, 0, len(h.rooms))

	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}

	h.rooms = make(map[uint]*room)
	h.mu.Unlock()

	for _, r := range rooms {
		r.stopSubscriber()
		r.closeClients()
	}

	return nil
}

// ServeWS 升级连接并进入读写循环，直到连接结束。
func (h *Hub) ServeWS(c *gin.Context, userID uint) error {
	c.Status(http.StatusSwitchingProtocols)

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, http.Header{"X-Request-ID": []string{c.Writer.Header().Get("X-Request-ID")}})
	if err != nil {
		res.RecordFailure(wslog.UpgradeFailure(err, c.Writer.Status()), c)
		return err
	}

	connectionCtx := wslog.Connection(c.Request.Context(), strconv.FormatUint(uint64(userID), 10))
	res.SetSuccess(c, true)

	logctx.With(connectionCtx, h.log()).Info("websocket.connection.opened", zap.String("event", "websocket.connection.opened"))
	defer logctx.With(connectionCtx, h.log()).Info("websocket.connection.closed", zap.String("event", "websocket.connection.closed"))

	client := &client{
		id:     fmt.Sprintf("%s:%s", h.serverID, uuid.NewString()),
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, chatWSSendBuffer),
		ctx:    connectionCtx,
	}

	room := h.getRoom(userID)
	client.room = room

	room.addClient(client)

	go client.writePump()

	client.readPump()

	room.removeClient(client)

	return nil
}

func (h *Hub) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

func (h *Hub) getRoom(userID uint) *room {
	h.mu.RLock()
	r := h.rooms[userID]
	h.mu.RUnlock()

	if r != nil {
		return r
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.rooms[userID]; ok {
		return existing
	}

	r = &room{
		userID:  userID,
		hub:     h,
		clients: make(map[*client]struct{}),
	}
	h.rooms[userID] = r

	return r
}

// tryDeleteRoom 在 hub 锁下二次确认房间仍为空且为 map 中同一实例后再删除并停订阅。
func (h *Hub) tryDeleteRoom(r *room) {
	if r == nil {
		return
	}

	shouldStop := false

	h.mu.Lock()

	if cur, ok := h.rooms[r.userID]; ok && cur == r {
		r.mu.Lock()

		if len(r.clients) == 0 {
			delete(h.rooms, r.userID)

			shouldStop = true
		}

		r.mu.Unlock()
	}

	h.mu.Unlock()

	if shouldStop {
		r.stopSubscriber()
	}
}

// deliverToUser 仅向本实例内该用户的连接派发事件。
func (h *Hub) deliverToUser(userID uint, event WSEvent) {
	h.mu.RLock()
	r := h.rooms[userID]
	h.mu.RUnlock()

	if r == nil {
		return
	}

	r.dispatch(event)
}

// publishToUser 向其它实例广播事件（带 serverId 以便对端过滤自身消息）。
func (h *Hub) publishToUser(ctx context.Context, userID uint, event WSEvent) {
	if h.bus == nil || !h.bus.Available() || userID == 0 {
		return
	}

	payload, err := json.Marshal(envelope{ServerID: h.serverID, Event: event})
	if err != nil {
		return
	}

	h.bus.Publish(ctx, userID, payload)
}

// room 单个用户的连接集合与跨实例订阅。
type room struct {
	userID  uint
	hub     *Hub
	clients map[*client]struct{}
	mu      sync.RWMutex
	subOnce sync.Once
	stopSub func()
	stopMu  sync.Mutex
}

func (r *room) addClient(c *client) {
	r.mu.Lock()
	r.clients[c] = struct{}{}
	r.mu.Unlock()
	r.startSubscriber()
}

func (r *room) removeClient(c *client) {
	r.mu.Lock()
	delete(r.clients, c)
	left := len(r.clients)
	r.mu.Unlock()
	close(c.send)
	_ = c.conn.Close()

	if left == 0 {
		// 二次确认：避免 left==0 解锁后与新加入竞态，误删仍有连接的房间。
		r.hub.tryDeleteRoom(r)
	}
}

// closeClients 断开房间内全部连接；读循环随之退出并自行摘除。
func (r *room) closeClients() {
	r.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(r.clients))

	for c := range r.clients {
		conns = append(conns, c.conn)
	}

	r.mu.RUnlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

// dispatch 向房间内所有连接派发事件；缓冲区满则关闭该连接。
//
// 写入每连接的有界 send 队列，避免慢客户端阻塞整个房间；真正网络写出集中在 writePump。
// 队列已接受不等于对端已收到，重连后的历史与未读同步仍需查询持久化数据。
func (r *room) dispatch(event WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	r.mu.RLock()

	for c := range r.clients {
		select {
		case c.send <- data:
		default:
			go c.closeWithReason("send buffer full")
		}
	}

	r.mu.RUnlock()
}

// startSubscriber 懒启动跨实例订阅（每个房间仅一次）。
func (r *room) startSubscriber() {
	r.subOnce.Do(func() {
		if r.hub == nil || r.hub.bus == nil || !r.hub.bus.Available() {
			return
		}

		stop := r.hub.bus.Subscribe(r.userID, func(payload []byte) {
			var env envelope
			if err := json.Unmarshal(payload, &env); err != nil {
				return
			}

			if env.ServerID == r.hub.serverID {
				return
			}

			r.dispatch(env.Event)
		})

		r.stopMu.Lock()
		r.stopSub = stop
		r.stopMu.Unlock()
	})
}

// stopSubscriber 停止跨实例订阅（幂等）。
func (r *room) stopSubscriber() {
	r.stopMu.Lock()
	stop := r.stopSub
	r.stopSub = nil
	r.stopMu.Unlock()

	if stop != nil {
		stop()
	}
}

// client 单条 WebSocket 连接。
type client struct {
	id     string
	userID uint
	conn   *websocket.Conn
	room   *room
	send   chan []byte
	ctx    context.Context //nolint:containedctx // Owns a WebSocket connection lifetime beyond the HTTP upgrade request.
}

func (c *client) readPump() {
	defer c.closeWithReason("read loop exit")

	c.conn.SetReadLimit(chatWSReadLimit)
	_ = c.conn.SetReadDeadline(time.Now().Add(chatWSPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(chatWSPongWait))

		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var evt clientEvent
		if err := json.Unmarshal(message, &evt); err != nil {
			wslog.Run(c.ctx, c.room.hub.log(), "chat.decode", func(context.Context) error { return apperr.Wrap("websocket.decode", nil, err, apperr.Validation) })
			continue
		}

		switch evt.Type {
		case clientEventPing:
			// handled by ws ping frame
		case clientEventSend:
			wslog.Run(c.ctx, c.room.hub.log(), "chat.send", func(ctx context.Context) error { return c.handleSend(ctx, evt) })
		case clientEventRead:
			wslog.Run(c.ctx, c.room.hub.log(), "chat.read", func(ctx context.Context) error { return c.handleRead(ctx, evt) })
		default:
			// ignore
		}
	}
}

// writePump 串行负责本连接的业务帧与周期 ping，配合写截止时间淘汰不再读取的客户端。
// 其他 goroutine 通过 send 队列交付数据，避免并发调用连接的常规写方法。
func (c *client) writePump() {
	pingTicker := time.NewTicker(chatWSPingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			if ok && c.userID != 0 {
				authCtx, cancel := context.WithTimeout(c.ctx, 3*time.Second)
				err := authsession.Validate(authCtx)

				cancel()

				if err != nil {
					wslog.Result(c.ctx, c.room.hub.log(), "chat.session.validate", err, "", 0)
					c.closeWithReason("session revoked")

					return
				}
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(chatWSWriteWait))

			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})

				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			if c.userID != 0 {
				authCtx, cancel := context.WithTimeout(c.ctx, 3*time.Second)
				err := authsession.Validate(authCtx)

				cancel()

				if err != nil {
					wslog.Result(c.ctx, c.room.hub.log(), "chat.session.validate", err, "", 0)
					c.closeWithReason("session revoked")

					return
				}
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(chatWSWriteWait))

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *client) closeWithReason(_ string) {
	_ = c.conn.Close()
}

func (c *client) sendSystem(msg string) {
	payload, err := json.Marshal(WSEvent{Type: serverEventSystem, System: msg})
	if err != nil {
		return
	}

	select {
	case c.send <- payload:
	default:
	}
}

func (c *client) handleSend(parent context.Context, evt clientEvent) error {
	if c.userID == 0 {
		c.sendSystem("请先登录")
		return apperr.Wrap("chat.send", nil, errors.New("authentication required"), apperr.Rejected)
	}

	if evt.To == 0 {
		c.sendSystem("to 不能为空")
		return apperr.Wrap("chat.send", nil, errors.New("recipient required"), apperr.Validation)
	}

	ctx, cancel := context.WithTimeout(parent, sendTimeout)
	defer cancel()

	if err := authsession.Validate(ctx); err != nil {
		c.sendSystem("session expired, please sign in again")
		return err
	}

	msg, err := c.room.hub.service.SendMessage(ctx, c.userID, evt.To, evt.MsgType, evt.Text, evt.Emoji, evt.Media)
	if err != nil {
		c.sendSystem(clienterror.Message(err))
		return err
	}

	ack := WSEvent{Type: serverEventAck, ClientMsgID: evt.ClientMsgID, Message: msg}
	c.room.dispatch(ack)

	messageEvt := WSEvent{Type: serverEventMessage, Message: msg}
	c.room.hub.deliverToUser(evt.To, messageEvt)
	c.room.hub.publishToUser(ctx, evt.To, messageEvt)

	return nil
}

func (c *client) handleRead(parent context.Context, evt clientEvent) error {
	if c.userID == 0 {
		return apperr.Wrap("chat.read", nil, errors.New("authentication required"), apperr.Rejected)
	}

	if evt.PeerID == 0 {
		return apperr.Wrap("chat.read", nil, errors.New("peer required"), apperr.Validation)
	}

	ctx, cancel := context.WithTimeout(parent, readTimeout)
	defer cancel()

	if err := authsession.Validate(ctx); err != nil {
		c.sendSystem("session expired, please sign in again")
		return err
	}

	if err := c.room.hub.service.MarkRead(ctx, c.userID, evt.PeerID); err != nil {
		return err
	}

	c.room.dispatch(WSEvent{Type: serverEventReadAck, PeerID: evt.PeerID})

	return nil
}
