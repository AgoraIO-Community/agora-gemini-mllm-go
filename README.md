# agora-gemini-mllm — Go demo

This demo pairs a Next.js voice client with a Go Gin backend and the published Agora Go Agent SDK. One `NewGeminiLive` provider handles audio input and output end to end; there is no separate STT, LLM, or TTS stage.

The browser chooses `models/gemini-3.8-live` or `models/gemini-3.8-live-extended-thinking`. Extended Thinking exposes a low/medium/high slider; the regular model sends no thinking level. `POST /api/startAgent` carries the public model ID and optional `thinkingLevel`; Go validates the selection. The SDK routes Gemini sessions to the preview gateway with `agora-feature: gemini-live` and sends the Google credential as `mllm.api_key`.

## Requirements

- Go 1.23+, Node.js 22+, and pnpm.
- Agora App ID, App Certificate, and Google API key.

## Run locally

From this demo folder:

```bash
make setup
# Fill AGORA_APP_ID, AGORA_APP_CERTIFICATE, and GOOGLE_API_KEY in server/.env.local.
pnpm run dev:parallel
```

Gin listens on `http://localhost:8111`. Next.js chooses an available frontend port; open the Local URL it prints. The browser calls Next `/api/*` paths, which rewrite to Gin. The local credential file needs only the three secrets above. Prompt, greeting, model, voice, and session settings live in `server/agent.go`.

## Verify

```bash
cd server && go test ./...
cd ..
pnpm run verify:web
```

With valid credentials, run `cd server && LIVE_GEMINI_CHECK=1 go test -run TestLiveGeminiPreview -v` to start a real session, confirm `RUNNING`, and stop it.

## Deploy

Deploy `client/` and `server/` separately. Set `AGENT_BACKEND_URL` in the Next.js deployment to the public Go backend URL. Keep `AGORA_APP_CERTIFICATE` and `GOOGLE_API_KEY` on the server. The Gemini models in this demo still require the preview gateway.

See [ARCHITECTURE.md](./ARCHITECTURE.md), [AGENTS.md](./AGENTS.md), and the [recipe contract](./docs/ai/RECIPE.md) for the request flow and extension points. Licensed under [MIT](./LICENSE).
