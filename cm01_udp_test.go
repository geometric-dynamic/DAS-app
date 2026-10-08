package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUDPDeviceDiscoveryAndSamples(t *testing.T) {
	m := &CM01Manager{sessions: make(map[string]*cm01Session)}
	packet := func(ip, payload string) {
		t.Helper()
		m.handlePacket(&net.UDPAddr{IP: net.ParseIP(ip)}, []byte(payload))
	}
	packet("192.0.2.1", `{"type":"esp_heartbeat","dev":"UNKNOWN"}`)
	packet("192.0.2.3", `{"type":"data_batch","records":[{"ch":"INPUT_VOLTAGE","ts":10,"cur":28}]}`)
	if len(m.sessions) != 0 {
		t.Fatal("unknown device or undiscovered batch created a session")
	}
	packet("192.0.2.1", `{"type":"esp_heartbeat","dev":"DAS_NODE_CM01"}`)
	packet("192.0.2.2", `{"type":"esp_heartbeat","dev":"PDATX_A"}`)
	cm, pd := m.sessions["192.0.2.1"], m.sessions["192.0.2.2"]
	if cm == nil || pd == nil || cm.typeCode != cm01TypeCode || pd.typeCode != pdatxTypeCode || cm.nodeKey == pd.nodeKey {
		t.Fatal("CM01 and PDATX-A discovery did not produce distinct node types and keys")
	}
	if found := m.SnapshotDiscoveredNodes()[pd.nodeKey]; found.Name != "PDATX-A-192-0-2-2" || found.TypeCode != pdatxTypeCode {
		t.Fatalf("wrong PDATX-A discovery: %+v", found)
	}
	packet("192.0.2.1", `{"type":"esp_heartbeat","dev":"PDATX_A"}`)
	if m.sessions["192.0.2.1"].nodeKey == cm.nodeKey {
		t.Fatal("device types at the same IP share a node key")
	}
	pd.lastSeen = time.Time{}
	packet("192.0.2.2", `{"type":"data_batch","records":[{"ch":"INPUT_VOLTAGE","ts":90,"cur":27}]}`)
	if pd.lastSeen.IsZero() {
		t.Fatal("discovered device batch did not confirm the connection handshake")
	}

	node := &Node{}
	node.InitFrom(ScanData{Type: pdatxTypeCode, Mac: pd.mac})
	node.Name = "PDATX-A-test"
	nodesMu.Lock()
	nodes[pd.nodeKey] = node
	nodesMu.Unlock()
	t.Cleanup(func() {
		nodesMu.Lock()
		delete(nodes, pd.nodeKey)
		nodesMu.Unlock()
	})
	pd.connected = true
	recorderManager.mu.Lock()
	recorderManager.active = true
	recorderManager.dir = t.TempDir()
	recorderManager.recorders = make(map[string]*NodeRecorder)
	recorderManager.mu.Unlock()
	t.Cleanup(func() { recorderManager.Stop() })
	packet("192.0.2.2", `{"type":"data_batch","records":[{"ch":"INPUT_VOLTAGE","ts":100,"cur":27.99},{"ch":"OUTPUT_CURRENT","ts":100,"cur":1.2},{"ch":"INPUT_VOLTAGE","ts":110,"cur":28},{"ch":"OUTPUT_CURRENT","ts":120,"cur":2}]}`)
	if len(node.AxisX) != 1 || len(node.Metrics) != 2 || node.Metrics[0].CurrentValue != 27.99 || node.Metrics[1].CurrentValue != 1.2 || len(node.Stats) != 0 {
		t.Fatalf("wrong PDATX-A sample mapping: axis=%v metrics=%+v stats=%v", node.AxisX, node.Metrics, node.Stats)
	}
	cols := NodeTypeMap[pdatxTypeCode].CSVColumns
	if len(cols) != 2 || cols[0].Name != "input_voltage" || cols[1].Name != "output_current" {
		t.Fatalf("wrong PDATX-A CSV columns: %+v", cols)
	}
	recorderManager.Stop()
	csv, err := os.ReadFile(filepath.Join(recorderManager.dir, node.Name+"-"+pd.nodeKey+".csv"))
	if err != nil || !strings.Contains(string(csv), "local_timestamp,input_voltage,output_current\n") || !strings.Contains(string(csv), ",27.99,1.2\n") || strings.Count(string(csv), "\n") != 2 {
		t.Fatalf("wrong PDATX-A CSV: %q, %v", csv, err)
	}
}
