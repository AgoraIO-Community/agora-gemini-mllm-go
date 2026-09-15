package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/AgoraIO/agora-agents-go/v2/agentkit"
	"github.com/AgoraIO/agora-agents-go/v2/agentkit/vendors"
	"github.com/AgoraIO/agora-agents-go/v2/option"
)

// Captures the start request this demo actually sends.
type wireRecorder struct {
	req  *http.Request
	body []byte
}

func (w *wireRecorder) Do(req *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(req.Body)
	w.req, w.body = req, b
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`{"agent_id":"a1"}`)),
		Header:     make(http.Header),
	}, nil
}

func TestPreviewWireShape(t *testing.T) {
	if os.Getenv("WIRE_CHECK") == "" {
		t.Skip("set WIRE_CHECK=1")
	}
	os.Setenv("AGORA_APP_ID", "81190c52971d4004b7244bdcd93e2f34")
	os.Setenv("AGORA_APP_CERTIFICATE", "0123456789abcdef0123456789abcdef")
	os.Setenv("GOOGLE_API_KEY", "FAKEKEY")

	svc, err := newAgentService()
	if err != nil {
		t.Fatal(err)
	}
	// Same package: swap in a preview client whose transport records the request.
	rec := &wireRecorder{}
	svc.sessionClient = agentkit.NewAgoraClient(agentkit.AgoraClientOptions{
		Area:           option.AreaUS,
		AppID:          os.Getenv("AGORA_APP_ID"),
		AppCertificate: os.Getenv("AGORA_APP_CERTIFICATE"),
		HTTPClient:     rec,
	})

	if _, err := svc.start("ch", 123456, 100, modelSelection{Model: requestModelExtendedThinking, ThinkingLevel: vendors.GeminiThinkingLevelMedium}); err != nil {
		t.Logf("start returned: %v", err)
	}
	if rec.req == nil {
		t.Fatal("no request captured")
	}
	if got := rec.req.Header.Get(agentkit.PreviewFeatureHeader); got != agentkit.PreviewFeatureGeminiLive {
		t.Fatalf("preview gate = %q", got)
	}
	if !strings.HasPrefix(rec.req.URL.String(), agentkit.PreviewAPIBaseURL) {
		t.Fatalf("request did not use preview endpoint: %s", rec.req.URL)
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.body, &payload)
	props, _ := payload["properties"].(map[string]interface{})
	out, _ := json.Marshal(props)
	t.Logf("URL   : %s", rec.req.URL)
	t.Logf("gate  : %s", rec.req.Header.Get("agora-feature"))
	t.Logf("props : %.400s", out)
	if mllm, ok := props["mllm"].(map[string]interface{}); ok {
		if params, ok := mllm["params"].(map[string]interface{}); ok {
			expectedModel := requestModelExtendedThinking
			if got := params["model"]; got != expectedModel {
				t.Fatalf("model = %v", got)
			}
			if _, exists := params["language"]; exists {
				t.Fatal("Gemini preview uses language_codes, not language")
			}
			if _, exists := params["language_codes"]; !exists {
				t.Fatal("missing language_codes")
			}
			t.Logf("model=%v thinking_level=%v voice=%v language=%v transcribe_agent=%v",
				params["model"], params["thinking_level"], params["voice"], params["language"], params["transcribe_agent"])
		}
		if _, exists := mllm["greeting_message"]; exists {
			t.Fatal("Gemini preview uses greeting, not greeting_message")
		}
	}
}

func TestDemoModelAndThinkingLevel(t *testing.T) {
	t.Setenv("AGORA_APP_ID", "81190c52971d4004b7244bdcd93e2f34")
	t.Setenv("AGORA_APP_CERTIFICATE", "0123456789abcdef0123456789abcdef")
	t.Setenv("GOOGLE_API_KEY", "FAKEKEY")
	svc, err := newAgentService()
	if err != nil {
		t.Fatal(err)
	}
	rec := &wireRecorder{}
	svc.sessionClient = agentkit.NewAgoraClient(agentkit.AgoraClientOptions{
		Area: option.AreaUS, AppID: os.Getenv("AGORA_APP_ID"), AppCertificate: os.Getenv("AGORA_APP_CERTIFICATE"), HTTPClient: rec,
	})
	if _, err := svc.start("ch", 123456, 100, modelSelection{Model: requestModelExtendedThinking, ThinkingLevel: vendors.GeminiThinkingLevelMedium}); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Properties struct {
			MLLM struct {
				Params map[string]interface{} `json:"params"`
			} `json:"mllm"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(rec.body, &payload); err != nil {
		t.Fatal(err)
	}
	params := payload.Properties.MLLM.Params
	if params["model"] != requestModelExtendedThinking {
		t.Fatalf("model = %v", params["model"])
	}
	if params["thinking_level"] != vendors.GeminiThinkingLevelMedium {
		t.Fatalf("thinking_level = %v", params["thinking_level"])
	}
}

func TestCombinedModelSelection(t *testing.T) {
	t.Setenv("AGORA_APP_ID", "81190c52971d4004b7244bdcd93e2f34")
	t.Setenv("AGORA_APP_CERTIFICATE", "0123456789abcdef0123456789abcdef")
	t.Setenv("GOOGLE_API_KEY", "FAKEKEY")
	for _, tc := range []struct{ model, vendorModel, level string }{
		{requestModelLive, requestModelLive, ""},
		{requestModelExtendedThinking, requestModelExtendedThinking, "low"},
		{requestModelExtendedThinking, requestModelExtendedThinking, "medium"},
		{requestModelExtendedThinking, requestModelExtendedThinking, "high"},
	} {
		t.Run(tc.model+tc.level, func(t *testing.T) {
			svc, err := newAgentService()
			if err != nil {
				t.Fatal(err)
			}
			rec := &wireRecorder{}
			svc.sessionClient = agentkit.NewAgoraClient(agentkit.AgoraClientOptions{
				Area: option.AreaUS, AppID: os.Getenv("AGORA_APP_ID"), AppCertificate: os.Getenv("AGORA_APP_CERTIFICATE"), HTTPClient: rec,
			})
			if _, err := svc.start("ch", 123456, 100, modelSelection{Model: tc.model, ThinkingLevel: tc.level}); err != nil {
				t.Fatal(err)
			}
			var payload struct {
				Properties struct {
					MLLM struct {
						Params map[string]interface{} `json:"params"`
					} `json:"mllm"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(rec.body, &payload); err != nil {
				t.Fatal(err)
			}
			params := payload.Properties.MLLM.Params
			if params["model"] != tc.vendorModel {
				t.Fatalf("model = %v", params["model"])
			}
			if tc.level == "" {
				if _, exists := params["thinking_level"]; exists {
					t.Fatal("low-latency model sent thinking_level")
				}
			} else if params["thinking_level"] != tc.level {
				t.Fatalf("thinking_level = %v", params["thinking_level"])
			}
		})
	}
}
