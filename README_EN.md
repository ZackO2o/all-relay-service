# ALL Relay Service

> Multi-platform AI API relay service — one endpoint to connect all major AI models.

---

## Overview

ALL Relay Service is a unified multi-platform AI API relay middleware that sits between your clients and upstream AI providers. It supports OpenAI, Anthropic Claude, Google Gemini, and Chinese domestic models (DeepSeek, Qwen, GLM, MiniMax, Kimi).

### Key Features

- **Multi-account management** — Pool and schedule OpenAI / Claude / Gemini accounts
- **Unified API** — OpenAI-compatible Chat Completions format across all models
- **Model aliasing** — Smart model name mapping (`gpt-5.5-codex` → `gpt-5.5`)
- **Health monitoring** — Auto-detect rate-limited or failed accounts
- **Usage analytics** — Complete request logging and cost tracking
- **Webhook notifications** — Push alerts for account anomalies
- **Subscription sharing** — Share accounts across users with usage-based billing
- **Admin dashboard** — Chinese-friendly UI with mobile support

---

## Quick Start

### Prerequisites

- Node.js >= 18.x
- Redis >= 6.x
- npm / yarn

### Install

```bash
git clone https://github.com/ZackO2o/all-relay-service.git
cd all-relay-service
npm install
npm run setup
npm run dev
```

### Docker

```bash
docker compose up -d
```

Admin dashboard: `http://localhost:3000/admin-next/login`

---

## Supported Models

### OpenAI

| Model | Description |
|-------|-------------|
| gpt-5.5 | Latest GPT model |
| gpt-5.5-pro | GPT enhanced |
| gpt-5.5-codex | Codex CLI model |
| gpt-5.4 | Previous flagship |
| gpt-5.4-mini / gpt-5.4-nano | Lightweight |
| o3 / o4-mini | Reasoning models |

### Anthropic

| Model | Description |
|-------|-------------|
| claude-opus-4-7 | Claude Opus |
| claude-sonnet-4-6 | Claude Sonnet |
| claude-haiku-4-5 | Claude Haiku |

### Google

| Model | Description |
|-------|-------------|
| gemini-2.5-pro | Gemini flagship |
| gemini-3-pro-preview | Gemini 3 series |

### Chinese Models

| Model | Provider |
|-------|----------|
| deepseek-chat / deepseek-reasoner | DeepSeek |
| Qwen3-235B-A22B / Qwen3.5-397B-A17B | Alibaba Qwen |
| MiniMax-M2.5 / MiniMax-M2.7 | MiniMax |
| glm-5 / glm-5.1 | Zhipu GLM |
| kimi-k2.5 | Moonshot Kimi |

---

## API Usage

### OpenAI-Compatible

```bash
curl http://localhost:3000/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "model": "gpt-5.5",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### Anthropic Messages

```bash
curl http://localhost:3000/api/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | 3000 |
| `REDIS_HOST` | Redis host | localhost |
| `REDIS_PORT` | Redis port | 6379 |
| `JWT_SECRET` | JWT secret (required) | - |
| `ENCRYPTION_KEY` | Encryption key (required, 32 chars) | - |
| `LOG_LEVEL` | Log level | info |

---

## License

MIT
