# ALL Relay Service — Developer Guide

## 项目概述

ALL Relay Service — 多平台 AI API 中转服务，作为客户端与上游 AI API 之间的中间件。
支持 OpenAI、Anthropic Claude、Google Gemini、国产模型（DeepSeek、Qwen 等）等多种账户类型。
核心能力：多账户管理、API Key 认证、统一调度、代理配置、限流、成本统计。

## 项目结构

```
src/
├── routes/              # HTTP 路由
│   ├── api.js           # Claude API 主路由
│   ├── admin/           # 管理后台路由
│   ├── openaiRoutes.js  # OpenAI 兼容路由
│   ├── geminiRoutes.js  # Gemini 路由
│   └── ...
├── middleware/           # 认证、限流等中间件
├── services/             # 业务逻辑
│   ├── relay/            # 各平台转发服务
│   ├── account/          # 各平台账户管理
│   └── scheduler/        # 统一调度器
├── utils/                # 工具函数
├── config/               # 配置
├── web/admin-spa/        # Vue3 管理后台
├── cli/                  # 命令行工具
└── scripts/              # 运维脚本
```

## 核心架构

### 请求流程

```
客户端 → API 路由 → 认证中间件 → 统一调度器(选账户)
  → Token 检查/刷新 → 转发服务 → 上游 API
  → 流式/非流式响应 → Usage 捕获 → 成本计算 → 返回
```

### 关键机制

- **粘性会话**: 基于请求内容 hash 绑定账户，同一会话用同一账户
- **并发控制**: Redis Sorted Set 实现，支持排队等待
- **模型别名**: 智能映射（`gpt-5.5-codex` → `gpt-5.5`）
- **加密存储**: 敏感信息（OAuth token、credentials）AES 加密存于 Redis

## 开发规范

### 代码风格

- **无分号**、**单引号**、**100字符行宽**
- 强制 `const`，严格相等 `===`
- 使用 Prettier 格式化

### 常用命令

```bash
npm install && npm run setup    # 初始化
npm run dev                     # 开发模式（nodemon 热重载）
npm start                       # 生产模式
npm run lint                    # ESLint 检查
npm run format                  # Prettier 格式化
npm test                        # 运行测试
```
