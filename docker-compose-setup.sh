#!/bin/bash
# =============================================
#  ALL Relay Service - Docker 一键部署脚本
#  https://github.com/ZackO2o/all-relay-service
# =============================================

set -e

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;36m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'
print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_ok()   { echo -e "${GREEN}[OK]${NC} $1"; }
print_warn() { echo -e "${YELLOW}[!]${NC} $1"; }
print_err()  { echo -e "${RED}[x]${NC} $1"; }

# 检查 Docker
if ! command -v docker &>/dev/null; then
    print_err "请先安装 Docker: https://docs.docker.com/engine/install/"
    exit 1
fi

if ! command -v docker-compose &>/dev/null && ! docker compose version &>/dev/null 2>&1; then
    print_err "请先安装 Docker Compose"
    exit 1
fi

COMPOSE_CMD="docker-compose"
docker compose version &>/dev/null 2>&1 && COMPOSE_CMD="docker compose"

print_info "正在生成 docker-compose.yml..."

# 生成密钥
JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || date +%s%N | md5sum | head -c 32)
ENC_KEY=$(openssl rand -base64 24 2>/dev/null || date +%s%N | md5sum | head -c 32)

# 创建 docker-compose.yml
cat > docker-compose.yml << EOF
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: all-relay-redis
    restart: always
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes

  app:
    image: ghcr.io/zacko2o/all-relay-service:latest
    container_name: all-relay-service
    restart: always
    ports:
      - "3000:3000"
    depends_on:
      - redis
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - ENCRYPTION_KEY=${ENC_KEY}
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=
      - LOG_LEVEL=info
      - PORT=3000
      - NODE_ENV=production
    volumes:
      - app_data:/app/data
      - app_logs:/app/logs

volumes:
  redis_data:
  app_data:
  app_logs:
EOF

print_ok "docker-compose.yml 已生成"
print_info "启动服务..."
$COMPOSE_CMD up -d

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║${NC}  ${BOLD}🎉 ALL Relay Service Docker 部署完成！${NC}${GREEN}║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════╝${NC}"
echo ""
echo -e "  管理后台: ${BLUE}http://localhost:3000/admin-next/login${NC}"
echo ""
echo -e "  管理员账号: $COMPOSE_CMD logs app | grep '管理员'"
echo ""

# 等几秒后显示管理员信息
sleep 5
$COMPOSE_CMD logs app 2>/dev/null | grep -E "管理员|admin|username|password|init" | head -5 || print_info "查看管理员信息: $COMPOSE_CMD logs app"
