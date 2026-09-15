package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	Agora "github.com/AgoraIO/agora-agents-go/v2"
	"github.com/AgoraIO/agora-agents-go/v2/agentkit"
	"github.com/AgoraIO/agora-agents-go/v2/agentkit/vendors"
	"github.com/AgoraIO/agora-agents-go/v2/core"
	"github.com/AgoraIO/agora-agents-go/v2/option"
)

const adaPrompt = `You are Ada, an agentic developer advocate from Agora. You help developers understand and build with Agora's Conversational AI platform.

Agora is a real-time communications company. The product you represent is the Agora Conversational AI Engine.

If you do not know a specific fact about Agora, say so plainly and suggest checking docs.agora.io. Keep most replies to one or two sentences unless the user explicitly asks for more detail.
`

const demoGreeting = "Hi there! I'm Ada, your virtual assistant from Agora. How can I help?"

type agentService struct {
	appID         string
	certificate   string
	greeting      string
	googleAPIKey  string
	sessionClient *agentkit.AgoraClient
	stopClient    agentStopper

	mu       sync.Mutex
	sessions map[string]sessionStopper
}

type agentStopper interface {
	StopAgent(ctx context.Context, agentID string) error
}

// The stateless fallback needs the same host and gate as the session start.
type previewStopClient struct {
	client      *agentkit.AgoraClient
	appID       string
	certificate string
}

func (p *previewStopClient) StopAgent(ctx context.Context, agentID string) error {
	token, err := agentkit.GenerateConvoAIToken(agentkit.GenerateConvoAITokenOptions{
		AppID: p.appID, AppCertificate: p.certificate, ChannelName: "stop", UID: 0,
	})
	if err != nil {
		return err
	}
	err = p.client.Agents.Stop(ctx, &Agora.StopAgentsRequest{Appid: p.appID, AgentID: agentID},
		option.WithToken(token),
		option.WithBaseURL(agentkit.PreviewAPIBaseURL),
		option.WithHTTPHeader(http.Header{agentkit.PreviewFeatureHeader: {agentkit.PreviewFeatureGeminiLive}}),
	)
	var apiErr *core.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

type sessionStopper interface {
	Stop(ctx context.Context) error
}

type configData struct {
	AppID       string `json:"app_id"`
	Token       string `json:"token"`
	UID         string `json:"uid"`
	ChannelName string `json:"channel_name"`
	AgentUID    string `json:"agent_uid"`
}

type startAgentResult struct {
	AgentID     string `json:"agent_id"`
	ChannelName string `json:"channel_name"`
	Status      string `json:"status"`
}

func newAgentService() (*agentService, error) {
	appID := strings.TrimSpace(os.Getenv("AGORA_APP_ID"))
	certificate := strings.TrimSpace(os.Getenv("AGORA_APP_CERTIFICATE"))
	if appID == "" || certificate == "" {
		return nil, errors.New("AGORA_APP_ID and AGORA_APP_CERTIFICATE are required")
	}
	googleAPIKey := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	if googleAPIKey == "" {
		return nil, errors.New("GOOGLE_API_KEY is required for the Gemini preview providers")
	}

	// AgentSession detects the Gemini MLLM and pins its preview route and gate.
	agoraClient := agentkit.NewAgoraClient(agentkit.AgoraClientOptions{
		Area:           option.AreaUS,
		AppID:          appID,
		AppCertificate: certificate,
	})

	return &agentService{
		appID:         appID,
		certificate:   certificate,
		googleAPIKey:  googleAPIKey,
		greeting:      demoGreeting,
		sessionClient: agoraClient,
		stopClient:    &previewStopClient{client: agoraClient, appID: appID, certificate: certificate},
		sessions:      make(map[string]sessionStopper),
	}, nil
}

func (s *agentService) generateConfig(channel string, uid int) (*configData, error) {
	userUID := uid
	if userUID <= 0 {
		userUID = randomInt(1000, 9999999)
	}

	channelName := strings.TrimSpace(channel)
	if channelName == "" {
		channelName = generateChannelName()
	}

	agentUID := randomInt(10000000, 99999999)
	expiry, err := agentkit.ExpiresInHours(1)
	if err != nil {
		return nil, fmt.Errorf("resolve token expiry: %w", err)
	}

	token, err := agentkit.GenerateConvoAIToken(agentkit.GenerateConvoAITokenOptions{
		AppID:          s.appID,
		AppCertificate: s.certificate,
		ChannelName:    channelName,
		UID:            userUID,
		TokenExpire:    expiry,
	})
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &configData{
		AppID:       s.appID,
		Token:       token,
		UID:         strconv.Itoa(userUID),
		ChannelName: channelName,
		AgentUID:    strconv.Itoa(agentUID),
	}, nil
}

type modelSelection struct {
	Model         string
	ThinkingLevel string
}

const (
	requestModelLive             = "models/gemini-3.8-live"
	requestModelExtendedThinking = "models/gemini-3.8-live-extended-thinking"
)

func (s *agentService) start(channelName string, agentUID, userUID int, selections ...modelSelection) (*startAgentResult, error) {
	channelName = strings.TrimSpace(channelName)
	if channelName == "" {
		return nil, errors.New("channel_name is required and cannot be empty")
	}
	if agentUID <= 0 {
		return nil, errors.New("agent_uid is required and cannot be empty")
	}
	if userUID <= 0 {
		return nil, errors.New("user_uid is required and cannot be empty")
	}

	expiresIn, err := agentkit.ExpiresInHours(1)
	if err != nil {
		return nil, fmt.Errorf("resolve session expiry: %w", err)
	}

	enableRTM := true
	enableTools := false
	enableErrorMessage := true
	enableMetrics := true
	dataChannel := agentkit.ParametersDataChannel("rtm")
	enableStringUID := false
	idleTimeout := 30
	selected := modelSelection{Model: requestModelLive}
	if len(selections) > 0 {
		selected = selections[0]
		if selected.Model == "" {
			selected.Model = requestModelLive
		}
	}
	if selected.Model != requestModelLive && selected.Model != requestModelExtendedThinking {
		return nil, errors.New("invalid model")
	}
	if selected.ThinkingLevel != "" && (selected.Model != requestModelExtendedThinking || (selected.ThinkingLevel != vendors.GeminiThinkingLevelLow && selected.ThinkingLevel != vendors.GeminiThinkingLevelMedium && selected.ThinkingLevel != vendors.GeminiThinkingLevelHigh)) {
		return nil, errors.New("invalid thinking level for model")
	}
	model := selected.Model
	thinkingLevel := ""
	if selected.Model == requestModelExtendedThinking {
		thinkingLevel = selected.ThinkingLevel
		if thinkingLevel == "" {
			thinkingLevel = vendors.GeminiThinkingLevelMedium
		}
	}

	agent := agentkit.NewAgent(
		s.sessionClient,
		agentkit.WithGreeting(s.greeting),
		agentkit.WithFailureMessage("Please wait a moment."),
		agentkit.WithAdvancedFeatures(&agentkit.AdvancedFeatures{
			EnableRtm:   &enableRTM,
			EnableTools: &enableTools,
		}),
		agentkit.WithParameters(&agentkit.SessionParams{
			DataChannel:        &dataChannel,
			EnableErrorMessage: &enableErrorMessage,
			EnableMetrics:      &enableMetrics,
		}),
		// web client → ultra-low-latency chorus profile
		agentkit.WithAudioScenario(agentkit.ParametersAudioScenario("chorus")),
	).
		// Extended Thinking adds a reasoning budget.
		// In MLLM mode agent-level instructions are not sent, so the system
		// prompt goes on the vendor, where it serialises to mllm.params.instructions.
		WithMllm(vendors.NewGeminiLive(vendors.GeminiLiveOptions{
			APIKey:          s.googleAPIKey,
			Model:           model,
			Instructions:    adaPrompt,
			Voice:           "Puck",
			LanguageCodes:   []string{"en-US"},
			TranscribeAgent: boolPtr(true),
			TranscribeUser:  boolPtr(true),
            GreetingMessage: s.greeting,
			FailureMessage:  "Please wait a moment.",
			TurnDetection: &agentkit.MllmTurnDetectionConfig{
				Mode: agentkit.MllmTurnDetectionModeServerVad.Ptr(),
			},
			ThinkingLevel: thinkingLevel,
		}))

	session := agent.CreateSession(agentkit.CreateSessionOptions{
		Channel:         channelName,
		AgentUID:        strconv.Itoa(agentUID),
		RemoteUIDs:      []string{strconv.Itoa(userUID)},
		EnableStringUID: &enableStringUID,
		IdleTimeout:     &idleTimeout,
		ExpiresIn:       expiresIn,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agentID, err := session.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("start agent: %w", err)
	}

	s.mu.Lock()
	s.sessions[agentID] = session
	s.mu.Unlock()

	return &startAgentResult{
		AgentID:     agentID,
		ChannelName: channelName,
		Status:      "started",
	}, nil
}

func (s *agentService) stop(agentID string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return errors.New("agent_id is required and cannot be empty")
	}

	s.mu.Lock()
	session, ok := s.sessions[agentID]
	if ok {
		delete(s.sessions, agentID)
	}
	s.mu.Unlock()

	if ok {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := session.Stop(ctx); err == nil {
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.stopClient.StopAgent(ctx, agentID); err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}

	return nil
}

func generateChannelName() string {
	return fmt.Sprintf("ai-conversation-%d-%d", time.Now().Unix(), randomInt(1000, 9999))
}

func randomInt(minInclusive, maxInclusive int) int {
	if maxInclusive <= minInclusive {
		return minInclusive
	}
	return minInclusive + rand.Intn(maxInclusive-minInclusive+1)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}
