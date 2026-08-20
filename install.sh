#!/bin/bash
set -e

# ══════════════════════════════════════════════════════════
# MP-CORE Panel Installer
# Downloads pre-built binary - works on 512MB RAM VPS!
# ══════════════════════════════════════════════════════════

GITHUB_USER="crypt0mp73"
GITHUB_REPO="mp-core"
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

# ── Install minimal dependencies ─────────────────────────
if ! command -v curl >/dev/null 2>&1; then
    apt-get update -y >/dev/null 2>&1 || true
    apt-get install -y curl >/dev/null 2>&1
fi
if ! command -v python3 >/dev/null 2>&1; then
    apt-get update -y >/dev/null 2>&1 || true
    apt-get install -y python3 >/dev/null 2>&1
fi

# ── Step 1: Detect server IP ─────────────────────────────
echo -e "${CYAN}» Testing server network reachability...${NC}"
SERVER_IP=$(curl -4 -fsSL --max-time 5 ifconfig.me 2>/dev/null || \
            curl -4 -fsSL --max-time 5 icanhazip.com 2>/dev/null || echo "")

if [ -z "$SERVER_IP" ]; then
    echo -e "${YELLOW}! Could not determine public IP, skipping ping test${NC}"
else
    echo -e "${GREEN}✓ Server IP detected: ${SERVER_IP}${NC}"
    echo -e "${CYAN}» Running global ping test...${NC}"

    python3 - "$SERVER_IP" <<'PYEOF'
import sys, json, time, urllib.request

ip = sys.argv[1]

def get(url):
    try:
        req = urllib.request.Request(url, headers={'Accept':'application/json','User-Agent':'mp-core-installer'})
        with urllib.request.urlopen(req, timeout=15) as r:
            return json.load(r)
    except Exception:
        return None

# Fetch node list
nodes_data = get("https://check-host.net/nodes/hosts") or {}
nodes = nodes_data.get('nodes', {})

ir_nodes = []
int_nodes = []
for key, val in nodes.items():
    loc = val.get('location', [])
    if not loc:
        continue
    if loc[0] == 'ir':
        city = loc[2] if len(loc) > 2 else 'Iran'
        ir_nodes.append((key, city))
    elif key in ('us1.node.check-host.net','de4.node.check-host.net','nl1.node.check-host.net'):
        country = loc[1] if len(loc) > 1 else 'Global'
        int_nodes.append((key, country))

if not ir_nodes:
    ir_nodes = [('ir1.node.check-host.net','Tehran'), ('ir2.node.check-host.net','Isfahan')]
if not int_nodes:
    int_nodes = [('us1.node.check-host.net','USA'), ('de4.node.check-host.net','Germany'), ('nl1.node.check-host.net','Netherlands')]

# Submit ping test
node_params = ''.join('&node=' + k for k, _ in (ir_nodes + int_nodes))
init = get("https://check-host.net/check-ping?host=" + ip + node_params)
if not init or 'request_id' not in init:
    print("  ! Could not connect to check API")
    sys.exit(0)

req_id = init['request_id']
print("  Waiting for results (~10s)...")
time.sleep(10)

result = get("https://check-host.net/check-result/" + req_id) or {}

def show(nodes_list, header, is_iran):
    print()
    print("  " + header)
    print("  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌")
    for key, name in nodes_list:
        entries = result.get(key)
        ok = False
        ms = 0
        if entries and len(entries) > 0 and len(entries[0]) > 0:
            ping = entries[0][0]
            if isinstance(ping, list) and len(ping) >= 2 and ping[0] == 'OK':
                ok = True
                ms = round(ping[1] * 1000)
        prefix = ("Iran, " + name) if is_iran else name
        if ok:
            print("  ✓ " + prefix + ": ONLINE (" + str(ms) + " ms)")
        else:
            print("  ✗ " + prefix + ": TIMEOUT / BLOCKED")

show(ir_nodes, "DOMESTIC/IRAN NETWORKS", True)
show(int_nodes, "INTERNATIONAL NODES", False)
PYEOF
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

# ── Step 3: Detect architecture ──────────────────────────
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

# ── Step 4: Download pre-built binary ────────────────────
echo -e "${CYAN}» Downloading latest release from GitHub...${NC}"
mkdir -p /usr/local/mp-core

RELEASE_INFO=$(curl -fsSL "$RELEASES_API" 2>/dev/null || echo "")
if [ -z "$RELEASE_INFO" ]; then
    echo -e "${RED}✗ Could not fetch release info from GitHub${NC}"
    exit 1
fi

DOWNLOAD_URL=$(echo "$RELEASE_INFO" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for asset in data.get('assets', []):
    if asset['name'] == '${BINARY_NAME}':
        print(asset['browser_download_url'])
        break
" 2>/dev/null)

if [ -z "$DOWNLOAD_URL" ]; then
    echo -e "${RED}✗ Could not find binary '${BINARY_NAME}' in the release${NC}"
    echo -e "${YELLOW}  You must build & upload the binary first (see guide).${NC}"
    exit 1
fi

VERSION=$(echo "$RELEASE_INFO" | python3 -c "import sys,json; print(json.load(sys.stdin).get('tag_name','unknown'))" 2>/dev/null)
echo -e "${CYAN}  Found version: ${VERSION}${NC}"

curl -fsSL -o /usr/local/mp-core/mp-core "$DOWNLOAD_URL"
chmod +x /usr/local/mp-core/mp-core

if [ -s /usr/local/mp-core/mp-core ]; then
    SIZE=$(du -h /usr/local/mp-core/mp-core | cut -f1)
    echo -e "${GREEN}✓ Downloaded ${BINARY_NAME} (${SIZE})${NC}"
else
    echo -e "${RED}✗ Download failed${NC}"
    exit 1
fi

# ── Step 5: Systemd service ──────────────────────────────
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
