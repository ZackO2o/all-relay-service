# ALL Relay Service

> 全平台 AI API 中转服务 — 一个服务，连接所有主流 AI 模型

[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

---

## 简介

ALL Relay Service 是一款统一的多平台 AI API 中转服务，作为客户端与各大 AI 厂商 API 之间的中间层。支持 OpenAI、Anthropic Claude、Google Gemini、国产模型（DeepSeek、Qwen、GLM、MiniMax、Kimi）等主流平台。

### 核心能力

- **多账户管理** — OpenAI / Claude / Gemini 等多账户池化调度
- **统一 API** — 兼容 OpenAI Chat Completions 格式，一套 API 调所有模型
- **模型别名** — 智能模型名映射 (`gpt-5.5-codex` → `gpt-5.5`)
- **账户健康监控** — 自动检测失效/限流账户，踢出调度池
- **用量统计** — 完整的请求日志与成本统计
- **Webhook 通知** — 账户异常、余额不足时主动推送
- **拼车模式** — 多人共用订阅，按用量分摊
- **管理后台** — 中文友好界面，移动端适配

---

## 快速开始

### 前置要求

- Node.js >= 18.x
- Redis >= 6.x
- npm / yarn

### 安装

```bash
# 克隆项目
git clone https://github.com/ZackO2o/all-relay-service.git
cd all-relay-service

# 安装依赖
npm install

# 初始化
npm run setup

# 启动开发模式
npm run dev
```

### Docker 部署

```bash
docker compose up -d
```

访问管理后台：`http://localhost:3000/admin-next/login`

---

## 支持的模型

### OpenAI 系列

| 模型 | 说明 |
|------|------|
| gpt-5.5 | 最新 GPT 模型 |
| gpt-5.5-pro | GPT 增强版 |
| gpt-5.5-codex | Codex CLI 专用 |
| gpt-5.4 | 前代旗舰 |
| gpt-5.4-mini / gpt-5.4-nano | 轻量版本 |
| gpt-5.3-codex | 前代 Codex 模型 |
| o3 / o4-mini | 推理模型 |

### Anthropic 系列

| 模型 | 说明 |
|------|------|
| claude-opus-4-7 | Claude Opus |
| claude-sonnet-4-6 | Claude Sonnet |
| claude-haiku-4-5 | Claude Haiku |

### Google 系列

| 模型 | 说明 |
|------|------|
| gemini-2.5-pro | Gemini 旗舰 |
| gemini-3-pro-preview | Gemini 3 系列 |

### 国产模型

| 模型 | 提供方 |
|------|--------|
| deepseek-chat / deepseek-reasoner | DeepSeek |
| Qwen3-235B-A22B / Qwen3.5-397B-A17B | 阿里通义千问 |
| MiniMax-M2.5 / MiniMax-M2.7 | MiniMax |
| glm-5 / glm-5.1 | 智谱 GLM |
| kimi-k2.5 | Moonshot Kimi |

---

## API 使用

### OpenAI 兼容接口

```bash
curl http://localhost:3000/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "model": "gpt-5.5",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

### Anthropic Messages 接口

```bash
curl http://localhost:3000/api/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

---

## 管理后台

部署完成后访问 `/admin-next/login` 进入管理后台：

- **总览** — 实时流量大盘、请求统计
- **API 密钥** — 创建和管理 API Key
- **模型管理** — 控制模型可用性
- **账户池** — 管理上游 API 账户
- **用户管理** — 多用户与权限控制
- **财务中心** — 消费记录与充值
- **系统设置** — 全局配置

---

## 配置

核心配置通过环境变量或 `config/config.js` 设置：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | 3000 |
| `REDIS_HOST` | Redis 地址 | localhost |
| `REDIS_PORT` | Redis 端口 | 6379 |
| `JWT_SECRET` | JWT 密钥（必填） | - |
| `ENCRYPTION_KEY` | 加密密钥（必填，32位） | - |
| `LOG_LEVEL` | 日志级别 | info |

---

## 项目结构

```
all-relay-service/
├── src/
│   ├── routes/          # API 路由
│   ├── middleware/       # 认证、限流等中间件
│   ├── services/        # 业务逻辑
│   │   ├── relay/       # 各平台转发
│   │   ├── account/     # 账户管理
│   │   └── scheduler/   # 调度器
│   ├── models/          # 数据模型
│   └── utils/           # 工具函数
├── config/              # 配置文件
├── web/admin-spa/       # 管理后台前端
├── cli/                 # 命令行工具
└── scripts/             # 运维脚本
```

---

## 开发

```bash
# 开发模式（热重载）
npm run dev

# 代码检查
npm run lint

# 格式化
npm run format

# 运行测试
npm test
```

---

## 许可证

MIT
