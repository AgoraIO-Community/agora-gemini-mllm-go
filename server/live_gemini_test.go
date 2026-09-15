package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	Agora "github.com/AgoraIO/agora-agents-go/v2"
	"github.com/AgoraIO/agora-agents-go/v2/agentkit"
	"github.com/joho/godotenv"
)

// Run with LIVE_GEMINI_CHECK=1 to verify Agora accepts and runs this demo.
func TestLiveGeminiPreview(t *testing.T) {
	if os.Getenv("LIVE_GEMINI_CHECK") != "1" {
		t.Skip("set LIVE_GEMINI_CHECK=1 to run the live preview check")
	}
	if err := godotenv.Load(".env.local"); err != nil {
		t.Fatal("load .env.local:", err)
	}
	service, err := newAgentService()
	if err != nil {
		t.Fatalf("configure demo: %T", err)
	}
	channel := fmt.Sprintf("sdk-smoke-%d", time.Now().UnixNano())
	started, err := service.start(channel, 19991234, 19995678)
	if err != nil {
		t.Fatalf("START failed: %T", err)
	}
	t.Log("START accepted; agent ID returned")
	defer func() {
		if err := service.stop(started.AgentID); err != nil {
			t.Errorf("STOP failed: %T", err)
		} else {
			t.Log("STOP accepted")
		}
	}()
	time.Sleep(6 * time.Second)
	service.mu.Lock()
	session, ok := service.sessions[started.AgentID].(*agentkit.AgentSession)
	service.mu.Unlock()
	if !ok {
		t.Fatal("active session missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	info, err := session.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GET failed: %T", err)
	}
	if info.Status == nil || *info.Status != Agora.GetAgentsResponseStatusRunning {
		t.Fatalf("unexpected agent status: %v", info.Status)
	}
	t.Log("GET status: RUNNING")
}
