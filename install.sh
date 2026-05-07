#!/bin/bash
# =============================================
#  ALL Relay Service - 一键安装脚本
#  https://github.com/ZackO2o/all-relay-service
# =============================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

print_info()  { echo -e "${BLUE}[INFO]${NC} $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
print_warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
print_err()   { echo -e "${RED}[x]${NC} $1"; }

# ----- 检测系统 -----
detect_pkg_manager() {
    if command -v apt-get &>/dev/null; then
        PKG="apt-get"
        PKG_INSTALL="apt-get install -y"
    elif command -v yum &>/dev/null; then
        PKG="yum"
        PKG_INSTALL="yum install -y"
    elif command -v dnf &>/dev/null; then
        PKG="dnf"
        PKG_INSTALL="dnf install -y"
    elif command -v pacman &>/dev/null; then
        PKG="pacman"
        PKG_INSTALL="pacman -S --noconfirm"
    elif command -v brew &>/dev/null; then
        PKG="brew"
        PKG_INSTALL="brew install"
    else
        print_err "无法识别系统包管理器，请手动安装 Node.js 18+ 和 Redis 后重试"
        exit 1
    fi
}

# ----- 检查命令 -----
check_cmd() { command -v "$1" &>/dev/null; }

# ----- banner -----
print_banner() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}  ${BOLD}ALL Relay Service 一键安装${NC}            ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  全平台 AI API 中转服务                  ${BLUE}║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════╝${NC}"
    echo ""
}

# =============================================
#  安装 Node.js
# =============================================
install_nodejs() {
    if check_cmd node; then
        local ver=$(node -v | sed 's/v//' | cut -d. -f1)
        if [ "$ver" -ge 18 ] 2>/dev/null; then
            print_ok "Node.js $(node -v) 已安装"
            return 0
        fi
        print_warn "Node.js 版本过低 ($(node -v))，需要 18+"
    fi

    print_info "安装 Node.js 18+..."
    if [ "$PKG" = "brew" ]; then
        brew install node@18
    elif [ "$PKG" = "pacman" ]; then
        pacman -S --noconfirm nodejs npm
    else
        curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
        $PKG_INSTALL nodejs
    fi

    if check_cmd node; then
        print_ok "Node.js $(node -v) 安装成功"
    else
        print_err "Node.js 安装失败，请手动安装"
        exit 1
    fi
}

# =============================================
#  安装 Redis
# =============================================
install_redis() {
    if check_cmd redis-server || check_cmd redis-cli; then
        print_ok "Redis 已安装"
        return 0
    fi

    print_info "安装 Redis..."
    if [ "$PKG" = "brew" ]; then
        brew install redis
        brew services start redis
    elif [ "$PKG" = "pacman" ]; then
        pacman -S --noconfirm redis
        systemctl start redis 2>/dev/null || redis-server --daemonize yes
    else
        $PKG_INSTALL redis-server redis
        systemctl start redis 2>/dev/null || service redis-server start 2>/dev/null || redis-server --daemonize yes
    fi

    if check_cmd redis-cli; then
        print_ok "Redis 已安装并启动"
    else
        print_warn "Redis 安装后未检测到，请手动安装"
    fi
}

# =============================================
#  安装 ALL Relay Service
# =============================================
install_service() {
    local install_dir="${1:-$HOME/all-relay-service}"

    # 克隆项目
    if [ -d "$install_dir" ]; then
        print_warn "目录 $install_dir 已存在"
        echo -n "是否覆盖？(y/N): "
        read -r overwrite
        if [ "$overwrite" = "y" ] || [ "$overwrite" = "Y" ]; then
            rm -rf "$install_dir"
        else
            print_info "使用已有目录"
        fi
    fi

    if [ ! -d "$install_dir" ]; then
        print_info "下载 ALL Relay Service..."
        git clone https://github.com/ZackO2o/all-relay-service.git "$install_dir"
        print_ok "下载完成"
    fi

    cd "$install_dir"

    # 安装依赖
    print_info "安装 npm 依赖..."
    npm install --production
    print_ok "依赖安装完成"

    # 复制配置
    if [ ! -f config/config.js ]; then
        cp config/config.example.js config/config.js
    fi
    if [ ! -f .env ]; then
        cp .env.example .env
    fi

    # 生成密钥
    local jwt_secret=$(openssl rand -base64 32 2>/dev/null || date +%s%N | md5sum | head -c 32)
    local enc_key=$(openssl rand -base64 24 2>/dev/null || date +%s%N | md5sum | head -c 32)

    # 写入 .env
    cat > .env << EOF
# ALL Relay Service 配置
JWT_SECRET=$jwt_secret
ENCRYPTION_KEY=$enc_key
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
LOG_LEVEL=info
PORT=3000
EOF
    print_ok "配置文件已生成"

    # 创建 systemd 服务
    cat > /etc/systemd/system/all-relay.service << 'EOF' 2>/dev/null || true
[Unit]
Description=ALL Relay Service
After=network.target redis-server.service

[Service]
Type=simple
User=root
WorkingDirectory=__INSTALL_DIR__
ExecStart=/usr/bin/node __INSTALL_DIR__/src/app.js
Restart=always
RestartSec=10
EnvironmentFile=__INSTALL_DIR__/.env

[Install]
WantedBy=multi-user.target
EOF

    local systemd_file="/etc/systemd/system/all-relay.service"
    if [ -f "$systemd_file" ]; then
        sed -i "s|__INSTALL_DIR__|$install_dir|g" "$systemd_file"
        systemctl daemon-reload
        systemctl enable all-relay 2>/dev/null
        print_ok "Systemd 服务已创建"
    fi

    # 设置快捷命令
    local alias_cmd="alias ars='bash $install_dir/scripts/manage.sh'"
    if [ -f "$HOME/.bashrc" ]; then
        if ! grep -q "alias ars=" "$HOME/.bashrc"; then
            echo "$alias_cmd" >> "$HOME/.bashrc"
        fi
    fi
    if [ -f "$HOME/.zshrc" ]; then
        if ! grep -q "alias ars=" "$HOME/.zshrc"; then
            echo "$alias_cmd" >> "$HOME/.zshrc"
        fi
    fi
}

# =============================================
#  启动服务
# =============================================
start_service() {
    local install_dir="${1:-$HOME/all-relay-service}"

    print_info "初始化系统..."
    cd "$install_dir"
    node scripts/setup.js 2>/dev/null || true

    # 优先使用 systemd
    if systemctl start all-relay 2>/dev/null; then
        print_ok "服务已通过 systemd 启动"
    else
        # fallback: 直接启动
        print_info "使用 PM2 启动..."
        if ! check_cmd pm2; then
            npm install -g pm2 2>/dev/null
        fi
        pm2 start "$install_dir/src/app.js" --name "all-relay" --log "$install_dir/logs/pm2.log" 2>/dev/null || true
    fi

    # 等待启动
    sleep 3
}

# =============================================
#  显示结果
# =============================================
show_result() {
    local install_dir="${1:-$HOME/all-relay-service}"
    local port="${2:-3000}"

    # 获取 IP
    local ip=""
    ip=$(curl -s --connect-timeout 3 https://api.ipify.org 2>/dev/null || \
         curl -s --connect-timeout 3 https://ifconfig.me 2>/dev/null || \
         hostname -I 2>/dev/null | awk '{print $1}')

    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}  ${BOLD}🎉 ALL Relay Service 安装完成！${NC}       ${GREEN}║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  ${BOLD}访问地址：${NC}"
    echo -e "  本地管理后台: ${BLUE}http://localhost:${port}/admin-next/login${NC}"
    [ -n "$ip" ] && echo -e "  公网管理后台: ${BLUE}http://${ip}:${port}/admin-next/login${NC}"
    echo ""
    echo -e "  ${BOLD}管理命令：${NC}"
    echo -e "  ars start     启动服务"
    echo -e "  ars stop      停止服务"
    echo -e "  ars restart   重启服务"
    echo -e "  ars status    查看状态"
    echo -e "  ars update    更新服务"
    echo -e "  ars logs      查看日志"
    echo ""
    echo -e "  ${YELLOW}管理员账号已保存到: ${install_dir}/data/init.json${NC}"
    echo -e "  ${YELLOW}如未生效，请执行: source ~/.bashrc${NC}"
    echo ""

    # 显示 init.json 中的账号
    if [ -f "$install_dir/data/init.json" ]; then
        echo -e "  ${BOLD}管理员凭据：${NC}"
        cat "$install_dir/data/init.json" | python3 -m json.tool 2>/dev/null || cat "$install_dir/data/init.json"
        echo ""
    fi
}

# =============================================
#  主流程
# =============================================
main() {
    print_banner

    detect_pkg_manager

    install_nodejs
    install_redis

    local install_dir="${1:-$HOME/all-relay-service}"
    local port="${2:-3000}"

    install_service "$install_dir"
    start_service "$install_dir"
    show_result "$install_dir" "$port"
}

# 参数：$1=安装目录, $2=端口
main "$@"
