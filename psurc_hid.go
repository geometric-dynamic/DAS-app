package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/karalabe/hid"
)

const (
	psurcVendorID       = 0x303A
	psurcProductID      = 0x4004
	psurcTypeCode       = 0xDC03
	psurcReportID       = 0x01
	psurcReportPayload  = 32
	psurcMagic          = 0xA5
	psurcDiscoverPeriod = time.Second
	psurcOfflineTimeout = 3 * time.Second
	psurcUdevErrorTag   = "PSURC_UDEV_PERMISSION"
	psurcWarmupDrop     = 3
)

type psurcSession struct {
	nodeKey   string
	name      string
	mac       [6]byte
	info      hid.DeviceInfo
	lastSeen  time.Time
	connected bool
	device    *hid.Device
}

type psurcFrame struct {
	tick       uint64
	nodeID     uint8
	ctrlOn     bool
	drvOn      bool
	isStop     bool
	rssiHint   int8
	supplyRMSV float32
	supplyMinV float32
	supplyMaxV float32
	ctrlRMSA   float32
	ctrlMinA   float32
	ctrlMaxA   float32
	drvRMSA    float32
	drvMinA    float32
	drvMaxA    float32
}

type PSURCManager struct {
	mu       sync.Mutex
	sessions map[string]*psurcSession
	paused   bool
	started  bool
}

var psurcManager = &PSURCManager{sessions: make(map[string]*psurcSession)}

func StartPSURCManager() {
	psurcManager.mu.Lock()
	if psurcManager.started {
		psurcManager.mu.Unlock()
		return
	}
	psurcManager.started = true
	psurcManager.mu.Unlock()
	go psurcManager.run()
}

func (m *PSURCManager) SetPaused(paused bool) {
	m.mu.Lock()
	m.paused = paused
	m.mu.Unlock()
}

func (m *PSURCManager) isPaused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paused
}

func (m *PSURCManager) run() {
	ticker := time.NewTicker(psurcDiscoverPeriod)
	defer ticker.Stop()
	m.scanDevices()
	for range ticker.C {
		if m.isPaused() {
			continue
		}
		m.scanDevices()
	}
}

func (m *PSURCManager) scanDevices() {
	infos := hid.Enumerate(psurcVendorID, psurcProductID)
	now := time.Now()
	present := make(map[string]hid.DeviceInfo, len(infos))
	for _, info := range infos {
		key := psurcNodeKey(info)
		present[key] = info
	}

	changed := false
	removedKeys := make([]string, 0)

	m.mu.Lock()
	for key, info := range present {
		session, ok := m.sessions[key]
		if !ok {
			session = &psurcSession{
				nodeKey: key,
				name:    psurcNodeName(key, info),
				mac:     pseudoMacFromString(key),
				info:    info,
			}
			m.sessions[key] = session
			changed = true
		}
		session.info = info
		session.name = psurcNodeName(key, info)
		session.lastSeen = now
	}

	for key, session := range m.sessions {
		if _, ok := present[key]; ok {
			continue
		}
		if now.Sub(session.lastSeen) < psurcOfflineTimeout {
			continue
		}
		if session.device != nil {
			_ = session.device.Close()
			session.device = nil
		}
		delete(m.sessions, key)
		removedKeys = append(removedKeys, key)
		changed = true
	}
	m.mu.Unlock()

	if len(removedKeys) > 0 {
		nodesMu.Lock()
		for _, key := range removedKeys {
			delete(nodes, key)
		}
		nodesMu.Unlock()
	}

	if changed && app != nil {
		app.NewDataNotify()
	}
}

func (m *PSURCManager) SnapshotDiscoveredNodes() map[string]DiscoveredNode {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]DiscoveredNode, len(m.sessions))
	now := time.Now()
	for _, session := range m.sessions {
		result[session.nodeKey] = DiscoveredNode{
			NodeKey:   session.nodeKey,
			Name:      session.name,
			TypeCode:  psurcTypeCode,
			TypeName:  NodeTypeMap[psurcTypeCode].Name,
			IP:        session.info.Path,
			Mac:       session.mac,
			Online:    now.Sub(session.lastSeen) < psurcOfflineTimeout,
			Connected: session.connected,
			LastSeen:  session.lastSeen.UnixMilli(),
		}
	}
	return result
}

func (m *PSURCManager) ConnectNode(nodeKey string) error {
	m.mu.Lock()
	session, ok := m.sessions[nodeKey]
	if !ok {
		m.mu.Unlock()
		return errors.New("node not found")
	}
	if time.Since(session.lastSeen) >= psurcOfflineTimeout {
		m.mu.Unlock()
		return errors.New("node offline")
	}
	if session.connected {
		m.mu.Unlock()
		return nil
	}
	info := session.info
	mac := session.mac
	name := session.name
	m.mu.Unlock()

	device, err := info.Open()
	if err != nil {
		fmt.Printf("[PSURC_DEBUG] connect failed: nodeKey=%s path=%s err=%v\n", nodeKey, info.Path, err)
		if runtime.GOOS == "linux" {
			return fmt.Errorf("%s: open failed (%v)\n%s", psurcUdevErrorTag, err, psurcUdevFixCommand())
		}
		return err
	}

	if simulating {
		StopSim()
	}

	nodesMu.Lock()
	if _, exists := nodes[nodeKey]; !exists {
		node := &Node{}
		node.InitFrom(ScanData{Type: psurcTypeCode, Tick: 0, Mac: mac})
		node.Source = "external"
		node.Name = name
		node.TypeName = NodeTypeMap[psurcTypeCode].Name
		node.Leds = psurcDeviceMarkerLeds(mac)
		nodes[nodeKey] = node
	}
	nodesMu.Unlock()

	m.mu.Lock()
	session, ok = m.sessions[nodeKey]
	if !ok {
		m.mu.Unlock()
		_ = device.Close()
		return errors.New("node offline")
	}
	session.connected = true
	session.device = device
	m.mu.Unlock()

	go m.readLoop(nodeKey, device)
	fmt.Printf("[PSURC_DEBUG] connected: nodeKey=%s path=%s\n", nodeKey, info.Path)

	if app != nil {
		app.NewDataNotify()
	}
	return nil
}

func (m *PSURCManager) readLoop(nodeKey string, device *hid.Device) {
	defer func() {
		_ = device.Close()
		m.markDisconnected(nodeKey)
		nodesMu.Lock()
		delete(nodes, nodeKey)
		nodesMu.Unlock()
		if app != nil {
			app.NewDataNotify()
		}
	}()

	buf := make([]byte, 64)
	warmupRemaining := psurcWarmupDrop
	for {
		n, err := device.Read(buf)
		if err != nil {
			fmt.Printf("[PSURC_DEBUG] read loop exit: nodeKey=%s err=%v\n", nodeKey, err)
			return
		}
		frame, err := parsePSURCReport(buf[:n])
		if err != nil {
			fmt.Printf("[PSURC_DEBUG] drop report: nodeKey=%s bytes=%d err=%v\n", nodeKey, n, err)
			continue
		}
		if warmupRemaining > 0 {
			warmupRemaining--
			fmt.Printf("[PSURC_DEBUG] warmup drop: nodeKey=%s tick=%d remain=%d\n", nodeKey, frame.tick, warmupRemaining)
			continue
		}
		values := map[string]float32{
			"supplyVoltageMin":  frame.supplyMinV,
			"supplyVoltageAvg":  frame.supplyRMSV,
			"supplyVoltageMax":  frame.supplyMaxV,
			"controlCurrentMin": frame.ctrlMinA,
			"controlCurrentAvg": frame.ctrlRMSA,
			"controlCurrentMax": frame.ctrlMaxA,
			"driverCurrentMin":  frame.drvMinA,
			"driverCurrentAvg":  frame.drvRMSA,
			"driverCurrentMax":  frame.drvMaxA,
		}
		if def, ok := NodeTypeMap[psurcTypeCode]; ok {
			for key, compute := range def.Compute {
				values[key] = compute(values)
			}
		}

		nodesMu.Lock()
		node, ok := nodes[nodeKey]
		if ok {
			ts := node.NormalizeTimestamp(frame.tick, time.Now().UnixMilli())
			node.AppendFrame(ts, values)
			node.RSSI = int(frame.rssiHint)
			node.LastDeviceTs = frame.tick
		}
		nodesMu.Unlock()
		if ok && app != nil {
			app.NewDataNotify()
		}
	}
}

func (m *PSURCManager) markDisconnected(nodeKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session, ok := m.sessions[nodeKey]; ok {
		session.connected = false
		session.device = nil
	}
}

func parsePSURCReport(report []byte) (*psurcFrame, error) {
	if len(report) < psurcReportPayload+1 {
		return nil, errors.New("short report")
	}
	if report[0] != psurcReportID {
		return nil, errors.New("unexpected report id")
	}
	payload := report[1 : psurcReportPayload+1]
	if payload[4] != psurcMagic {
		return nil, errors.New("bad magic")
	}

	le := binary.LittleEndian
	frame := &psurcFrame{
		tick:       uint64(le.Uint32(payload[0:4])),
		nodeID:     payload[5],
		ctrlOn:     payload[6] != 0,
		drvOn:      payload[7] != 0,
		isStop:     payload[8] != 0,
		rssiHint:   int8(payload[9]),
		supplyRMSV: float32(le.Uint16(payload[10:12])) / 1000.0,
		supplyMinV: float32(le.Uint16(payload[12:14])) / 1000.0,
		supplyMaxV: float32(le.Uint16(payload[14:16])) / 1000.0,
		ctrlRMSA:   float32(int16(le.Uint16(payload[16:18]))) / 1000.0,
		ctrlMinA:   float32(int16(le.Uint16(payload[18:20]))) / 1000.0,
		ctrlMaxA:   float32(int16(le.Uint16(payload[20:22]))) / 1000.0,
		drvRMSA:    float32(int16(le.Uint16(payload[22:24]))) / 1000.0,
		drvMinA:    float32(int16(le.Uint16(payload[24:26]))) / 1000.0,
		drvMaxA:    float32(int16(le.Uint16(payload[26:28]))) / 1000.0,
	}
	return frame, nil
}

func psurcNodeKey(info hid.DeviceInfo) string {
	base := strings.TrimSpace(info.Serial)
	if base == "" {
		base = strings.TrimSpace(info.Path)
	}
	if base == "" {
		base = fmt.Sprintf("%04x-%04x-%d", info.VendorID, info.ProductID, info.Interface)
	}
	base = strings.ToLower(base)
	base = strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-").Replace(base)
	return "psurc-" + base
}

func psurcNodeName(key string, info hid.DeviceInfo) string {
	if serial := strings.TrimSpace(info.Serial); serial != "" {
		return "PSU-RC-" + serial
	}
	if product := strings.TrimSpace(info.Product); product != "" {
		return product
	}
	return strings.ToUpper(key)
}

func pseudoMacFromString(seed string) [6]byte {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(seed))
	sum := hash.Sum64()
	return [6]byte{
		0xDC,
		0x03,
		byte(sum >> 24),
		byte(sum >> 16),
		byte(sum >> 8),
		byte(sum),
	}
}

func psurcDeviceMarkerLeds(mac [6]byte) [5]bool {
	var leds [5]bool
	seed := uint16(mac[4])<<8 | uint16(mac[5])
	if seed == 0 {
		seed = 1
	}
	var anyOn bool
	for i := range 5 {
		leds[i] = (seed & (1 << i)) != 0
		anyOn = anyOn || leds[i]
	}
	if !anyOn {
		leds[0] = true
	}
	return leds
}

func psurcUdevFixCommand() string {
	return `sudo tee /etc/udev/rules.d/99-psurc-hid.rules >/dev/null <<'EOF'
SUBSYSTEM=="usb", ATTR{idVendor}=="303a", ATTR{idProduct}=="4004", TAG+="uaccess"
KERNEL=="hidraw*", ATTRS{idVendor}=="303a", ATTRS{idProduct}=="4004", TAG+="uaccess"
EOF
sudo udevadm control --reload-rules
sudo udevadm trigger`
}
