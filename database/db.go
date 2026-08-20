package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB

func Init(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create db directory: %w", err)
		}
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	DB.SetMaxOpenConns(5)
	DB.SetMaxIdleConns(2)
	DB.SetConnMaxLifetime(time.Hour)

	if err := DB.Ping(); err != nil {
		return err
	}

	return migrate()
}

func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE TABLE IF NOT EXISTS inbounds (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		remark          TEXT,
		port            INTEGER,
		protocol        TEXT,
		transport       TEXT DEFAULT 'tcp',
		security        TEXT DEFAULT 'none',
		settings        TEXT DEFAULT '{}',
		stream_settings TEXT DEFAULT '{}',
		enabled         INTEGER DEFAULT 1,
		traffic_total   INTEGER DEFAULT 0,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS clients (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		inbound_id  INTEGER,
		email       TEXT,
		uuid        TEXT,
		flow        TEXT DEFAULT '',
		enabled     INTEGER DEFAULT 1,
		expiry_time INTEGER DEFAULT 0,
		total_gb    INTEGER DEFAULT 0,
		up          INTEGER DEFAULT 0,
		down        INTEGER DEFAULT 0,
		ip_limit    INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (inbound_id) REFERENCES inbounds(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS routing_rules (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT,
		rule_type   TEXT,
		matcher     TEXT,
		outbound    TEXT,
		enabled     INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS outbounds (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		tag      TEXT UNIQUE,
		protocol TEXT,
		settings TEXT DEFAULT '{}',
		is_default INTEGER DEFAULT 0
	);
	`
	_, err := DB.Exec(schema)
	return err
}

func SeedAdmin(username, password string) error {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count > 0 {
		return nil
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	_, err := DB.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
	return err
}

func SeedDefaults() error {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM outbounds").Scan(&count)
	if count > 0 {
		return nil
	}

	defaults := []struct{ tag, protocol string }{
		{"direct", "freedom"},
		{"blocked", "blackhole"},
		{"warp", "wireguard"},
		{"tor-out", "socks"},
	}
	for _, d := range defaults {
		settings := "{}"
		if d.protocol == "freedom" {
			settings = `{"domainStrategy":"AsIs"}`
		}
		isDefault := 0
		if d.tag == "direct" {
			isDefault = 1
		}
		DB.Exec("INSERT INTO outbounds (tag, protocol, settings, is_default) VALUES (?,?,?,?)", d.tag, d.protocol, settings, isDefault)
	}

	rules := []struct{ name, typ, matcher, outbound string }{
		{"Block Bittorrent", "field", `{"protocol":["bittorrent"]}`, "blocked"},
		{"Block Private IPs", "field", `{"ip":["geoip:private"]}`, "blocked"},
	}
	for _, r := range rules {
		DB.Exec("INSERT INTO routing_rules (name, rule_type, matcher, outbound) VALUES (?,?,?,?)", r.name, r.typ, r.matcher, r.outbound)
	}

	return nil
}

func CheckLogin(username, password string) (bool, error) {
	var hash string
	err := DB.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil, nil
}

// ─────────── Settings ───────────
func GetSetting(key, defaultVal string) string {
	var v string
	err := DB.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&v)
	if err != nil {
		return defaultVal
	}
	return v
}

func SetSetting(key, value string) error {
	_, err := DB.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?,?)", key, value)
	return err
}

func GetAllSettings() map[string]string {
	rows, err := DB.Query("SELECT key, value FROM settings")
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		m[k] = v
	}
	return m
}

// ─────────── Inbounds ───────────
type Inbound struct {
	ID             int64  `json:"id"`
	Remark         string `json:"remark"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"`
	Transport      string `json:"transport"`
	Security       string `json:"security"`
	Settings       string `json:"settings"`
	StreamSettings string `json:"stream_settings"`
	Enabled        bool   `json:"enabled"`
	TrafficTotal   int64  `json:"traffic_total"`
	ClientCount    int    `json:"client_count,omitempty"`
}

func ListInbounds() ([]Inbound, error) {
	rows, err := DB.Query(`
		SELECT i.id, i.remark, i.port, i.protocol, i.transport, i.security, i.settings, i.stream_settings, i.enabled, i.traffic_total,
			(SELECT COUNT(*) FROM clients c WHERE c.inbound_id = i.id) as client_count
		FROM inbounds i ORDER BY i.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Inbound
	for rows.Next() {
		var i Inbound
		rows.Scan(&i.ID, &i.Remark, &i.Port, &i.Protocol, &i.Transport, &i.Security, &i.Settings, &i.StreamSettings, &i.Enabled, &i.TrafficTotal, &i.ClientCount)
		list = append(list, i)
	}
	return list, nil
}

func GetInbound(id int64) (*Inbound, error) {
	var i Inbound
	err := DB.QueryRow(`SELECT id, remark, port, protocol, transport, security, settings, stream_settings, enabled, traffic_total
		FROM inbounds WHERE id=?`, id).Scan(&i.ID, &i.Remark, &i.Port, &i.Protocol, &i.Transport, &i.Security, &i.Settings, &i.StreamSettings, &i.Enabled, &i.TrafficTotal)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func CreateInbound(i *Inbound) (int64, error) {
	res, err := DB.Exec(`INSERT INTO inbounds (remark, port, protocol, transport, security, settings, stream_settings, enabled)
		VALUES (?,?,?,?,?,?,?,?)`, i.Remark, i.Port, i.Protocol, i.Transport, i.Security, i.Settings, i.StreamSettings, i.Enabled)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateInbound(i *Inbound) error {
	_, err := DB.Exec(`UPDATE inbounds SET remark=?, port=?, protocol=?, transport=?, security=?, settings=?, stream_settings=?, enabled=? WHERE id=?`,
		i.Remark, i.Port, i.Protocol, i.Transport, i.Security, i.Settings, i.StreamSettings, i.Enabled, i.ID)
	return err
}

func DeleteInbound(id int64) error {
	DB.Exec("DELETE FROM clients WHERE inbound_id=?", id)
	_, err := DB.Exec("DELETE FROM inbounds WHERE id=?", id)
	return err
}

// ─────────── Clients ───────────
type Client struct {
	ID         int64  `json:"id"`
	InboundID  int64  `json:"inbound_id"`
	Inbound    string `json:"inbound_remark,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int    `json:"port,omitempty"`
	Email      string `json:"email"`
	UUID       string `json:"uuid"`
	Flow       string `json:"flow"`
	Enabled    bool   `json:"enabled"`
	ExpiryTime int64  `json:"expiry_time"`
	TotalGB    int64  `json:"total_gb"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	IPLimit    int    `json:"ip_limit"`
	CreatedAt  string `json:"created_at"`
}

func ListClients() ([]Client, error) {
	rows, err := DB.Query(`
		SELECT c.id, c.inbound_id, COALESCE(i.remark,''), COALESCE(i.protocol,''), COALESCE(i.port,0),
			c.email, c.uuid, c.flow, c.enabled, c.expiry_time, c.total_gb, c.up, c.down, c.ip_limit, c.created_at
		FROM clients c LEFT JOIN inbounds i ON c.inbound_id = i.id ORDER BY c.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Client
	for rows.Next() {
		var c Client
		rows.Scan(&c.ID, &c.InboundID, &c.Inbound, &c.Protocol, &c.Port,
			&c.Email, &c.UUID, &c.Flow, &c.Enabled, &c.ExpiryTime, &c.TotalGB, &c.Up, &c.Down, &c.IPLimit, &c.CreatedAt)
		list = append(list, c)
	}
	return list, nil
}

func CreateClient(c *Client) (int64, error) {
	res, err := DB.Exec(`INSERT INTO clients (inbound_id, email, uuid, flow, enabled, expiry_time, total_gb, ip_limit)
		VALUES (?,?,?,?,?,?,?,?)`, c.InboundID, c.Email, c.UUID, c.Flow, c.Enabled, c.ExpiryTime, c.TotalGB, c.IPLimit)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateClient(c *Client) error {
	_, err := DB.Exec(`UPDATE clients SET inbound_id=?, email=?, uuid=?, flow=?, enabled=?, expiry_time=?, total_gb=?, ip_limit=? WHERE id=?`,
		c.InboundID, c.Email, c.UUID, c.Flow, c.Enabled, c.ExpiryTime, c.TotalGB, c.IPLimit, c.ID)
	return err
}

func DeleteClient(id int64) error {
	_, err := DB.Exec("DELETE FROM clients WHERE id=?", id)
	return err
}

func ToggleClient(id int64, enabled bool) error {
	_, err := DB.Exec("UPDATE clients SET enabled=? WHERE id=?", enabled, id)
	return err
}

func ResetClientTraffic(id int64) error {
	_, err := DB.Exec("UPDATE clients SET up=0, down=0 WHERE id=?", id)
	return err
}

// ─────────── Routing ───────────
type RoutingRule struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	RuleType string `json:"rule_type"`
	Matcher  string `json:"matcher"`
	Outbound string `json:"outbound"`
	Enabled  bool   `json:"enabled"`
}

func ListRoutingRules() ([]RoutingRule, error) {
	rows, err := DB.Query("SELECT id, name, rule_type, matcher, outbound, enabled FROM routing_rules ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RoutingRule
	for rows.Next() {
		var r RoutingRule
		rows.Scan(&r.ID, &r.Name, &r.RuleType, &r.Matcher, &r.Outbound, &r.Enabled)
		list = append(list, r)
	}
	return list, nil
}

func CreateRoutingRule(r *RoutingRule) (int64, error) {
	res, err := DB.Exec("INSERT INTO routing_rules (name, rule_type, matcher, outbound, enabled) VALUES (?,?,?,?,?)",
		r.Name, r.RuleType, r.Matcher, r.Outbound, r.Enabled)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateRoutingRule(r *RoutingRule) error {
	_, err := DB.Exec("UPDATE routing_rules SET name=?, rule_type=?, matcher=?, outbound=?, enabled=? WHERE id=?",
		r.Name, r.RuleType, r.Matcher, r.Outbound, r.Enabled, r.ID)
	return err
}

func DeleteRoutingRule(id int64) error {
	_, err := DB.Exec("DELETE FROM routing_rules WHERE id=?", id)
	return err
}

// ─────────── Outbounds ───────────
type Outbound struct {
	ID        int64  `json:"id"`
	Tag       string `json:"tag"`
	Protocol  string `json:"protocol"`
	Settings  string `json:"settings"`
	IsDefault bool   `json:"is_default"`
}

func ListOutbounds() ([]Outbound, error) {
	rows, err := DB.Query("SELECT id, tag, protocol, settings, is_default FROM outbounds ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Outbound
	for rows.Next() {
		var o Outbound
		rows.Scan(&o.ID, &o.Tag, &o.Protocol, &o.Settings, &o.IsDefault)
		list = append(list, o)
	}
	return list, nil
}

func CreateOutbound(o *Outbound) (int64, error) {
	res, err := DB.Exec("INSERT INTO outbounds (tag, protocol, settings, is_default) VALUES (?,?,?,?)",
		o.Tag, o.Protocol, o.Settings, o.IsDefault)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateOutbound(o *Outbound) error {
	_, err := DB.Exec("UPDATE outbounds SET tag=?, protocol=?, settings=?, is_default=? WHERE id=?",
		o.Tag, o.Protocol, o.Settings, o.IsDefault, o.ID)
	return err
}

func DeleteOutbound(id int64) error {
	_, err := DB.Exec("DELETE FROM outbounds WHERE id=?", id)
	return err
}

// ─────────── SSH Clients ───────────
type SSHClient struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port     int    `json:"port"`
	Enabled  bool   `json:"enabled"`
	Expiry   int64  `json:"expiry"`
}

func ListSSHClients() ([]SSHClient, error) {
	rows, err := DB.Query("SELECT id, username, password, port, enabled, expiry FROM settings WHERE key LIKE 'ssh_client_%'")
	_ = rows
	// Using settings table with JSON for simplicity
	raw := GetSetting("ssh_clients", "[]")
	var list []SSHClient
	json.Unmarshal([]byte(raw), &list)
	return list, nil
}

func SaveSSHClients(list []SSHClient) error {
	data, _ := json.Marshal(list)
	return SetSetting("ssh_clients", string(data))
}
