package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	cm01Port            = 19000
	cm01HeartbeatPort   = 19001
	cm01HeartbeatPeriod = time.Second
	cm01OfflineTimeout  = 3 * time.Second
	cm01TypeCode        = 0xDC01
)

type cm01HeartbeatPacket struct {
	Type string `json:"type"`
	Dev  string `json:"dev"`
}

type cm01BatchRecord struct {
	Channel string  `json:"ch"`
	TS      uint64  `json:"ts"`
	Current float32 `json:"cur"`
}

type cm01DataBatchPacket struct {
	Type    string            `json:"type"`
	Seq     uint32            `json:"seq"`
	Records []cm01BatchRecord `json:"records"`
}

type cm01Session struct {
	ip           string
	mac          [6]byte
	nodeKey      string
	lastSeen     time.Time
	heartbeating bool
	connected    bool
}

type DiscoveredNode struct {
	NodeKey   string  `json:"nodeKey"`
	Name      string  `json:"name"`
	TypeCode  uint16  `json:"typeCode"`
	TypeName  string  `json:"typeName"`
	IP        string  `json:"ip"`
	Mac       [6]byte `json:"mac"`
	Online    bool    `json:"online"`
	Connected bool    `json:"connected"`
	LastSeen  int64   `json:"lastSeen"`
}

type CM01Manager struct {
	mu       sync.Mutex
	conn     *net.UDPConn
	sessions map[string]*cm01Session
	paused   bool
	started  bool
}

var cm01Manager = &CM01Manager{sessions: make(map[string]*cm01Session)}

const cm01DebugLogPrefix = "[CM01_DEBUG]"

func StartCM01Manager() {
	cm01Manager.mu.Lock()
	if cm01Manager.started {
		cm01Manager.mu.Unlock()
		return
	}
	cm01Manager.started = true
	cm01Manager.mu.Unlock()
	go cm01Manager.run()
}

func StartExternalAcquisition() {
	StartCM01Manager()
	StartPSURCManager()
}

func PauseExternalAcquisition() {
	cm01Manager.SetPaused(true)
	psurcManager.SetPaused(true)
}

func ResumeExternalAcquisition() {
	cm01Manager.SetPaused(false)
	psurcManager.SetPaused(false)
}

func (m *CM01Manager) run() {
	addr := &net.UDPAddr{Port: cm01Port, IP: net.IPv4zero}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("cm01 listen udp failed:", err)
		return
	}
	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()
	go m.heartbeatLoop()
	buf := make([]byte, 64*1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("cm01 read udp failed:", err)
			continue
		}
		payload := append([]byte(nil), buf[:n]...)
		m.handlePacket(remote, payload)
	}
}

func (m *CM01Manager) SetPaused(paused bool) {
	m.mu.Lock()
	m.paused = paused
	m.mu.Unlock()
}

func (m *CM01Manager) isPaused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paused
}

func (m *CM01Manager) handlePacket(remote *net.UDPAddr, payload []byte) {
	if m.isPaused() {
		fmt.Printf("%s handlePacket skipped: paused remote=%s payload=%s\n", cm01DebugLogPrefix, remote.String(), string(payload))
		return
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		fmt.Printf("%s handlePacket json parse failed: remote=%s err=%v payload=%s\n", cm01DebugLogPrefix, remote.String(), err, string(payload))
		return
	}
	fmt.Printf("%s handlePacket received: remote=%s type=%s payload=%s\n", cm01DebugLogPrefix, remote.String(), envelope.Type, string(payload))
	switch envelope.Type {
	case "esp_heartbeat":
		var hb cm01HeartbeatPacket
		if err := json.Unmarshal(payload, &hb); err != nil {
			fmt.Printf("%s esp_heartbeat parse failed: remote=%s err=%v payload=%s\n", cm01DebugLogPrefix, remote.String(), err, string(payload))
			return
		}
		if hb.Dev != "DAS_NODE_CM01" {
			fmt.Printf("%s esp_heartbeat ignored: remote=%s dev=%s\n", cm01DebugLogPrefix, remote.String(), hb.Dev)
			return
		}
		session, created := m.touchSession(remote.IP.String())
		fmt.Printf("%s esp_heartbeat accepted: remoteIP=%s created=%v nodeKey=%s name=%s\n", cm01DebugLogPrefix, remote.IP.String(), created, session.nodeKey, "CM01-"+strings.ReplaceAll(session.ip, ".", "-"))
		if created && app != nil {
			fmt.Printf("%s esp_heartbeat notify frontend: remoteIP=%s nodeKey=%s\n", cm01DebugLogPrefix, remote.IP.String(), session.nodeKey)
			app.NewDataNotify()
		}
	case "data_batch":
		var batch cm01DataBatchPacket
		if err := json.Unmarshal(payload, &batch); err != nil {
			fmt.Printf("%s data_batch parse failed: remote=%s err=%v payload=%s\n", cm01DebugLogPrefix, remote.String(), err, string(payload))
			return
		}
		fmt.Printf("%s data_batch accepted: remoteIP=%s seq=%d records=%d\n", cm01DebugLogPrefix, remote.IP.String(), batch.Seq, len(batch.Records))
		m.handleBatch(remote.IP.String(), batch)
	default:
		fmt.Printf("%s handlePacket ignored unknown type: remote=%s type=%s payload=%s\n", cm01DebugLogPrefix, remote.String(), envelope.Type, string(payload))
	}
}

func (m *CM01Manager) touchSession(ip string) (*cm01Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[ip]
	created := false
	if !ok {
		mac := pseudoMacFromIP(ip)
		nodeKey := MacStr(mac)
		session = &cm01Session{ip: ip, mac: mac, nodeKey: nodeKey}
		m.sessions[ip] = session
		created = true
	}
	session.lastSeen = time.Now()
	name := "CM01-" + strings.ReplaceAll(session.ip, ".", "-")
	fmt.Printf("%s touchSession: ip=%s created=%v nodeKey=%s name=%s sessions=%d lastSeen=%d\n", cm01DebugLogPrefix, ip, created, session.nodeKey, name, len(m.sessions), session.lastSeen.UnixMilli())
	return session, created
}

func (m *CM01Manager) SnapshotDiscoveredNodes() map[string]DiscoveredNode {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]DiscoveredNode, len(m.sessions))
	now := time.Now()
	for _, session := range m.sessions {
		result[session.nodeKey] = DiscoveredNode{
			NodeKey:   session.nodeKey,
			Name:      "CM01-" + strings.ReplaceAll(session.ip, ".", "-"),
			TypeCode:  cm01TypeCode,
			TypeName:  NodeTypeMap[cm01TypeCode].Name,
			IP:        session.ip,
			Mac:       session.mac,
			Online:    now.Sub(session.lastSeen) < cm01OfflineTimeout,
			Connected: session.connected,
			LastSeen:  session.lastSeen.UnixMilli(),
		}
	}
	return result
}

func (m *CM01Manager) ConnectNode(nodeKey string) error {
	m.mu.Lock()
	var session *cm01Session
	for _, candidate := range m.sessions {
		if candidate.nodeKey == nodeKey {
			session = candidate
			break
		}
	}
	if session == nil {
		m.mu.Unlock()
		return errors.New("node not found")
	}
	if time.Since(session.lastSeen) >= cm01OfflineTimeout {
		m.markOfflineLocked(session)
		m.mu.Unlock()
		app.NewDataNotify()
		return errors.New("node offline")
	}
	conn := m.conn
	initialSeen := session.lastSeen
	ip := session.ip
	mac := session.mac
	m.mu.Unlock()

	if conn == nil {
		m.forceOfflineByNodeKey(nodeKey)
		return errors.New("udp not ready")
	}

	addr := &net.UDPAddr{IP: net.ParseIP(ip), Port: cm01Port}
	if addr.IP == nil {
		m.forceOfflineByNodeKey(nodeKey)
		return errors.New("invalid ip")
	}
	if _, err := conn.WriteToUDP([]byte(`{"type":"heartbeat"}`), &net.UDPAddr{IP: addr.IP, Port: cm01HeartbeatPort}); err != nil {
		m.forceOfflineByNodeKey(nodeKey)
		return err
	}

	time.Sleep(1200 * time.Millisecond)

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, candidate := range m.sessions {
		if candidate.nodeKey != nodeKey {
			continue
		}
		if !candidate.lastSeen.After(initialSeen) {
			m.markOfflineLocked(candidate)
			go app.NewDataNotify()
			return errors.New("connect failed, node offline")
		}
		candidate.connected = true
		if simulating {
			m.mu.Unlock()
			StopSim()
			m.mu.Lock()
		}
		nodesMu.Lock()
		if _, exists := nodes[nodeKey]; !exists {
			node := &Node{}
			node.InitFrom(ScanData{Type: cm01TypeCode, Tick: 0, Mac: mac})
			node.Source = "external"
			node.Name = "CM01-" + strings.ReplaceAll(ip, ".", "-")
			node.TypeName = NodeTypeMap[cm01TypeCode].Name
			nodes[nodeKey] = node
		}
		nodesMu.Unlock()
		go app.NewDataNotify()
		return nil
	}
	return errors.New("node offline")
}

func (m *CM01Manager) markOfflineLocked(session *cm01Session) {
	delete(m.sessions, session.ip)
	nodesMu.Lock()
	delete(nodes, session.nodeKey)
	nodesMu.Unlock()
}

func (m *CM01Manager) forceOfflineByNodeKey(nodeKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, session := range m.sessions {
		if session.nodeKey == nodeKey {
			m.markOfflineLocked(session)
			break
		}
	}
	go app.NewDataNotify()
}

func (m *CM01Manager) handleBatch(ip string, batch cm01DataBatchPacket) {
	session, _ := m.touchSession(ip)
	if session == nil || !session.connected {
		return
	}
	recordsByTS := make(map[uint64]map[string]float32)
	var latestBattery float32
	var hasBattery bool
	for _, record := range batch.Records {
		frame, ok := recordsByTS[record.TS]
		if !ok {
			frame = make(map[string]float32)
			recordsByTS[record.TS] = frame
		}
		switch record.Channel {
		case "ADS_VOLTAGE":
			frame["voltage"] = record.Current
		case "ADS_CURRENT":
			frame["current"] = record.Current
		case "BATT_ADC":
			latestBattery = record.Current
			hasBattery = true
		}
	}
	nodesMu.RLock()
	node := nodes[session.nodeKey]
	nodesMu.RUnlock()
	if node == nil {
		return
	}
	filtered := make(map[uint64]map[string]float32)
	keys := make([]uint64, 0, len(recordsByTS))
	for ts := range recordsByTS {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, ts := range keys {
		frame := recordsByTS[ts]
		if _, ok := frame["voltage"]; !ok {
			continue
		}
		if _, ok := frame["current"]; !ok {
			continue
		}
		filtered[ts] = frame
	}
	hostNow := time.Now().UnixMilli()
	updated := node.ApplyMetricRecords(filtered, hostNow)
	if hasBattery {
		node.Battery = batteryPercentFromVoltage(latestBattery)
	}
	node.RSSI = 0
	if updated || hasBattery {
		app.NewDataNotify()
	}
}

func (m *CM01Manager) heartbeatLoop() {
	ticker := time.NewTicker(cm01HeartbeatPeriod)
	defer ticker.Stop()
	for range ticker.C {
		if m.isPaused() {
			continue
		}
		m.cleanupOffline()
		m.sendHeartbeats()
	}
}

func (m *CM01Manager) cleanupOffline() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	changed := false
	for ip, session := range m.sessions {
		if now.Sub(session.lastSeen) < cm01OfflineTimeout {
			continue
		}
		delete(m.sessions, ip)
		nodesMu.Lock()
		delete(nodes, session.nodeKey)
		nodesMu.Unlock()
		changed = true
	}
	if changed {
		app.NewDataNotify()
	}
}

func (m *CM01Manager) sendHeartbeats() {
	m.mu.Lock()
	conn := m.conn
	sessions := make([]*cm01Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		if !session.connected {
			continue
		}
		sessions = append(sessions, session)
	}
	m.mu.Unlock()
	if conn == nil {
		return
	}
	payload := []byte(`{"type":"heartbeat"}`)
	for _, session := range sessions {
		addr := &net.UDPAddr{IP: net.ParseIP(session.ip), Port: cm01HeartbeatPort}
		if addr.IP == nil {
			continue
		}
		_, _ = conn.WriteToUDP(payload, addr)
	}
}

func pseudoMacFromIP(ip string) [6]byte {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return [6]byte{0xDC, 0x01, 0, 0, 0, 0}
	}
	v4 := parsed.To4()
	if v4 == nil {
		return [6]byte{0xDC, 0x01, 0, 0, 0, 1}
	}
	return [6]byte{0xDC, 0x01, v4[0], v4[1], v4[2], v4[3]}
}

func batteryPercentFromVoltage(v float32) int {
	if v <= 3.2 {
		return 0
	}
	if v >= 4.2 {
		return 100
	}
	segments := []struct {
		voltage float64
		percent float64
	}{
		{4.20, 100},
		{4.10, 90},
		{4.00, 80},
		{3.92, 70},
		{3.87, 60},
		{3.82, 50},
		{3.79, 40},
		{3.77, 30},
		{3.74, 20},
		{3.68, 10},
		{3.20, 0},
	}
	vf := float64(v)
	for i := 0; i < len(segments)-1; i++ {
		high := segments[i]
		low := segments[i+1]
		if vf <= high.voltage && vf >= low.voltage {
			ratio := (vf - low.voltage) / (high.voltage - low.voltage)
			pct := low.percent + ratio*(high.percent-low.percent)
			return int(math.Round(pct))
		}
	}
	return 0
}
