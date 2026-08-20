#!/bin/bash
set -e

# ══════════════════════════════════════════════════════════
# MP-CORE Panel Installer
# ══════════════════════════════════════════════════════════

# ⚠️ IMPORTANT: Change this to YOUR GitHub username
REPO_URL="https://github.com/crypt0mp73/mp-core.git"

GREEN='\033[0;92m'; RED='\033[0;91m'; CYAN='\033[0;96m'; YELLOW='\033[1;33m'; NC='\033[0m'

echo -e "${CYAN}═══════════════════════════════════════${NC}"
echo -e "${CYAN}        MP-CORE Panel Installer        ${NC}"
echo -e "${CYAN}═══════════════════════════════════════${NC}"
echo ""

# ── Root check ────────────────────────────────────────────
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}✗ Root privileges required. Run with sudo.${NC}"
    exit 1
fi

# ── Step 1: Network reachability / ping test ──────────────
echo -e "${CYAN}» Testing server network reachability...${NC}"
SERVER_IP=$(curl -4 -fsSL --max-time 5 ifconfig.me 2>/dev/null || \
            curl -4 -fsSL --max-time 5 icanhazip.com 2>/dev/null || echo "")

if [ -z "$SERVER_IP" ]; then
    echo -e "${YELLOW}! Could not determine public IP, continuing anyway...${NC}"
else
    echo -e "${GREEN}✓ Server IP detected: ${SERVER_IP}${NC}"
    echo -e "${CYAN}» Running global ping test...${NC}"
    PING_RESULT=$(curl -fsSL -H "Accept: application/json" \
        "https://check-host.net/check-ping?host=${SERVER_IP}&node=us1.node.check-host.net&node=de1.node.check-host.net" \
        --max-time 10 2>/dev/null || echo "")
    if [ -n "$PING_RESULT" ]; then
        REQ_ID=$(echo "$PING_RESULT" | grep -o '"request_id":"[^"]*"' | cut -d'"' -f4 || echo "")
        if [ -n "$REQ_ID" ]; then
            sleep 6
            echo -e "${GREEN}✓ Ping test completed (request: ${REQ_ID})${NC}"
        else
            echo -e "${YELLOW}! Ping test inconclusive, continuing...${NC}"
        fi
    else
        echo -e "${YELLOW}! Ping test skipped (API unreachable), continuing...${NC}"
    fi
fi

echo ""
echo -e "${CYAN}Do you want to install the MP-CORE panel? (y/n)${NC}"
read -r INSTALL_CONFIRM
if [[ ! "$INSTALL_CONFIRM" =~ ^[Yy]$ ]]; then
    echo -e "${RED}Installation cancelled.${NC}"
    exit 0
fi

# ── Step 2: Gather settings ───────────────────────────────
echo ""
read -rp "Panel port [2053]: " PANEL_PORT
PANEL_PORT=${PANEL_PORT:-2053}

read -rp "Admin username [admin]: " ADMIN_USER
ADMIN_USER=${ADMIN_USER:-admin}

read -rsp "Admin password [admin]: " ADMIN_PASS
echo ""
ADMIN_PASS=${ADMIN_PASS:-admin}

# ── Step 3: Install dependencies (git, curl, Go) ─────────
echo -e "${CYAN}» Checking dependencies...${NC}"

if ! command -v git >/dev/null 2>&1; then
    echo -e "${YELLOW}» Installing git...${NC}"
    apt-get update -y >/dev/null 2>&1 || true
    apt-get install -y git >/dev/null 2>&1
fi

if ! command -v curl >/dev/null 2>&1; then
    apt-get install -y curl >/dev/null 2>&1
fi

if ! command -v go >/dev/null 2>&1; then
    echo -e "${YELLOW}» Installing Go...${NC}"
    GO_VERSION="1.22.5"
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc
fi

export PATH=$PATH:/usr/local/go/bin
echo -e "${GREEN}✓ Go $(go version | awk '{print $3}') ready${NC}"

# ── Step 4: Clone and build ───────────────────────────────
echo -e "${CYAN}» Downloading MP-CORE source...${NC}"
BUILD_DIR="/tmp/mp-core-build"
rm -rf "$BUILD_DIR"
git clone "$REPO_URL" "$BUILD_DIR" 2>/dev/null

cd "$BUILD_DIR"
echo -e "${CYAN}» Building MP-CORE...${NC}"
go mod tidy
CGO_ENABLED=0 go build -o /usr/local/mp-core/mp-core .
echo -e "${GREEN}✓ Build complete${NC}"

# ── Step 5: Systemd service ───────────────────────────────
mkdir -p /etc/mp-core
cat > /etc/systemd/system/mp-core.service <<EOF
[Unit]
Description=MP-CORE Panel
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/mp-core/mp-core
Environment=MP_CORE_PORT=${PANEL_PORT}
Environment=MP_CORE_DB_PATH=/etc/mp-core/mp-core.db
Environment=MP_CORE_ADMIN_USER=${ADMIN_USER}
Environment=MP_CORE_ADMIN_PASS=${ADMIN_PASS}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable mp-core >/dev/null 2>&1
systemctl restart mp-core

# ── Cleanup ───────────────────────────────────────────────
rm -rf "$BUILD_DIR"

sleep 2
if systemctl is-active --quiet mp-core; then
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "${GREEN}  ✓ MP-CORE installed successfully!    ${NC}"
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "  Panel URL: ${CYAN}http://${SERVER_IP:-YOUR_IP}:${PANEL_PORT}${NC}"
    echo -e "  Username:  ${CYAN}${ADMIN_USER}${NC}"
    echo -e "  Password:  ${CYAN}${ADMIN_PASS}${NC}"
    echo ""
else
    echo -e "${RED}✗ Service failed to start. Check: journalctl -u mp-core${NC}"
    exit 1
fi
