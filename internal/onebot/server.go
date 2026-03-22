package onebot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"milky-onebot11-bridge/internal/config"
)

type Handler interface {
	HandleAPI(context.Context, APIRequest) APIResponse
	OnWSConnect(context.Context, string) []any
	CurrentSelfID() int64
}

type Server struct {
	cfg      config.OneBotConfig
	logger   *slog.Logger
	handler  Handler
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[*clientConn]struct{}
	server  *http.Server
}

type clientConn struct {
	conn       *websocket.Conn
	canSend    bool
	canReceive bool
	name       string
	mu         sync.Mutex
}

func NewServer(cfg config.OneBotConfig, logger *slog.Logger, handler Handler) *Server {
	return &Server{
		cfg:     cfg,
		logger:  logger,
		handler: handler,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		clients: make(map[*clientConn]struct{}),
	}
}

func (s *Server) Start(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleUniversalWS)
	mux.HandleFunc("/api", s.handleAPIWS)
	mux.HandleFunc("/event", s.handleEventWS)
	mux.HandleFunc("/api/", s.handleAPIWS)
	mux.HandleFunc("/event/", s.handleEventWS)
	mux.HandleFunc("/http/", s.handleHTTPAPI)
	mux.HandleFunc("/http", s.handleHTTPAPI)

	s.server = &http.Server{
		Addr:    net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port)),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("shutdown onebot server failed", "err", err)
		}
	}()

	go func() {
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		close(errCh)
	}()

	s.startReverseClients(ctx)

	return errCh
}

func (s *Server) Broadcast(event any) {
	payload, err := json.Marshal(event)
	if err != nil {
		s.logger.Error("marshal onebot event failed", "err", err)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for client := range s.clients {
		if !client.canSend {
			continue
		}
		if err := client.write(websocket.TextMessage, payload); err != nil {
			s.logger.Warn("broadcast to client failed", "err", err, "name", client.name)
		}
	}
}

func (s *Server) handleUniversalWS(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.EnableWSUniversal {
		http.Error(w, "websocket universal mode disabled", http.StatusNotFound)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("upgrade websocket failed", "err", err)
		return
	}

	client := &clientConn{conn: conn, canSend: true, canReceive: true, name: "forward-universal"}
	s.addClient(client)
	defer s.removeClient(client)
	defer conn.Close()

	for _, event := range s.handler.OnWSConnect(r.Context(), "universal") {
		if event == nil {
			continue
		}
		payload, err := json.Marshal(event)
		if err != nil {
			s.logger.Error("marshal lifecycle event failed", "err", err)
			continue
		}
		if err := client.write(websocket.TextMessage, payload); err != nil {
			s.logger.Warn("send lifecycle event failed", "err", err)
		}
	}

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req APIRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			resp := Failure(1400, "invalid json request", nil)
			_ = client.writeJSON(resp)
			continue
		}

		resp := s.handler.HandleAPI(r.Context(), req)
		if err := client.writeJSON(resp); err != nil {
			s.logger.Warn("write api response failed", "err", err)
			return
		}
	}
}

func (s *Server) handleAPIWS(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.EnableWSAPI {
		http.Error(w, "websocket api mode disabled", http.StatusNotFound)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("upgrade websocket api failed", "err", err)
		return
	}
	client := &clientConn{conn: conn, canSend: false, canReceive: true, name: "forward-api"}
	s.addClient(client)
	defer s.removeClient(client)
	defer conn.Close()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var req APIRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			_ = client.writeJSON(Failure(1400, "invalid json request", nil))
			continue
		}
		resp := s.handler.HandleAPI(r.Context(), req)
		if err := client.writeJSON(resp); err != nil {
			s.logger.Warn("write api ws response failed", "err", err)
			return
		}
	}
}

func (s *Server) handleEventWS(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.EnableWSEvent {
		http.Error(w, "websocket event mode disabled", http.StatusNotFound)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("upgrade websocket event failed", "err", err)
		return
	}
	client := &clientConn{conn: conn, canSend: true, canReceive: false, name: "forward-event"}
	s.addClient(client)
	defer s.removeClient(client)
	defer conn.Close()

	for _, event := range s.handler.OnWSConnect(r.Context(), "event") {
		if event == nil {
			continue
		}
		payload, err := json.Marshal(event)
		if err != nil {
			s.logger.Error("marshal lifecycle event failed", "err", err)
			continue
		}
		if err := client.write(websocket.TextMessage, payload); err != nil {
			s.logger.Warn("send lifecycle event failed", "err", err)
		}
	}

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) handleHTTPAPI(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.EnableHTTPAPI {
		http.NotFound(w, r)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/http/")
	if action == "" || action == "/http" {
		http.Error(w, "action is required", http.StatusBadRequest)
		return
	}
	req := APIRequest{Action: action}
	params, err := buildHTTPParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(params) > 0 {
		req.Params = params
	}
	resp := s.handler.HandleAPI(r.Context(), req)
	w.Header().Set("Content-Type", "application/json")
	if resp.RetCode == 1400 {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func buildHTTPParams(r *http.Request) ([]byte, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	params := map[string]any{}
	for key, values := range r.Form {
		if len(values) == 0 {
			continue
		}
		if len(values) == 1 {
			params[key] = normalizeHTTPScalar(key, values[0])
			continue
		}
		items := make([]any, 0, len(values))
		for _, value := range values {
			items = append(items, normalizeHTTPScalar(key, value))
		}
		params[key] = items
	}

	if r.Method == http.MethodPost && r.Body != nil {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				return nil, fmt.Errorf("invalid json body: %w", err)
			}
			for k, v := range payload {
				params[k] = v
			}
		}
	}

	if len(params) == 0 {
		return nil, nil
	}
	return json.Marshal(params)
}

func normalizeHTTPScalar(key, value string) any {
	key = strings.TrimSpace(strings.ToLower(key))
	switch key {
	case "message":
		return value
	case "user_id", "group_id", "message_id", "self_id", "duration", "delay", "times":
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	case "auto_escape", "approve", "enable", "no_cache", "reject_add_request":
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return value
}

func (s *Server) startReverseClients(ctx context.Context) {
	cfg := s.cfg.Reverse
	if !cfg.Enable {
		return
	}
	if cfg.UseUniversalClient {
		target := firstNonEmpty(cfg.URL, cfg.APIURL, cfg.EventURL)
		if target != "" {
			go s.runReverseClient(ctx, target, "Universal", true, true)
		}
		return
	}
	apiURL := firstNonEmpty(cfg.APIURL, cfg.URL)
	eventURL := firstNonEmpty(cfg.EventURL, cfg.URL)
	if apiURL != "" {
		go s.runReverseClient(ctx, apiURL, "API", false, true)
	}
	if eventURL != "" {
		go s.runReverseClient(ctx, eventURL, "Event", true, false)
	}
}

func (s *Server) runReverseClient(ctx context.Context, target, role string, canSend, canReceive bool) {
	reconnect := time.Duration(s.cfg.Reverse.ReconnectIntervalMS) * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := s.dialReverse(ctx, target, role)
		if err != nil {
			s.logger.Warn("reverse websocket connect failed", "target", target, "role", role, "err", err)
			if !sleepWithContext(ctx, reconnect) {
				return
			}
			continue
		}

		client := &clientConn{
			conn:       conn,
			canSend:    canSend,
			canReceive: canReceive,
			name:       "reverse-" + strings.ToLower(role),
		}
		s.addClient(client)
		s.logger.Info("reverse websocket connected", "target", target, "role", role)

		if canSend {
			for _, event := range s.handler.OnWSConnect(ctx, "reverse-"+strings.ToLower(role)) {
				if event == nil {
					continue
				}
				payload, err := json.Marshal(event)
				if err != nil {
					s.logger.Error("marshal reverse lifecycle event failed", "err", err)
					continue
				}
				if err := client.write(websocket.TextMessage, payload); err != nil {
					s.logger.Warn("send reverse lifecycle event failed", "err", err)
				}
			}
		}

		err = s.serveReverseConnection(ctx, client)
		s.removeClient(client)
		_ = conn.Close()
		if err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Warn("reverse websocket disconnected", "target", target, "role", role, "err", err)
		}
		if !sleepWithContext(ctx, reconnect) {
			return
		}
	}
}

func (s *Server) dialReverse(ctx context.Context, target, role string) (*websocket.Conn, error) {
	headers := http.Header{}
	if selfID := s.handler.CurrentSelfID(); selfID != 0 {
		headers.Set("X-Self-ID", strconv.FormatInt(selfID, 10))
	}
	headers.Set("X-Client-Role", role)
	if token := strings.TrimSpace(s.cfg.AccessToken); token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}
	dialer := websocket.Dialer{}
	conn, resp, err := dialer.DialContext(ctx, target, headers)
	if err != nil {
		if resp == nil {
			return nil, err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("websocket handshake failed: status=%d %s body=%q err=%w", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)), err)
	}
	return conn, nil
}

func (s *Server) serveReverseConnection(ctx context.Context, client *clientConn) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, payload, err := client.conn.ReadMessage()
		if err != nil {
			return err
		}
		if !client.canReceive {
			continue
		}

		var req APIRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if err := client.writeJSON(Failure(1400, "invalid json request", nil)); err != nil {
				return err
			}
			continue
		}
		resp := s.handler.HandleAPI(ctx, req)
		if err := client.writeJSON(resp); err != nil {
			return err
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sleepWithContext(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Server) authorized(r *http.Request) bool {
	if strings.TrimSpace(s.cfg.AccessToken) == "" {
		return true
	}
	queryToken := strings.TrimSpace(r.URL.Query().Get("access_token"))
	if queryToken == s.cfg.AccessToken {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return false
	}
	auth = strings.TrimPrefix(auth, "Bearer ")
	auth = strings.TrimPrefix(auth, "Token ")
	return auth == s.cfg.AccessToken
}

func (s *Server) addClient(client *clientConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client] = struct{}{}
}

func (s *Server) removeClient(client *clientConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
}

func (c *clientConn) writeJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.write(websocket.TextMessage, payload)
}

func (c *clientConn) write(messageType int, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(messageType, payload)
}
