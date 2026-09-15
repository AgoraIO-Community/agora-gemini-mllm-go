---
recipe_version: 1.0.0
recipe_status: experimental
extension_points:
  - id: agent.prompt
    name: Ada system prompt and greeting
  - id: agent.pipeline
    name: Gemini MLLM model, thinking level, VAD, and session settings
  - id: api.routes
    name: Go backend routes exposed through Next rewrites
  - id: ui.conversation
    name: pre-call, conversation, transcript, metrics, and microphone UI
  - id: verification.contracts
    name: API, proxy, local Go, and backend verification harnesses
invariants:
  - id: api.browser-paths
    summary: browser code calls /api/* and never the Go backend URL directly
  - id: routing.rewrite-only
    summary: Next rewrites proxy /api/* to Go; no client/app/api route handlers exist
  - id: secrets.server-only
    summary: AGORA_APP_CERTIFICATE remains server-side
  - id: uid.concrete-rtm
    summary: RTM login and renewal use concrete non-zero UIDs
stable_contracts:
  - id: env.required
    summary: AGORA_APP_ID, AGORA_APP_CERTIFICATE, and GOOGLE_API_KEY are required by the Go server
  - id: api.config
    summary: GET /api/get_config returns app_id, token, uid, channel_name, and agent_uid
  - id: api.lifecycle
    summary: POST /api/startAgent starts a session; POST /api/stopAgent stops it by agentId
  - id: verification.make
    summary: root Make targets are the canonical setup, dev, build, test, and verification interface
---

# Recipe Contract

This repo is a base recipe for a Go-backed Agora Conversational AI quickstart. It publishes the extension points that downstream recipes can customize while keeping the browser contract, secret boundary, and local verification flow stable.

## Recipe Role

- Role: `base` quickstart recipe.
- Target audience: developers bootstrapping a production-style Conversational AI app with a Go Gin backend and Next.js web client.
- Reuse model: keep this demo beside the local Go SDK checkout, configure three secrets, run, then customize the MLLM or UI.

## Recipe Scope

This base recipe provides a copyable split-process starter with:

- Go Gin token generation and managed agent lifecycle.
- Next.js browser UI with RTC audio, RTM events, transcript, metrics, and connection status.
- Rewrite-only `/api/*` browser facade that hides backend placement.
- One Gemini 3.8 MLLM handles voice end to end; there is no separate STT, LLM, or TTS stage.
- Contract and local smoke verification that do not require live Agora calls.

## Baseline Implementation Guidance

This Gemini demo is adapted from the Go-backed Agora quickstart. Use its source and progressive disclosure docs as the starting point for further customization.

Do not recreate Agora ConvoAI integration from memory. Provider schemas, SDK builder fields, token behavior, and RTM event details can drift. For a new baseline implementation, follow [L1/L2/from_scratch_bootstrap.md](L1/L2/from_scratch_bootstrap.md) while copying verified patterns from this repo.

## Extension Points

| ID | Surface | Files | Intended Changes |
| -- | ------- | ----- | ---------------- |
| `agent.prompt` | Agent identity and first utterance | `server/agent.go` | Change `adaPrompt` or `demoGreeting`; this demo uses either public Gemini 3.8 model ID. |
| `agent.pipeline` | Gemini MLLM and session behavior | `server/agent.go`, `docs/ai/L1/L2/managed_agent_config.md` | Select the public Gemini 3.8 model, set thinking only for Extended Thinking, and tune VAD, RTM, metrics, idle timeout, or expiry. |
| `api.routes` | Backend and browser API contract | `server/main.go`, `client/next.config.ts`, `client/src/services/api.ts`, `client/scripts/verify-api-contracts.ts` | Add or change route handlers, rewrites, request bodies, response payloads, and contract checks together. |
| `ui.conversation` | Browser conversation experience | `client/src/components/`, `client/src/lib/conversation.ts`, `client/src/types/conversation.ts` | Customize pre-call UI, connection details, transcript rendering, metrics, visualizer, mic controls, or end-call behavior. |
| `verification.contracts` | Local confidence checks | `Makefile`, `package.json`, `client/scripts/`, `server/main_test.go`, `server/cmd/fake-server/main.go` | Extend checks when routes, request shapes, or local runtime assumptions change. |

## Invariants

| ID | Rule | Why It Matters |
| -- | ---- | -------------- |
| `api.browser-paths` | Browser code calls `/api/get_config`, `/api/startAgent`, and `/api/stopAgent`; it does not call `AGENT_BACKEND_URL` directly. | Keeps local and deployed browser code identical. |
| `routing.rewrite-only` | `client/next.config.ts` owns `/api/*` rewrites; `client/app/api/**/route.ts` must not be introduced unless the architecture intentionally changes. | Prevents route handlers from shadowing rewrites and splitting behavior. |
| `secrets.server-only` | `AGORA_APP_CERTIFICATE` stays in the Go server environment and never in `client/` or `NEXT_PUBLIC_*` variables. | Protects token signing credentials. |
| `uid.concrete-rtm` | Backend-generated UIDs must be non-zero before RTM login or renewal. | RTM tokens are tied to a concrete login subject; `0` is not valid for RTM. |
| `token.renewal-two-uids` | Renewal keeps separate RTC and RTM token requests when their UIDs can differ. | Prevents RTM renewal with a token minted for the wrong UID. |
| `agent.gemini-mllm` | Keep one `NewGeminiLive` provider and the Google key server-side. | Preserves the end-to-end voice flow and preview gateway routing. |

## Stable Contracts

| Contract | Shape |
| -------- | ----- |
| Setup | `make setup` prepares env template, Go deps, and pnpm workspace deps. |
| Local dev | `make dev` starts Gin on `localhost:8000`; Next chooses an available frontend port and proxies through `AGENT_BACKEND_URL=http://localhost:8000`. |
| Required env | Go server requires `AGORA_APP_ID` and `AGORA_APP_CERTIFICATE`; required `GOOGLE_API_KEY`; pass `PORT` only at launch. |
| Rewrite env | Next requires `AGENT_BACKEND_URL` anywhere `/api/*` should resolve to the Go backend. |
| Config API | `GET /api/get_config?channel=&uid=` returns `{ code, msg, data: { app_id, token, uid, channel_name, agent_uid } }`. |
| Start API | `POST /api/startAgent` sends `{ channelName, rtcUid, userUid, model, thinkingLevel? }`; `thinkingLevel` is low/medium/high only for Extended Thinking and returns `data.agent_id`. |
| Stop API | `POST /api/stopAgent` sends `{ agentId }`; the client skips the request when the id is empty. |
| Verification | `make verify` is web-focused; `make verify-local` adds local Go-backed checks; `make verify-backend` runs Go tests. |

## Internal / Subject to Change

- Component composition inside `client/src/components/` can change as long as `LandingPage` still owns bootstrap and `ConversationComponent` still owns active RTC/RTM orchestration.
- Exact UI copy, Tailwind classes, logos, layout components, and visualizer presentation are not stable recipe contracts.
- Gemini model IDs, thinking defaults, and VAD numbers are recipe defaults; update docs and tests when changing them.
- `server/agent.go` may be split into more files later, but the Go server remains the owner of Agora SDK calls and secret-backed token generation.
- Verification internals under `client/scripts/` may change, but route contracts and Make target names should remain stable for downstream recipes.

## Related Progressive Disclosure Docs

- `L1/01_setup.md` — setup, env, and command reference.
- `L1/02_architecture.md` — request flow and component topology.
- `L1/05_workflows.md` — common modification workflows.
- `L1/06_interfaces.md` — route, rewrite, env, and event contracts.
- `L1/L2/from_scratch_bootstrap.md` — implementation map for recreating the Go-backed quickstart recipe.
- `L1/L2/managed_agent_config.md` — full agent config detail.
- `L1/L2/session_lifecycle.md` — RTC/RTM/session orchestration.
- `L1/L2/verification_scripts.md` — verification harness behavior.
