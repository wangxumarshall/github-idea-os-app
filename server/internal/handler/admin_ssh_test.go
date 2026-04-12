package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func loadHandlerTestRuntimeID(t *testing.T) string {
	t.Helper()

	var runtimeID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id
		FROM agent_runtime
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&runtimeID); err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	return runtimeID
}

func setRuntimeMetadata(t *testing.T, runtimeID string, metadata string) {
	t.Helper()

	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_runtime
		SET metadata = $2::jsonb, status = 'online', updated_at = now()
		WHERE id = $1
	`, runtimeID, metadata); err != nil {
		t.Fatalf("update runtime metadata: %v", err)
	}
}

func setupFakeSSHEnvironment(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	sshPath := filepath.Join(tempDir, "ssh")
	script := "#!/bin/sh\nprintf 'connected via fake ssh\\r\\n'\ncat\n"
	if err := os.WriteFile(sshPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ssh script: %v", err)
	}

	keyPath := filepath.Join(tempDir, "id_ed25519")
	if err := os.WriteFile(keyPath, []byte("fake-key"), 0o600); err != nil {
		t.Fatalf("write fake key: %v", err)
	}

	knownHostsPath := filepath.Join(tempDir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte("fake-host ssh-ed25519 AAAA\n"), 0o644); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	t.Setenv("PATH", tempDir+":"+os.Getenv("PATH"))
	t.Setenv("SUPER_ADMIN_SSH_KEY_PATH", keyPath)
	t.Setenv("SUPER_ADMIN_SSH_KNOWN_HOSTS_PATH", knownHostsPath)
}

func TestCreateAdminSSHSessionRejectsNonSuperAdmin(t *testing.T) {
	runtimeID := loadHandlerTestRuntimeID(t)
	setRuntimeMetadata(t, runtimeID, `{"ssh_enabled":true,"ssh_host":"runtime.test","ssh_port":22,"ssh_user":"ubuntu"}`)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/admin/runtimes/"+runtimeID+"/ssh-sessions", nil)
	req = withURLParam(req, "runtimeId", runtimeID)

	testHandler.CreateAdminSSHSession(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("CreateAdminSSHSession: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateAdminSSHSessionRejectsMissingMetadata(t *testing.T) {
	t.Setenv("SUPER_ADMIN_EMAIL", handlerTestEmail)

	runtimeID := loadHandlerTestRuntimeID(t)
	setRuntimeMetadata(t, runtimeID, `{}`)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/admin/runtimes/"+runtimeID+"/ssh-sessions", nil)
	req = withURLParam(req, "runtimeId", runtimeID)

	testHandler.CreateAdminSSHSession(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateAdminSSHSession: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateAdminSSHSessionStartsAndStopsSession(t *testing.T) {
	t.Setenv("SUPER_ADMIN_EMAIL", handlerTestEmail)
	setupFakeSSHEnvironment(t)

	runtimeID := loadHandlerTestRuntimeID(t)
	setRuntimeMetadata(t, runtimeID, `{"ssh_enabled":true,"ssh_host":"runtime.test","ssh_port":22,"ssh_user":"ubuntu"}`)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/admin/runtimes/"+runtimeID+"/ssh-sessions", nil)
	req = withURLParam(req, "runtimeId", runtimeID)

	testHandler.CreateAdminSSHSession(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAdminSSHSession: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created AdminSSHSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Status != "connected" {
		t.Fatalf("expected connected status, got %q", created.Status)
	}
	if !created.CanReconnect {
		t.Fatal("expected session to be reconnectable")
	}

	w = httptest.NewRecorder()
	req = newRequest(http.MethodPost, "/api/admin/runtimes/"+runtimeID+"/ssh-sessions", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.CreateAdminSSHSession(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CreateAdminSSHSession reuse: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var reused AdminSSHSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&reused); err != nil {
		t.Fatalf("decode reused response: %v", err)
	}
	if reused.ID != created.ID {
		t.Fatalf("expected reused session %q, got %q", created.ID, reused.ID)
	}

	w = httptest.NewRecorder()
	req = newRequest(http.MethodDelete, "/api/admin/ssh-sessions/"+created.ID, nil)
	req = withURLParam(req, "sessionId", created.ID)
	testHandler.DeleteAdminSSHSession(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DeleteAdminSSHSession: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminSSHWebSocketStreamsOutput(t *testing.T) {
	t.Setenv("SUPER_ADMIN_EMAIL", handlerTestEmail)
	setupFakeSSHEnvironment(t)

	runtimeID := loadHandlerTestRuntimeID(t)
	setRuntimeMetadata(t, runtimeID, `{"ssh_enabled":true,"ssh_host":"runtime.test","ssh_port":22,"ssh_user":"ubuntu"}`)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/admin/runtimes/"+runtimeID+"/ssh-sessions", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.CreateAdminSSHSession(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAdminSSHSession: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created AdminSSHSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	user, err := testHandler.Queries.GetUser(context.Background(), parseUUID(testUserID))
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	token, err := testHandler.issueJWT(user)
	if err != nil {
		t.Fatalf("issue JWT: %v", err)
	}

	router := chi.NewRouter()
	router.Get("/api/admin/ssh-sessions/{sessionId}/ws", testHandler.AdminSSHWebSocket)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/api/admin/ssh-sessions/" + created.ID + "/ws?token=" + url.QueryEscape(token)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg adminSSHStreamEvent
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read ready event: %v", err)
	}
	if msg.Type != "ready" {
		t.Fatalf("expected ready event, got %+v", msg)
	}

	foundOutput := false
	for i := 0; i < 4; i++ {
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("read output event: %v", err)
		}
		if msg.Type == "output" && strings.Contains(msg.Data, "connected via fake ssh") {
			foundOutput = true
			break
		}
	}
	if !foundOutput {
		t.Fatal("expected backlog output from fake ssh")
	}

	if err := conn.WriteJSON(adminSSHClientMessage{
		Type: "stdin",
		Data: "ping from websocket\n",
	}); err != nil {
		t.Fatalf("write stdin event: %v", err)
	}

	echoed := false
	for i := 0; i < 6; i++ {
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("read echoed output: %v", err)
		}
		if msg.Type == "output" && strings.Contains(msg.Data, "ping from websocket") {
			echoed = true
			break
		}
	}
	if !echoed {
		t.Fatal("expected echoed stdin from fake ssh session")
	}

	closeReq := newRequest(http.MethodDelete, "/api/admin/ssh-sessions/"+created.ID, nil)
	closeReq = withURLParam(closeReq, "sessionId", created.ID)
	closeW := httptest.NewRecorder()
	testHandler.DeleteAdminSSHSession(closeW, closeReq)
	if closeW.Code != http.StatusOK {
		t.Fatalf("DeleteAdminSSHSession: expected 200, got %d: %s", closeW.Code, closeW.Body.String())
	}

	for i := 0; i < 4; i++ {
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("read exit event: %v", err)
		}
		if msg.Type == "exit" {
			return
		}
	}
	t.Fatal("expected exit event after closing session")
}

func TestAuthenticateAdminWebSocketRejectsInvalidToken(t *testing.T) {
	_, _, err := authenticateAdminWebSocket(httptest.NewRequest(
		http.MethodGet,
		"/api/admin/ssh-sessions/session/ws?token=bad-token",
		bytes.NewReader(nil),
	))
	if err == nil {
		t.Fatal("expected invalid token error")
	}
}
