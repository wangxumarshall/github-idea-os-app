package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	adminSSHSessionTTL        = 5 * time.Minute
	adminSSHOutputBufferBytes = 256 * 1024
	adminSSHTmuxSessionName   = "multica-admin"
)

var adminSSHUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type adminSSHStreamEvent struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Cols     int    `json:"cols,omitempty"`
	Rows     int    `json:"rows,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

type adminSSHClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type runtimeSSHConfig struct {
	Host    string
	Port    int
	User    string
	Enabled bool
}

type adminSSHSession struct {
	ID              string
	RuntimeID       string
	RuntimeName     string
	Host            string
	Port            int
	User            string
	TmuxSession     string
	CreatedByUserID string
	CreatedByEmail  string
	CreatedAt       time.Time

	mu             sync.RWMutex
	Status         string
	UpdatedAt      time.Time
	ExitCode       *int
	ExitError      string
	ExitedAt       *time.Time
	LastDetachedAt *time.Time
	buffer         []byte
	subscribers    map[chan adminSSHStreamEvent]struct{}
	ptmx           *os.File
	cmd            *exec.Cmd
}

type AdminSSHSessionResponse struct {
	ID            string  `json:"id"`
	RuntimeID     string  `json:"runtime_id"`
	RuntimeName   string  `json:"runtime_name"`
	Status        string  `json:"status"`
	CanReconnect  bool    `json:"can_reconnect"`
	Host          string  `json:"host"`
	Port          int     `json:"port"`
	User          string  `json:"user"`
	TmuxSession   string  `json:"tmux_session"`
	WebSocketPath string  `json:"websocket_path"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	LastActiveAt  string  `json:"last_active_at"`
	ExitedAt      *string `json:"exited_at,omitempty"`
	ExitCode      *int    `json:"exit_code,omitempty"`
	ExitError     string  `json:"exit_error,omitempty"`
}

type SSHSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*adminSSHSession
}

func NewSSHSessionStore() *SSHSessionStore {
	store := &SSHSessionStore{
		sessions: make(map[string]*adminSSHSession),
	}
	go store.runJanitor()
	return store
}

func (s *SSHSessionStore) runJanitor() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var toClose []*adminSSHSession
		var toDelete []string

		s.mu.RLock()
		for id, session := range s.sessions {
			if session.shouldExpireDetached(adminSSHSessionTTL) {
				toClose = append(toClose, session)
				continue
			}
			if session.shouldDelete(adminSSHSessionTTL) {
				toDelete = append(toDelete, id)
			}
		}
		s.mu.RUnlock()

		for _, session := range toClose {
			session.close("session expired after disconnect")
		}

		if len(toDelete) == 0 {
			continue
		}

		s.mu.Lock()
		for _, id := range toDelete {
			delete(s.sessions, id)
		}
		s.mu.Unlock()
	}
}

func (s *SSHSessionStore) Create(session *adminSSHSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
}

func (s *SSHSessionStore) Get(id string) (*adminSSHSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *SSHSessionStore) FindReusable(runtimeID, userID string) (*adminSSHSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		session.mu.RLock()
		matches := session.RuntimeID == runtimeID &&
			session.CreatedByUserID == userID &&
			(session.Status == "connected" || session.Status == "starting")
		session.mu.RUnlock()
		if matches {
			return session, true
		}
	}

	return nil, false
}

func (s *SSHSessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *adminSSHSession) shouldExpireDetached(ttl time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Status != "connected" || len(s.subscribers) > 0 || s.LastDetachedAt == nil {
		return false
	}
	return time.Since(*s.LastDetachedAt) > ttl
}

func (s *adminSSHSession) shouldDelete(ttl time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Status != "closed" && s.Status != "exited" {
		return false
	}
	if s.ExitedAt == nil {
		return false
	}
	return time.Since(*s.ExitedAt) > ttl
}

func (s *adminSSHSession) appendOutput(chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer = append(s.buffer, chunk...)
	if len(s.buffer) > adminSSHOutputBufferBytes {
		s.buffer = append([]byte(nil), s.buffer[len(s.buffer)-adminSSHOutputBufferBytes:]...)
	}
	s.UpdatedAt = time.Now().UTC()
}

func (s *adminSSHSession) publish(event adminSSHStreamEvent) {
	s.mu.RLock()
	subscribers := make([]chan adminSSHStreamEvent, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.mu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
			s.unsubscribe(ch)
		}
	}
}

func (s *adminSSHSession) unsubscribe(ch chan adminSSHStreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subscribers[ch]; !ok {
		return
	}
	delete(s.subscribers, ch)
	if len(s.subscribers) == 0 {
		now := time.Now().UTC()
		s.LastDetachedAt = &now
	}
}

func (s *adminSSHSession) subscribe() (chan adminSSHStreamEvent, []byte, *adminSSHStreamEvent) {
	ch := make(chan adminSSHStreamEvent, 64)

	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.LastDetachedAt = nil
	buffer := append([]byte(nil), s.buffer...)

	var exitEvent *adminSSHStreamEvent
	if s.Status == "closed" || s.Status == "exited" {
		exitCode := s.ExitCode
		exitEvent = &adminSSHStreamEvent{
			Type:     "exit",
			ExitCode: exitCode,
			Error:    s.ExitError,
		}
	}
	s.mu.Unlock()

	return ch, buffer, exitEvent
}

func (s *adminSSHSession) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdatedAt = time.Now().UTC()
}

func (s *adminSSHSession) snapshot() AdminSSHSessionResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exitedAt *string
	if s.ExitedAt != nil {
		formatted := formatAdminSSHTime(*s.ExitedAt)
		exitedAt = &formatted
	}

	return AdminSSHSessionResponse{
		ID:            s.ID,
		RuntimeID:     s.RuntimeID,
		RuntimeName:   s.RuntimeName,
		Status:        s.Status,
		CanReconnect:  s.Status == "connected",
		Host:          s.Host,
		Port:          s.Port,
		User:          s.User,
		TmuxSession:   s.TmuxSession,
		WebSocketPath: fmt.Sprintf("/api/admin/ssh-sessions/%s/ws", s.ID),
		CreatedAt:     formatAdminSSHTime(s.CreatedAt),
		UpdatedAt:     formatAdminSSHTime(s.UpdatedAt),
		LastActiveAt:  formatAdminSSHTime(s.UpdatedAt),
		ExitedAt:      exitedAt,
		ExitCode:      s.ExitCode,
		ExitError:     s.ExitError,
	}
}

func (s *adminSSHSession) writeInput(data string) error {
	s.mu.RLock()
	ptmx := s.ptmx
	status := s.Status
	s.mu.RUnlock()

	if status != "connected" || ptmx == nil {
		return errors.New("session is not connected")
	}

	_, err := ptmx.Write([]byte(data))
	if err == nil {
		s.touch()
	}
	return err
}

func (s *adminSSHSession) resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return errors.New("invalid terminal size")
	}

	s.mu.RLock()
	ptmx := s.ptmx
	status := s.Status
	s.mu.RUnlock()

	if status != "connected" || ptmx == nil {
		return errors.New("session is not connected")
	}

	if err := pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}); err != nil {
		return err
	}
	s.touch()
	return nil
}

func (s *adminSSHSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.appendOutput(chunk)
			s.publish(adminSSHStreamEvent{
				Type: "output",
				Data: string(chunk),
			})
		}
		if err != nil {
			return
		}
	}
}

func (s *adminSSHSession) waitLoop() {
	err := s.cmd.Wait()

	exitCode := 0
	exitError := ""
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		exitError = err.Error()
	}

	now := time.Now().UTC()
	alreadyClosed := false

	s.mu.Lock()
	if s.Status == "closed" {
		alreadyClosed = true
	} else {
		s.Status = "exited"
		s.ExitCode = &exitCode
		s.ExitError = exitError
		s.ExitedAt = &now
		s.UpdatedAt = now
	}
	ptmx := s.ptmx
	s.ptmx = nil
	s.mu.Unlock()

	if ptmx != nil {
		_ = ptmx.Close()
	}

	if alreadyClosed {
		return
	}

	s.publish(adminSSHStreamEvent{
		Type:     "exit",
		ExitCode: &exitCode,
		Error:    exitError,
	})

	slog.Info("admin ssh session exited",
		"session_id", s.ID,
		"runtime_id", s.RuntimeID,
		"host", s.Host,
		"user_id", s.CreatedByUserID,
		"exit_code", exitCode,
		"error", exitError,
	)
}

func (s *adminSSHSession) close(reason string) {
	now := time.Now().UTC()

	s.mu.Lock()
	if s.Status == "closed" || s.Status == "exited" {
		s.mu.Unlock()
		return
	}

	s.Status = "closed"
	s.ExitError = reason
	s.ExitedAt = &now
	s.UpdatedAt = now
	exitCode := -1
	s.ExitCode = &exitCode
	ptmx := s.ptmx
	cmd := s.cmd
	s.ptmx = nil
	s.mu.Unlock()

	if ptmx != nil {
		_ = ptmx.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	s.publish(adminSSHStreamEvent{
		Type:     "exit",
		ExitCode: &exitCode,
		Error:    reason,
	})

	slog.Info("admin ssh session closed",
		"session_id", s.ID,
		"runtime_id", s.RuntimeID,
		"host", s.Host,
		"user_id", s.CreatedByUserID,
		"reason", reason,
	)
}

func formatAdminSSHTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseRuntimeSSHConfig(metadata []byte) (runtimeSSHConfig, error) {
	cfg := runtimeSSHConfig{Port: 22}
	if len(metadata) == 0 {
		return cfg, errors.New("runtime does not have SSH metadata configured")
	}

	var raw map[string]any
	if err := json.Unmarshal(metadata, &raw); err != nil {
		return cfg, errors.New("runtime metadata is invalid")
	}

	enabled, ok := raw["ssh_enabled"].(bool)
	if !ok || !enabled {
		return cfg, errors.New("runtime SSH is not enabled")
	}
	cfg.Enabled = enabled

	host, _ := raw["ssh_host"].(string)
	user, _ := raw["ssh_user"].(string)
	cfg.Host = strings.TrimSpace(host)
	cfg.User = strings.TrimSpace(user)
	if cfg.Host == "" || cfg.User == "" {
		return cfg, errors.New("runtime SSH host and user are required")
	}

	if rawPort, ok := raw["ssh_port"]; ok {
		switch v := rawPort.(type) {
		case float64:
			cfg.Port = int(v)
		case int:
			cfg.Port = v
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				cfg.Port = parsed
			}
		}
	}
	if cfg.Port <= 0 {
		cfg.Port = 22
	}

	return cfg, nil
}

func adminSSHKeyPath() string {
	return strings.TrimSpace(os.Getenv("SUPER_ADMIN_SSH_KEY_PATH"))
}

func adminSSHKnownHostsPath() string {
	return strings.TrimSpace(os.Getenv("SUPER_ADMIN_SSH_KNOWN_HOSTS_PATH"))
}

func buildAdminSSHCommand(cfg runtimeSSHConfig) (*exec.Cmd, error) {
	keyPath := adminSSHKeyPath()
	knownHostsPath := adminSSHKnownHostsPath()
	if keyPath == "" {
		return nil, errors.New("SUPER_ADMIN_SSH_KEY_PATH is not configured")
	}
	if knownHostsPath == "" {
		return nil, errors.New("SUPER_ADMIN_SSH_KNOWN_HOSTS_PATH is not configured")
	}
	if _, err := os.Stat(keyPath); err != nil {
		return nil, fmt.Errorf("SSH private key is unavailable: %w", err)
	}
	if _, err := os.Stat(knownHostsPath); err != nil {
		return nil, fmt.Errorf("SSH known_hosts file is unavailable: %w", err)
	}

	args := []string{
		"-tt",
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "ConnectTimeout=10",
		"-o", "UserKnownHostsFile=" + knownHostsPath,
		"-i", keyPath,
		"-p", strconv.Itoa(cfg.Port),
		fmt.Sprintf("%s@%s", cfg.User, cfg.Host),
		"sh", "-lc",
		fmt.Sprintf("tmux attach -t %s || tmux new -s %s", adminSSHTmuxSessionName, adminSSHTmuxSessionName),
	}

	cmd := exec.Command("ssh", args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	return cmd, nil
}

func (h *Handler) requireSuperAdminUser(w http.ResponseWriter, r *http.Request) (db.User, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return db.User{}, false
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return db.User{}, false
	}

	if !auth.IsSuperAdminEmail(user.Email) {
		writeError(w, http.StatusForbidden, "super admin access required")
		return db.User{}, false
	}

	return user, true
}

func (h *Handler) createAdminSSHSession(runtime db.AgentRuntime, user db.User, cfg runtimeSSHConfig) (*adminSSHSession, error) {
	cmd, err := buildAdminSSHCommand(cfg)
	if err != nil {
		return nil, err
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 36})
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	session := &adminSSHSession{
		ID:              randomID(),
		RuntimeID:       uuidToString(runtime.ID),
		RuntimeName:     runtime.Name,
		Host:            cfg.Host,
		Port:            cfg.Port,
		User:            cfg.User,
		TmuxSession:     adminSSHTmuxSessionName,
		CreatedByUserID: uuidToString(user.ID),
		CreatedByEmail:  user.Email,
		CreatedAt:       now,
		Status:          "connected",
		UpdatedAt:       now,
		subscribers:     make(map[chan adminSSHStreamEvent]struct{}),
		ptmx:            ptmx,
		cmd:             cmd,
	}

	go session.readLoop()
	go session.waitLoop()

	slog.Info("admin ssh session started",
		"session_id", session.ID,
		"runtime_id", session.RuntimeID,
		"runtime_name", session.RuntimeName,
		"host", session.Host,
		"port", session.Port,
		"user", session.User,
		"admin_user_id", session.CreatedByUserID,
		"admin_email", session.CreatedByEmail,
	)

	return session, nil
}

func (h *Handler) CreateAdminSSHSession(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireSuperAdminUser(w, r)
	if !ok {
		return
	}

	runtimeID := chi.URLParam(r, "runtimeId")
	runtime, err := h.Queries.GetAgentRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusNotFound, "runtime not found")
		return
	}
	if runtime.Status != "online" {
		writeError(w, http.StatusConflict, "runtime is offline")
		return
	}

	sshConfig, err := parseRuntimeSSHConfig(runtime.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if existing, ok := h.SSHSessionStore.FindReusable(uuidToString(runtime.ID), uuidToString(user.ID)); ok {
		writeJSON(w, http.StatusOK, existing.snapshot())
		return
	}

	session, err := h.createAdminSSHSession(runtime, user, sshConfig)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	h.SSHSessionStore.Create(session)
	writeJSON(w, http.StatusCreated, session.snapshot())
}

func (h *Handler) GetAdminSSHSession(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireSuperAdminUser(w, r)
	if !ok {
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	session, ok := h.SSHSessionStore.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "ssh session not found")
		return
	}
	if session.CreatedByUserID != uuidToString(user.ID) {
		writeError(w, http.StatusForbidden, "ssh session not available to this user")
		return
	}

	writeJSON(w, http.StatusOK, session.snapshot())
}

func (h *Handler) DeleteAdminSSHSession(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireSuperAdminUser(w, r)
	if !ok {
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	session, ok := h.SSHSessionStore.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "ssh session not found")
		return
	}
	if session.CreatedByUserID != uuidToString(user.ID) {
		writeError(w, http.StatusForbidden, "ssh session not available to this user")
		return
	}

	session.close("session closed by super admin")
	writeJSON(w, http.StatusOK, session.snapshot())
}

func authenticateAdminWebSocket(r *http.Request) (string, string, error) {
	tokenStr := strings.TrimSpace(r.URL.Query().Get("token"))
	if tokenStr == "" {
		return "", "", errors.New("missing token")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return auth.JWTSecret(), nil
	})
	if err != nil || !token.Valid {
		return "", "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid token claims")
	}

	userID, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	if strings.TrimSpace(userID) == "" {
		return "", "", errors.New("missing user identity")
	}
	if !auth.IsSuperAdminEmail(email) {
		return "", "", errors.New("super admin access required")
	}

	return userID, email, nil
}

func (h *Handler) AdminSSHWebSocket(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	session, ok := h.SSHSessionStore.Get(sessionID)
	if !ok {
		http.Error(w, `{"error":"ssh session not found"}`, http.StatusNotFound)
		return
	}

	userID, _, err := authenticateAdminWebSocket(r)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusUnauthorized)
		return
	}
	if session.CreatedByUserID != userID {
		http.Error(w, `{"error":"ssh session not available to this user"}`, http.StatusForbidden)
		return
	}

	conn, err := adminSSHUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("admin ssh websocket upgrade failed", "session_id", sessionID, "error", err)
		return
	}

	events, backlog, exitEvent := session.subscribe()
	stopWriter := make(chan struct{})
	defer func() {
		session.unsubscribe(events)
		close(stopWriter)
		_ = conn.Close()
	}()

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		defer conn.Close()

		if err := conn.WriteJSON(adminSSHStreamEvent{
			Type: "ready",
			Data: session.ID,
		}); err != nil {
			return
		}

		if len(backlog) > 0 {
			if err := conn.WriteJSON(adminSSHStreamEvent{
				Type: "output",
				Data: string(backlog),
			}); err != nil {
				return
			}
		}
		if exitEvent != nil {
			_ = conn.WriteJSON(*exitEvent)
			return
		}

		for {
			select {
			case <-stopWriter:
				return
			case event := <-events:
				if err := conn.WriteJSON(event); err != nil {
					return
				}
				if event.Type == "exit" {
					return
				}
			}
		}
	}()

	for {
		var msg adminSSHClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "stdin":
			if msg.Data == "" {
				continue
			}
			if err := session.writeInput(msg.Data); err != nil {
				session.publish(adminSSHStreamEvent{
					Type:  "error",
					Error: err.Error(),
				})
				break
			}
		case "resize":
			if err := session.resize(msg.Cols, msg.Rows); err != nil {
				session.publish(adminSSHStreamEvent{
					Type:  "error",
					Error: err.Error(),
				})
				break
			}
		default:
			session.publish(adminSSHStreamEvent{
				Type:  "error",
				Error: "unsupported message type",
			})
		}
	}

	<-writeDone
}
