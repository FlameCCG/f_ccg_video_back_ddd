package danmu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/ws/wsorigin"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 20 * time.Second
	// wsReadLimit 单帧读取上限（与原 SetReadLimit(2048) 一致）。
	wsReadLimit = 2048
	// wsSendBuffer 单连接发送缓冲区长度（与原 make(chan []byte, 32) 一致）。
	wsSendBuffer = 32
	// sendDanmuTimeout 客户端 danmu/send 事件的处理超时。
	sendDanmuTimeout = 3 * time.Second
)

// 客户端事件类型（入站）。
const (
	clientEventPing  = "ping"
	clientEventDanmu = "danmu"
	clientEventSend  = "send"
)

// 服务端事件类型（出站，属于外部契约）。
const (
	serverEventDanmu  = "danmu"
	serverEventOnline = "online"
	serverEventSystem = "system"
)

// loginRequiredMessage 未登录用户尝试发弹幕时的系统提示（文案属外部契约）。
const loginRequiredMessage = "请先登录后再发送弹幕"

// Event 弹幕房间下行事件（字段与原 video_service.DanmuEvent 逐字一致）。
type Event struct {
	Type        string             `json:"type"`
	VideoID     uint               `json:"videoId"`
	VideoPartID uint               `json:"videoPartId"`
	Online      int64              `json:"online,omitempty"`
	Danmu       *videoapp.DanmuDTO `json:"danmu,omitempty"`
	System      string             `json:"system,omitempty"`
}

// envelope 跨实例广播信封（serverId 用于过滤自身回环）。
type envelope struct {
	Event
	ServerID string `json:"serverId"`
}

// clientMessage 客户端上行消息（字段与原 clientMessage 逐字一致）。
type clientMessage struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	TimeOffset int    `json:"timeOffset"`
	Color      string `json:"color"`
	FontSize   int8   `json:"fontSize"`
	Position   int8   `json:"position"`
}

// roomKey 房间标识（视频 + 分P）。
type roomKey struct {
	videoID uint
	partID  uint
}

// Hub 弹幕 WebSocket 房间中心。
//
// 原实现是包级单例 globalDanmuHub + 包级 upgrader；新架构改为可注入结构体，
// 由 bootstrap 创建唯一实例并注入 HTTP 处理器与 App 生命周期。
type Hub struct {
	mu       sync.RWMutex
	rooms    map[roomKey]*room
	serverID string
	service  *videoapp.DanmuService
	bus      EventBus
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

// 编译期断言：Hub 满足应用层的弹幕广播端口。
var _ videoapp.DanmuBroadcaster = (*Hub)(nil)

// NewHub 构造弹幕 WebSocket 房间中心。
func NewHub(service *videoapp.DanmuService, bus EventBus, origin *wsorigin.Checker, logger *zap.Logger) *Hub {
	return &Hub{
		rooms:    make(map[roomKey]*room),
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

	h.rooms = make(map[roomKey]*room)
	h.mu.Unlock()

	for _, r := range rooms {
		r.stopSubscriber()
		r.closeClients()
	}

	return nil
}

// ServeWS 升级连接并进入读写循环，直到连接结束。
func (h *Hub) ServeWS(c *gin.Context, videoID, partID, userID uint) error {
	room := h.getRoom(videoID, partID)

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log().Warn("danmu websocket upgrade failed", zap.Error(err), zap.Uint("videoId", videoID))

		return err
	}

	clientCtx := c.Request.Context()

	cl := &client{
		id:      fmt.Sprintf("%s:%s", h.serverID, uuid.NewString()),
		userID:  userID,
		videoID: videoID,
		partID:  partID,
		conn:    conn,
		room:    room,
		send:    make(chan []byte, wsSendBuffer),
		ctx:     clientCtx,
	}

	room.addClient(cl)
	online := room.addOnline(clientCtx, cl)
	h.broadcastOnline(clientCtx, videoID, partID, online)

	go cl.writePump()

	cl.readPump()

	roomRemaining, userRemaining := room.removeClient(cl)
	if userRemaining == 0 {
		online = room.removeOnline(clientCtx, cl)
	} else {
		online = room.countOnline(clientCtx)
	}

	h.broadcastOnline(clientCtx, videoID, partID, online)

	if roomRemaining == 0 {
		h.deleteRoom(room)
	}

	return nil
}

// BroadcastDanmu 广播一条新弹幕。
//
// 仅 lookup 已有房间，不经 getRoom 创建空房（避免纯广播导致 rooms 无界增长）；
// 跨实例仍需 Publish。
func (h *Hub) BroadcastDanmu(ctx context.Context, dto *videoapp.DanmuDTO) {
	if dto == nil {
		return
	}

	baseCtx := ctx
	if baseCtx == nil {
		baseCtx = context.TODO()
	}

	event := Event{
		Type:        serverEventDanmu,
		VideoID:     dto.VideoID,
		VideoPartID: dto.VideoPartID,
		Danmu:       dto,
	}

	if r := h.lookupRoom(dto.VideoID, dto.VideoPartID); r != nil {
		r.dispatch(event)
	}

	h.publish(baseCtx, dto.VideoID, dto.VideoPartID, event)
}

// broadcastOnline 广播房间在线人数变化。
func (h *Hub) broadcastOnline(ctx context.Context, videoID, partID uint, online int64) {
	if videoID == 0 || online < 0 {
		return
	}

	baseCtx := ctx
	if baseCtx == nil {
		baseCtx = context.TODO()
	}

	event := Event{
		Type:        serverEventOnline,
		VideoID:     videoID,
		VideoPartID: partID,
		Online:      online,
	}

	if r := h.lookupRoom(videoID, partID); r != nil {
		r.dispatch(event)
	}

	h.publish(baseCtx, videoID, partID, event)
}

// publish 向其它实例广播事件（带 serverId 以便对端过滤自身消息）。
func (h *Hub) publish(ctx context.Context, videoID, partID uint, event Event) {
	if h.bus == nil || !h.bus.Available() {
		return
	}

	payload, err := json.Marshal(envelope{Event: event, ServerID: h.serverID})
	if err != nil {
		h.log().Warn("marshal danmu envelope failed", zap.Error(err), zap.Uint("videoId", videoID))

		return
	}

	h.bus.Publish(ctx, videoID, partID, payload)
}

// getRoom 取房间，不存在则创建。
func (h *Hub) getRoom(videoID, partID uint) *room {
	key := roomKey{videoID: videoID, partID: partID}

	h.mu.RLock()
	r := h.rooms[key]
	h.mu.RUnlock()

	if r != nil {
		return r
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.rooms[key]; ok {
		return existing
	}

	r = &room{
		videoID:    videoID,
		partID:     partID,
		hub:        h,
		clients:    make(map[*client]struct{}),
		userCounts: make(map[uint]int),
	}
	h.rooms[key] = r

	return r
}

// lookupRoom 只读查找，不存在则返回 nil（不创建）。
func (h *Hub) lookupRoom(videoID, partID uint) *room {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.rooms[roomKey{videoID: videoID, partID: partID}]
}

// deleteRoom 二次确认房间仍为空且为 map 中同一实例后再删除并停订阅。
func (h *Hub) deleteRoom(r *room) {
	if r == nil {
		return
	}

	key := roomKey{videoID: r.videoID, partID: r.partID}
	shouldStop := false

	h.mu.Lock()

	if cur, ok := h.rooms[key]; ok && cur == r {
		r.mu.Lock()

		if len(r.clients) == 0 {
			delete(h.rooms, key)
			shouldStop = true
		}

		r.mu.Unlock()
	}

	h.mu.Unlock()

	if shouldStop {
		r.stopSubscriber()
	}
}

// room 单个弹幕房间的连接集合与跨实例订阅。
type room struct {
	videoID    uint
	partID     uint
	hub        *Hub
	clients    map[*client]struct{}
	userCounts map[uint]int
	mu         sync.RWMutex
	subOnce    sync.Once
	stopSub    func()
	stopMu     sync.Mutex
}

// addClient 加入连接并懒启动跨实例订阅。
func (r *room) addClient(c *client) {
	r.mu.Lock()
	r.clients[c] = struct{}{}

	if c.userID != 0 {
		r.userCounts[c.userID]++
	}

	r.mu.Unlock()
	r.startSubscriber()
}

// removeClient 摘除连接，返回房间剩余连接数与该用户剩余连接数。
func (r *room) removeClient(c *client) (roomRemaining, userRemaining int) {
	r.mu.Lock()
	delete(r.clients, c)

	if c.userID != 0 {
		if current, ok := r.userCounts[c.userID]; ok {
			current--

			if current <= 0 {
				delete(r.userCounts, c.userID)
			} else {
				r.userCounts[c.userID] = current
				userRemaining = current
			}
		}
	}

	roomRemaining = len(r.clients)
	r.mu.Unlock()
	close(c.send)
	_ = c.conn.Close()

	return roomRemaining, userRemaining
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
func (r *room) dispatch(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		r.log().Warn("marshal danmu event failed", zap.Error(err), zap.Uint("videoId", r.videoID))

		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 纯匿名房间不广播 online 变化，减少无效推送
	if event.Type == serverEventOnline && len(r.userCounts) == 0 {
		return
	}

	for c := range r.clients {
		select {
		case c.send <- data:
		default:
			go c.closeWithReason("send buffer full")
		}
	}
}

// startSubscriber 懒启动跨实例订阅（每个房间仅一次）。
func (r *room) startSubscriber() {
	r.subOnce.Do(func() {
		if r.hub == nil || r.hub.bus == nil || !r.hub.bus.Available() {
			return
		}

		stop := r.hub.bus.Subscribe(r.videoID, r.partID, func(payload []byte) {
			var env envelope
			if err := json.Unmarshal(payload, &env); err != nil {
				r.log().Warn("unmarshal danmu envelope failed", zap.Error(err), zap.Uint("videoId", r.videoID))

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

func (h *Hub) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}

func (r *room) log() *zap.Logger {
	if r == nil || r.hub == nil {
		return zap.NewNop()
	}

	return r.hub.log()
}

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
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

// addOnline 把连接计入在线集合并返回在线人数（Redis 不可用时回落本地统计）。
func (r *room) addOnline(ctx context.Context, c *client) int64 {
	if c == nil || c.userID == 0 || r.hub == nil || r.hub.bus == nil {
		return r.localOnline()
	}

	if online, ok := r.hub.bus.AddOnline(ctx, r.videoID, r.partID, c.userID); ok {
		return online
	}

	return r.localOnline()
}

// removeOnline 把连接移出在线集合并返回在线人数。
func (r *room) removeOnline(ctx context.Context, c *client) int64 {
	if c == nil || c.userID == 0 || r.hub == nil || r.hub.bus == nil {
		return r.localOnline()
	}

	if online, ok := r.hub.bus.RemoveOnline(ctx, r.videoID, r.partID, c.userID); ok {
		return online
	}

	return r.localOnline()
}

// countOnline 读取在线人数。
func (r *room) countOnline(ctx context.Context) int64 {
	if r.hub == nil || r.hub.bus == nil {
		return r.localOnline()
	}

	if online, ok := r.hub.bus.CountOnline(ctx, r.videoID, r.partID); ok {
		return online
	}

	return r.localOnline()
}

// localOnline 本实例内的去重在线用户数（游客不计入）。
//
// 按用户而非连接去重，同一用户多个标签页不会在本地统计中重复计人。
// 这是 Redis 不可用时的实例内回退，不能代表多实例部署的全站在线人数。
func (r *room) localOnline() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[uint]struct{})

	for c := range r.clients {
		if c.userID == 0 {
			continue
		}

		seen[c.userID] = struct{}{}
	}

	return int64(len(seen))
}

// client 单条弹幕 WebSocket 连接。
type client struct {
	id      string
	userID  uint
	videoID uint
	partID  uint
	conn    *websocket.Conn
	room    *room
	send    chan []byte
	ctx     context.Context
}

// readPump 读循环：处理 ping / danmu / send 三类上行事件。
func (c *client) readPump() {
	defer c.closeWithReason("read loop exit")

	c.conn.SetReadLimit(wsReadLimit)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))

		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case clientEventPing:
			// reply handled in writePump via ping ticker
		case clientEventDanmu, clientEventSend:
			c.handleSend(msg)
		default:
			// ignore unknown
		}
	}
}

// handleSend 处理客户端发送弹幕事件。
func (c *client) handleSend(msg clientMessage) {
	if c.userID == 0 {
		c.sendSystem(loginRequiredMessage)

		return
	}

	if c.room == nil || c.room.hub == nil || c.room.hub.service == nil {
		return
	}

	ctx, cancel := context.WithTimeout(c.ctx, sendDanmuTimeout)
	defer cancel()

	_, err := c.room.hub.service.Send(ctx, c.userID, videoapp.SendDanmuParams{
		VideoID:     c.videoID,
		VideoPartID: c.partID,
		Content:     msg.Content,
		TimeOffset:  msg.TimeOffset,
		Color:       msg.Color,
		FontSize:    msg.FontSize,
		Position:    msg.Position,
	})
	if err != nil {
		c.sendSystem(err.Error())
	}
}

// writePump 写循环：转发下行事件、定时 ping 并续期在线集合。
func (c *client) writePump() {
	pingTicker := time.NewTicker(wsPingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))

			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})

				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

			// 续期在线集合 TTL（与原 refreshOnline 一致，复用 addOnline 语义）。
			c.room.addOnline(c.ctx, c)
		case <-c.ctx.Done():
			return
		}
	}
}

// closeWithReason 关闭连接（reason 仅用于可读性，与原实现一致不落日志）。
func (c *client) closeWithReason(_ string) {
	_ = c.conn.Close()
}

// sendSystem 向该连接下发系统提示。
func (c *client) sendSystem(msg string) {
	payload, err := json.Marshal(Event{
		Type:        serverEventSystem,
		VideoID:     c.videoID,
		VideoPartID: c.partID,
		System:      msg,
	})
	if err != nil {
		return
	}

	select {
	case c.send <- payload:
	default:
	}
}
