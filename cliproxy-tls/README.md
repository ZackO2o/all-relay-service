# CLIProxy-TLS — uTLS Claude API Proxy

Go 微服务，为 Claude API 请求提供 Chrome JA3 TLS 指纹伪装 + 设备指纹注入，绕过 Cloudflare 的机器人检测。

## 架构

```
All-Relay-Service (Node.js) → CLAUDE_TLS_PROXY → cliproxy-tls (Go) → api.anthropic.com
                                                                        ↑
                                                              uTLS + Chrome JA3
                                                              + X-Stainless-* headers
                                                              + HTTP/2 connection pool
```

## 编译

```bash
cd cliproxy-tls
go build -o cliproxy-tls .
```

## 部署

### systemd

```bash
cp cliproxy-tls /opt/cliproxy-tls/
cp server.crt server.key /opt/cliproxy-tls/

cat > /etc/systemd/system/cliproxy-tls.service << 'SVC'
[Unit]
Description=CLIProxy TLS - uTLS-powered Claude API proxy
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/cliproxy-tls
ExecStart=/opt/cliproxy-tls/cliproxy-tls
Restart=always
RestartSec=5
Environment=PORT=9200
Environment=GO_ENV=production

[Install]
WantedBy=multi-user.target
SVC

systemctl daemon-reload
systemctl enable --now cliproxy-tls
```

### 配置 All-Relay-Service

在 `.env` 中添加：

```env
CLAUDE_TLS_PROXY=https://localhost:9200
NODE_TLS_REJECT_UNAUTHORIZED=0
```

重启 All-Relay-Service 后，所有 Claude API 请求自动走 uTLS 代理。

## API

| 路由 | 方法 | 说明 |
|------|------|------|
| `/v1/messages` | POST | 转发到 Anthropic Messages API |
| `/v1/models` | GET | 模型列表 |
| `/oauth/authorize` | GET | OAuth 授权 URL |
| `/oauth/callback` | POST | 交换 Token |
| `/oauth/refresh` | POST | 刷新 Token |
| `/health` | GET | 健康检查 |

## 项目结构

```
cliproxy-tls/
├── main.go              # HTTP 服务入口
├── utls/transport.go    # uTLS Chrome JA3 传输层
├── profile/device.go    # 设备指纹注入 (X-Stainless-*)
├── proxy/claude.go      # Claude API 代理转发
├── oauth/oauth.go       # OAuth PKCE 流程
├── go.mod / go.sum
├── server.crt / server.key  # 自签名证书
└── README.md
```
