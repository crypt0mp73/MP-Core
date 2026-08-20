#!/bin/bash
set -e

# ══════════════════════════════════════════════════════════
# MP-CORE Panel Installer
# Downloads pre-built binary - works on 512MB VPS!
# ══════════════════════════════════════════════════════════

# ⚠️ IMPORTANT: Change this to YOUR GitHub username
GITHUB_USER="crypt0mp73"
GITHUB_REPO="mp-core"
REPO_RAW="https://raw.githubusercontent.com/${GITHUB_USER}/${GITHUB_REPO}/main"
RELEASES_API="https://api.github.com/repos/${GITHUB_USER}/${GITHUB_REPO}/releases/latest"

GREEN='\033[0;92m'; RED='\033[0;91m'; CYAN='\033[0;96m'; YELLOW='\033[1;33m'; NC='\033[0m'

echo -e "${CYAN}═══════════════════════════════════════${NC}"
echo -e "${CYAN}        MP-CORE Panel Installer        ${NC}"
echo -e "${CYAN}   Works on 512MB RAM VPS • No compile${NC}"
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
    echo -e "${YELLOW}! Could not determine public IP, skipping ping test${NC}"
else
    echo -e "${GREEN}✓ Server IP detected: ${SERVER_IP}${NC}"
    echo ""
    echo -e "${CYAN}» Running global ping test...${NC}"
    
    PING_RESULT=$(curl -fsSL -H "Accept: application/json" \
        "https://check-host.net/check-ping?host=${SERVER_IP}&node=ir1.node.check-host.net&node=ir2.node.check-host.net&node=us1.node.check-host.net&node=de1.node.check-host.net&node=nl1.node.check-host.net" \
        --max-time 15 2>/dev/null || echo "")
    
    if [ -z "$PING_RESULT" ]; then
        echo -e "${YELLOW}! Ping test API unreachable, skipping...${NC}"
    else
        REQ_ID=$(echo "$PING_RESULT" | grep -o '"request_id":"[^"]*"' | cut -d'"' -f4 || echo "")
        
        if [ -n "$REQ_ID" ]; then
            echo -e "${CYAN}  Waiting for results (10s)...${NC}"
            sleep 10
            
            RESULTS=$(curl -fsSL -H "Accept: application/json" \
                "https://check-host.net/check-result/${REQ_ID}" \
                --max-time 15 2>/dev/null || echo "")
            
            if [ -n "$RESULTS" ]; then
                echo ""
                echo -e "${CYAN}  DOMESTIC/IRAN NETWORKS${NC}"
                echo -e "  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌"
                
                echo "$RESULTS" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    nodes = data.get('nodes', {})
    iran_results = []
    intl_results = []
    
    for node, results in nodes.items():
        if results and len(results) > 0:
            r = results[0][0] if results[0] else None
            if r and isinstance(r, list) and len(r) >= 2:
                if node.startswith('ir'):
                    iran_results.append({'node': node, 'time': r[1]})
                else:
                    intl_results.append({'node': node, 'time': r[1]})
    
    for r in iran_results:
        node_name = {'ir1': 'Iran, Tehran', 'ir2': 'Iran, Isfahan'}.get(r['node'], r['node'])
        print(f\"  ✓ {node_name}: ONLINE ({r['time']} ms)\")
    if not iran_results:
        print(f\"  ✗ No Iran nodes responded\")
    
    print()
    print(f\"  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌\")
    print(f\"  INTERNATIONAL NODES\")
    print(f\"  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌\")
    
    for r in intl_results:
        node_name = {'us1': 'USA', 'de1': 'Germany', 'nl1': 'Netherlands'}.get(r['node'], r['node'])
        print(f\"  ✓ {node_name}: ONLINE ({r['time']} ms)\")
    if not intl_results:
        print(f\"  ✗ No international nodes responded\")
    
except Exception as e:
    print(f\"  ! Could not parse results\")
" 2>/dev/null || echo -e "  ${YELLOW}! Could not parse ping results${NC}"
                echo ""
            fi
        fi
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

# ── Step 3: Install minimal dependencies (no Go needed!) ─
echo -e "${CYAN}» Checking dependencies...${NC}"

install_pkg() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo -e "${YELLOW}» Installing $1...${NC}"
        if command -v apt-get >/dev/null 2>&1; then
            apt-get update -y >/dev/null 2>&1 || true
            apt-get install -y "$2" >/dev/null 2>&1
        elif command -v yum >/dev/null 2>&1; then
            yum install -y "$2" >/dev/null 2>&1
        elif command -v dnf >/dev/null 2>&1; then
            dnf install -y "$2" >/dev/null 2>&1
        fi
    fi
}

install_pkg "curl" "curl"
install_pkg "python3" "python3"

echo -e "${GREEN}✓ Dependencies ready${NC}"

# ── Step 4: Detect architecture ──────────────────────────
echo -e "${CYAN}» Detecting architecture...${NC}"
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        BINARY_NAME="mp-core-amd64"
        echo -e "${GREEN}✓ Detected: x86_64 (amd64)${NC}"
        ;;
    aarch64|arm64)
        BINARY_NAME="mp-core-arm64"
        echo -e "${GREEN}✓ Detected: ARM64${NC}"
        ;;
    *)
        echo -e "${RED}✗ Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

# ── Step 5: Download pre-built binary ────────────────────
echo -e "${CYAN}» Downloading latest release from GitHub...${NC}"

mkdir -p /usr/local/mp-core

# Get latest release info
RELEASE_INFO=$(curl -fsSL "$RELEASES_API" 2>/dev/null || echo "")

if [ -z "$RELEASE_INFO" ]; then
    echo -e "${RED}✗ Could not fetch release info from GitHub${NC}"
    echo -e "${YELLOW}  Have you created a release yet?${NC}"
    echo -e "${YELLOW}  Run: git tag v1.0.0 && git push origin v1.0.0${NC}"
    exit 1
fi

# Find the download URL for our architecture
DOWNLOAD_URL=$(echo "$RELEASE_INFO" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for asset in data.get('assets', []):
    if asset['name'] == '${BINARY_NAME}':
        print(asset['browser_download_url'])
        break
" 2>/dev/null)

if [ -z "$DOWNLOAD_URL" ]; then
    echo -e "${RED}✗ Could not find binary for ${BINARY_NAME} in release${NC}"
    exit 1
fi

VERSION=$(echo "$RELEASE_INFO" | python3 -c "import sys,json; print(json.load(sys.stdin).get('tag_name','unknown'))" 2>/dev/null)
echo -e "${CYAN}  Found version: ${VERSION}${NC}"

# Download binary
curl -fsSL -o /usr/local/mp-core/mp-core "$DOWNLOAD_URL"
chmod +x /usr/local/mp-core/mp-core

if [ -f /usr/local/mp-core/mp-core ]; then
    SIZE=$(du -h /usr/local/mp-core/mp-core | cut -f1)
    echo -e "${GREEN}✓ Downloaded ${BINARY_NAME} (${SIZE})${NC}"
else
    echo -e "${RED}✗ Download failed${NC}"
    exit 1
fi

# ── Step 6: Systemd service ──────────────────────────────
echo -e "${CYAN}» Setting up systemd service...${NC}"
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
MemoryMax=256M

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable mp-core >/dev/null 2>&1
systemctl restart mp-core

sleep 2
if systemctl is-active --quiet mp-core; then
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "${GREEN}  ✓ MP-CORE installed successfully!    ${NC}"
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "  Panel URL: ${CYAN}http://${SERVER_IP:-YOUR_IP}:${PANEL_PORT}${NC}"
    echo -e "  Username:  ${CYAN}${ADMIN_USER}${NC}"
    echo -e "  Password:  ${CYAN}${ADMIN_PASS}${NC}"
    echo -e "  Version:   ${CYAN}${VERSION}${NC}"
    echo -e "  RAM usage: ${CYAN}~30MB (works on 512MB VPS!)${NC}"
    echo ""
else
    echo -e "${RED}✗ Service failed to start. Check: journalctl -u mp-core${NC}"
    exit 1
fi
