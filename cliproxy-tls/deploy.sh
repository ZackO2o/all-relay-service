#!/bin/bash
# =============================================
#  cliproxy-tls 部署脚本
#  在目标服务器上快速部署 uTLS Go 代理
# =============================================
set -e

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;36m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'
print_info()  { echo -e "${BLUE}[INFO]${NC} $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
print_warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
print_err()   { echo -e "${RED}[x]${NC} $1"; }

# ----- 检查 Go -----
check_go() {
    if command -v go &>/dev/null; then
        print_ok "Go $(go version | grep -oP 'go\S+') 已安装"
        return 0
    fi
    print_info "安装 Go..."
    curl -fsSL https://go.dev/dl/go1.22.5.linux-amd64.tar.gz -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    print_ok "Go $(go version | grep -oP 'go\S+') 安装完成"
}

# ----- 生成自签名证书 -----
gen_cert() {
    local dir="$1"
    if [ -f "$dir/server.crt" ] && [ -f "$dir/server.key" ]; then
        print_ok "证书已存在，跳过生成"
        return
    fi
    print_info "生成自签名 TLS 证书..."
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
      -keyout "$dir/server.key" -out "$dir/server.crt" \
      -subj "/CN=localhost" 2>/dev/null
    print_ok "证书已生成: $dir/server.crt, $dir/server.key"
}

# ----- 从 GitHub 拉取代码 -----
clone_or_update() {
    local dir="$1"
    if [ -d "$dir/.git" ]; then
        print_info "更新代码..."
        cd "$dir" && git pull --ff-only
    else
        print_info "克隆代码..."
        git clone --depth 1 https://github.com/ZackO2o/all-relay-service.git "$dir"
    fi
}

# ----- 编译 -----
build() {
    local dir="$1/cliproxy-tls"
    cd "$dir"
    print_info "编译 Go 二进制..."
    GOPROXY=https://goproxy.cn,direct go build -o cliproxy-tls .
    print_ok "编译完成: $dir/cliproxy-tls"
}

# ----- 创建 systemd 服务 -----
install_systemd() {
    local dir="$1/cliproxy-tls"
    local service_file="/etc/systemd/system/cliproxy-tls.service"

    cat > "$service_file" << EOF
[Unit]
Description=CLIProxy TLS - uTLS-powered Claude API proxy
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$dir
ExecStart=$dir/cliproxy-tls
Restart=always
RestartSec=5
Environment=PORT=9200
Environment=GO_ENV=production

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable cliproxy-tls 2>/dev/null
    systemctl restart cliproxy-tls
    print_ok "Systemd 服务已创建并启动"
}

# ----- 测试 -----
test_service() {
    sleep 2
    if curl -sk https://localhost:9200/health 2>/dev/null; then
        echo ""
        print_ok "服务运行正常！"
    else
        print_warn "健康检查失败，查看日志: journalctl -u cliproxy-tls -n 50"
    fi

    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}  ${BOLD}🎉 cliproxy-tls 部署完成！${NC}              ${GREEN}║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  ${BOLD}代理地址:${NC} ${BLUE}https://localhost:9200${NC}"
    echo ""
    echo -e "  ${BOLD}在 All-Relay-Service 的 .env 中添加:${NC}"
    echo -e "  CLAUDE_TLS_PROXY=https://localhost:9200"
    echo -e "  NODE_TLS_REJECT_UNAUTHORIZED=0"
    echo ""
    echo -e "  ${BOLD}管理命令:${NC}"
    echo -e "  systemctl status cliproxy-tls    查看状态"
    echo -e "  systemctl restart cliproxy-tls   重启"
    echo -e "  journalctl -u cliproxy-tls -f    查看日志"
    echo ""
}

# ----- 主流程 -----
main() {
    local install_dir="${1:-/opt/all-relay-service}"

    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}  ${BOLD}cliproxy-tls 一键部署${NC}                  ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  uTLS Chrome JA3 Claude API Proxy       ${BLUE}║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════╝${NC}"
    echo ""

    check_go
    clone_or_update "$install_dir"
    gen_cert "$install_dir/cliproxy-tls"
    build "$install_dir"
    install_systemd "$install_dir"
    test_service
}

main "$@"
