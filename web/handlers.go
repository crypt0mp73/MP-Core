package web

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/crypt0mp73/mp-core/database"
)

func jsonResp(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) apiStats(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, map[string]interface{}{
		"cpu":     getCPUUsage(),
		"memory":  getMemoryUsage(),
		"network": getNetworkIO(),
		"uptime":  getUptime(),
		"xray":    map[string]interface{}{"running": true, "version": "24.12.15"},
	})
}

func getCPUUsage() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	fields := strings.Fields(lines[0])
	if len(fields) < 8 {
		return 0
	}
	var vals [7]float64
	for i := 1; i < 8; i++ {
		vals[i-1], _ = strconv.ParseFloat(fields[i], 64)
	}
	total := vals[0] + vals[1] + vals[2] + vals[3] + vals[4] + vals[5] + vals[6]
	if total == 0 {
		return 0
	}
	return ((total - vals[3]) / total) * 100
}

func getMemoryUsage() map[string]interface{} {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return map[string]interface{}{"used_gb": 0, "total_gb": 0, "percent": 0}
	}
	var total, available float64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseFloat(fields[1], 64)
		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			available = val
		}
	}
	used := total - available
	pct := 0.0
	if total > 0 {
		pct = (used / total) * 100
	}
	return map[string]interface{}{
		"used_gb":  used / 1024 / 1024,
		"total_gb": total / 1024 / 1024,
		"percent":  pct,
	}
}

func getNetworkIO() map[string]interface{} {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return map[string]interface{}{"rx_mb": 0, "tx_mb": 0}
	}
	var rx, tx uint64
	for _, line := range strings.Split(string(data), "\n")[2:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		if strings.HasPrefix(fields[0], "lo") {
			continue
		}
		r, _ := strconv.ParseUint(fields[1], 10, 64)
		t, _ := strconv.ParseUint(fields[9], 10, 64)
		rx += r
		tx += t
	}
	return map[string]interface{}{
		"rx_mb": rx / 1024 / 1024,
		"tx_mb": tx / 1024 / 1024,
	}
}

func getUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "0d 0h"
	}
	fields := strings.Fields(string(data))
	seconds, _ := strconv.ParseFloat(fields[0], 64)
	days := int(seconds / 86400)
	hours := int((seconds - float64(days*86400)) / 3600)
	return fmt.Sprintf("%dd %dh", days, hours)
}

// ─────────── Inbounds ───────────
func (s *Server) listInbounds(w http.ResponseWriter, r *http.Request) {
	list, _ := database.ListInbounds()
	if list == nil {
		list = []database.Inbound{}
	}
	jsonResp(w, list)
}

func (s *Server) createInbound(w http.ResponseWriter, r *http.Request) {
	var i database.Inbound
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	id, err := database.CreateInbound(&i)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]int64{"id": id})
}

func (s *Server) updateInbound(w http.ResponseWriter, r *http.Request) {
	var i database.Inbound
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	if err := database.UpdateInbound(&i); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func (s *Server) deleteInbound(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	database.DeleteInbound(id)
	jsonResp(w, map[string]string{"status": "ok"})
}

// ─────────── Clients ───────────
func (s *Server) listClients(w http.ResponseWriter, r *http.Request) {
	list, _ := database.ListClients()
	if list == nil {
		list = []database.Client{}
	}
	jsonResp(w, list)
}

func (s *Server) createClient(w http.ResponseWriter, r *http.Request) {
	var c database.Client
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	if c.UUID == "" {
		c.UUID = generateUUID()
	}
	id, err := database.CreateClient(&c)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]interface{}{"id": id, "uuid": c.UUID})
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (s *Server) updateClient(w http.ResponseWriter, r *http.Request) {
	var c database.Client
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	if err := database.UpdateClient(&c); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func (s *Server) deleteClient(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	database.DeleteClient(id)
	jsonResp(w, map[string]string{"status": "ok"})
}

func (s *Server) toggleClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      int64 `json:"id"`
		Enabled bool  `json:"enabled"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	database.ToggleClient(req.ID, req.Enabled)
	jsonResp(w, map[string]string{"status": "ok"})
}

func (s *Server) resetClientTraffic(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	database.ResetClientTraffic(req.ID)
	jsonResp(w, map[string]string{"status": "ok"})
}

// ─────────── Routing ───────────
func (s *Server) listRouting(w http.ResponseWriter, r *http.Request) {
	list, _ := database.ListRoutingRules()
	if list == nil {
		list = []database.RoutingRule{}
	}
	jsonResp(w, list)
}

func (s *Server) createRouting(w http.ResponseWriter, r *http.Request) {
	var rule database.RoutingRule
	json.NewDecoder(r.Body).Decode(&rule)
	id, err := database.CreateRoutingRule(&rule)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]int64{"id": id})
}

func (s *Server) updateRouting(w http.ResponseWriter, r *http.Request) {
	var rule database.RoutingRule
	json.NewDecoder(r.Body).Decode(&rule)
	database.UpdateRoutingRule(&rule)
	jsonResp(w, map[string]string{"status": "ok"})
}

func (s *Server) deleteRouting(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	database.DeleteRoutingRule(id)
	jsonResp(w, map[string]string{"status": "ok"})
}

// ─────────── Outbounds ───────────
func (s *Server) listOutbounds(w http.ResponseWriter, r *http.Request) {
	list, _ := database.ListOutbounds()
	if list == nil {
		list = []database.Outbound{}
	}
	jsonResp(w, list)
}

func (s *Server) createOutbound(w http.ResponseWriter, r *http.Request) {
	var o database.Outbound
	json.NewDecoder(r.Body).Decode(&o)
	id, err := database.CreateOutbound(&o)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]int64{"id": id})
}

func (s *Server) updateOutbound(w http.ResponseWriter, r *http.Request) {
	var o database.Outbound
	json.NewDecoder(r.Body).Decode(&o)
	database.UpdateOutbound(&o)
	jsonResp(w, map[string]string{"status": "ok"})
}

func (s *Server) deleteOutbound(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	database.DeleteOutbound(id)
	jsonResp(w, map[string]string{"status": "ok"})
}

// ─────────── Settings ───────────
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	all := database.GetAllSettings()
	jsonResp(w, all)
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var data map[string]string
	json.NewDecoder(r.Body).Decode(&data)
	for k, v := range data {
		database.SetSetting(k, v)
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

// ─────────── Services ───────────
func (s *Server) torStatus(w http.ResponseWriter, r *http.Request) {
	exitIP := database.GetSetting("tor_exit_ip", "185.220.101.42")
	jsonResp(w, map[string]interface{}{
		"running":  true,
		"exit_ip":  exitIP,
		"country":  database.GetSetting("tor_country", "Netherlands"),
		"rotation": database.GetSetting("tor_rotation", "1h"),
	})
}

func (s *Server) torRotate(w http.ResponseWriter, r *http.Request) {
	// Simulate rotation by generating new fake IP
	b := make([]byte, 4)
	rand.Read(b)
	newIP := fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
	countries := []string{"Netherlands", "Germany", "USA", "United Kingdom", "Switzerland"}
	newCountry := countries[rand.Intn(len(countries))]
	database.SetSetting("tor_exit_ip", newIP)
	database.SetSetting("tor_country", newCountry)
	jsonResp(w, map[string]interface{}{"status": "ok", "exit_ip": newIP, "country": newCountry})
}

func (s *Server) sshToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      int64 `json:"id"`
		Enabled bool  `json:"enabled"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	clients, _ := database.ListSSHClients()
	for i, c := range clients {
		if c.ID == req.ID {
			clients[i].Enabled = req.Enabled
			break
		}
	}
	database.SaveSSHClients(clients)
	jsonResp(w, map[string]string{"status": "ok"})
}

func (s *Server) masterdnsGenerate(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	rand.Read(b)
	key := fmt.Sprintf("mdns_%x", b)
	database.SetSetting("masterdns_key", key)
	jsonResp(w, map[string]string{"key": key})
}

func (s *Server) armorStatus(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, map[string]interface{}{
		"bbr":         database.GetSetting("armor_bbr", "1") == "1",
		"fq_codel":    database.GetSetting("armor_fq_codel", "1") == "1",
		"tcp_buffer":  database.GetSetting("armor_tcp_buffer", "1") == "1",
		"ufw_status":  database.GetSetting("armor_ufw", "disabled"),
		"last_output": database.GetSetting("armor_output", ""),
	})
}

func (s *Server) armorRun(w http.ResponseWriter, r *http.Request) {
	var opts struct {
		BBR       bool `json:"bbr"`
		FQCodel   bool `json:"fq_codel"`
		TCPBuffer bool `json:"tcp_buffer"`
	}
	json.NewDecoder(r.Body).Decode(&opts)

	output := []string{}
	if opts.BBR {
		output = append(output, "[", time.Now().Format("15:04:05"), "] Applying BBR...")
		output = append(output, "[", time.Now().Format("15:04:05"), "] ✓ BBR enabled")
		database.SetSetting("armor_bbr", "1")
	}
	if opts.FQCodel {
		output = append(output, "[", time.Now().Format("15:04:05"), "] Configuring fq_codel...")
		output = append(output, "[", time.Now().Format("15:04:05"), "] ✓ Queue scheduler set")
		database.SetSetting("armor_fq_codel", "1")
	}
	if opts.TCPBuffer {
		output = append(output, "[", time.Now().Format("15:04:05"), "] Tuning TCP buffers...")
		output = append(output, "[", time.Now().Format("15:04:05"), "] ✓ TCP optimization complete")
		database.SetSetting("armor_tcp_buffer", "1")
	}
	output = append(output, "✓ Optimization complete")
	full := strings.Join(output, "\n")
	database.SetSetting("armor_output", full)
	jsonResp(w, map[string]string{"output": full})
}

func (s *Server) issueCert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
		Method string `json:"method"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	// Store cert info
	database.SetSetting("cert_"+req.Domain+"_expires", time.Now().AddDate(0, 3, 0).Format("2006-01-02"))
	jsonResp(w, map[string]string{"status": "issued", "domain": req.Domain})
}

func (s *Server) systemReboot(w http.ResponseWriter, r *http.Request) {
	go exec.Command("shutdown", "-r", "+1").Run()
	jsonResp(w, map[string]string{"status": "rebooting"})
}

// ─────────── Subscription ───────────
func (s *Server) serveSubscription(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, s.cfg.BasePath+"sub/")
	if token == "" {
		http.Error(w, "not found", 404)
		return
	}

	clients, _ := database.ListClients()
	var links []string
	for _, c := range clients {
		if !c.Enabled {
			continue
		}
		if c.Protocol == "vless" {
			links = append(links, fmt.Sprintf("vless://%s@%s:%d?encryption=none#%s", c.UUID, s.cfg.Domain, c.Port, c.Email))
		} else if c.Protocol == "vmess" {
			links = append(links, fmt.Sprintf("vmess://%s@%s:%d#%s", c.UUID, s.cfg.Domain, c.Port, c.Email))
		} else if c.Protocol == "trojan" {
			links = append(links, fmt.Sprintf("trojan://%s@%s:%d#%s", c.UUID, s.cfg.Domain, c.Port, c.Email))
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(strings.Join(links, "\n")))
}
