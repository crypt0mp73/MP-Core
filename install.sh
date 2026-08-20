#!/bin/bash
set -e

GITHUB_USER="crypt0mp73"
GITHUB_REPO="mp-core"
RELEASES_API="https://api.github.com/repos/${GITHUB_USER}/${GITHUB_REPO}/releases/latest"

GREEN='\033[0;92m'; RED='\033[0;91m'; CYAN='\033[0;96m'; YELLOW='\033[1;33m'; NC='\033[0m'

echo -e "${CYAN}═══════════════════════════════════════${NC}"
echo -e "${CYAN}        MP-CORE Panel Installer        ${NC}"
echo -e "${CYAN}   Works on 512MB RAM VPS • No compile${NC}"
echo -e "${CYAN}═══════════════════════════════════════${NC}"
echo ""

if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}✗ Root required${NC}"
    exit 1
fi

# Deps
for pkg in curl python3; do
    if ! command -v $pkg >/dev/null 2>&1; then
        apt-get update -y >/dev/null 2>&1 || true
        apt-get install -y $pkg >/dev/null 2>&1
    fi
done

# Ping test
SERVER_IP=$(curl -4 -fsSL --max-time 5 ifconfig.me 2>/dev/null || curl -4 -fsSL --max-time 5 icanhazip.com 2>/dev/null || echo "")
if [ -n "$SERVER_IP" ]; then
    echo -e "${GREEN}✓ Server IP: ${SERVER_IP}${NC}"
    echo -e "${CYAN}» Running ping test...${NC}"
    python3 - "$SERVER_IP" <<'PYEOF'
import sys,json,time,urllib.request
ip = sys.argv[1]
def get(url):
    try:
        req = urllib.request.Request(url, headers={'Accept':'application/json','User-Agent':'mp-core'})
        with urllib.request.urlopen(req, timeout=15) as r:
            return json.load(r)
    except: return None
nodes_data = get("https://check-host.net/nodes/hosts") or {}
nodes = nodes_data.get('nodes', {})
ir_nodes, int_nodes = [], []
for key, val in nodes.items():
    loc = val.get('location', [])
    if not loc: continue
    if loc[0]=='ir': ir_nodes.append((key, loc[2] if len(loc)>2 else 'Iran'))
    elif key in ('us1.node.check-host.net','de4.node.check-host.net','nl1.node.check-host.net'):
        int_nodes.append((key, loc[1] if len(loc)>1 else 'Global'))
if not ir_nodes: ir_nodes = [('ir1.node.check-host.net','Tehran'),('ir2.node.check-host.net','Isfahan')]
if not int_nodes: int_nodes = [('us1.node.check-host.net','USA'),('de4.node.check-host.net','Germany'),('nl1.node.check-host.net','Netherlands')]
params = ''.join('&node='+k for k,_ in (ir_nodes+int_nodes))
init = get("https://check-host.net/check-ping?host="+ip+params)
if not init or 'request_id' not in init:
    print("  ! API unreachable"); sys.exit(0)
print("  Waiting for results (~10s)...")
time.sleep(10)
result = get("https://check-host.net/check-result/"+init['request_id']) or {}
def show(lst, header, is_iran):
    print(); print("  "+header); print("  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌")
    for key, name in lst:
        entries = result.get(key)
        ok, ms = False, 0
        if entries and len(entries)>0 and len(entries[0])>0:
            ping = entries[0][0]
            if isinstance(ping, list) and len(ping)>=2 and ping[0]=='OK':
                ok, ms = True, round(ping[1]*1000)
        prefix = ("Iran, "+name) if is_iran else name
        if ok: print("  ✓ "+prefix+": ONLINE ("+str(ms)+" ms)")
        else: print("  ✗ "+prefix+": TIMEOUT")
show(ir_nodes, "DOMESTIC/IRAN NETWORKS", True)
show(int_nodes, "INTERNATIONAL NODES", False)
PYEOF
fi

echo ""
echo -e "${CYAN}Install MP-CORE? (y/n)${NC}"
read -r CONFIRM
if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
    echo "Cancelled."; exit 0
fi

read -rp "Panel port [2053]: " PANEL_PORT
PANEL_PORT=${PANEL_PORT:-2053}

read -rp "Domain (empty to skip HTTPS): " DOMAIN
DOMAIN=${DOMAIN:-""}

if [ -z "$DOMAIN" ]; then
    BASE_PATH="/mp-$(cat /dev/urandom | tr -dc 'a-z0-9' | fold -w 8 | head -n 1)/"
    echo -e "${YELLOW}» Secure base path: ${BASE_PATH}${NC}"
else
    read -rp "Base path [/${DOMAIN}/]: " BASE_PATH
    BASE_PATH=${BASE_PATH:-/${DOMAIN}/}
fi

read -rp "Admin username [admin]: " ADMIN_USER
ADMIN_USER=${ADMIN_USER:-admin}
read -rsp "Admin password [admin]: " ADMIN_PASS
echo ""
ADMIN_PASS=${ADMIN_PASS:-admin}

# Architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) BIN="mp-core-amd64" ;;
    aarch64|arm64) BIN="mp-core-arm64" ;;
    *) echo "Unsupported arch"; exit 1 ;;
esac

echo -e "${CYAN}» Downloading release...${NC}"
mkdir -p /usr/local/mp-core
RELEASE=$(curl -fsSL "$RELEASES_API")
URL=$(echo "$RELEASE" | python3 -c "import sys,json; d=json.load(sys.stdin); print([a['browser_download_url'] for a in d['assets'] if a['name']=='$BIN'][0])" 2>/dev/null)

if [ -z "$URL" ]; then
    echo -e "${RED}✗ Binary not found in release. Make sure you created a release tag.${NC}"
    exit 1
fi

VERSION=$(echo "$RELEASE" | python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])")
curl -fsSL -o /usr/local/mp-core/mp-core "$URL"
chmod +x /usr/local/mp-core/mp-core
echo -e "${GREEN}✓ Downloaded $BIN ($VERSION)${NC}"

# Service
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
Environment=MP_CORE_DOMAIN=${DOMAIN}
Environment=MP_CORE_BASE_PATH=${BASE_PATH}
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
    echo -e "${GREEN}  ✓ MP-CORE installed!                 ${NC}"
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "  URL: ${CYAN}http://${SERVER_IP:-IP}:${PANEL_PORT}${BASE_PATH}${NC}"
    echo -e "  User: ${CYAN}${ADMIN_USER}${NC}"
    echo -e "  Pass: ${CYAN}${ADMIN_PASS}${NC}"
    echo -e "  Ver:  ${CYAN}${VERSION}${NC}"
    echo ""
else
    echo -e "${RED}✗ Failed. journalctl -u mp-core${NC}"
fi
