package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MetricKind string

const (
	MetricSource  MetricKind = "source"
	MetricDerived MetricKind = "derived"
)

type MetricDef struct {
	Key   string
	Name  string
	Unit  string
	Digit int
	Color string
	Kind  MetricKind
}

type TickToMillisFunc func(deltaTick uint64) int64
type ComputeMetricFunc func(values map[string]float32) float32
type StatRuntime map[string]any

type CreateStatRuntimeFunc func() StatRuntime
type ResetStatRuntimeFunc func(runtime StatRuntime)
type UpdateStatRuntimeFunc func(runtime StatRuntime, ts int64, values map[string]float32)

// StatDef defines a type-specific accumulated statistic.
// Statistics are owned by the node type template instead of Node itself,
// so future node types can implement distance, duration, energy, etc.
type StatDef struct {
	Key           string
	Name          string
	Unit          string
	Digit         int
	CreateRuntime CreateStatRuntimeFunc
	ResetRuntime  ResetStatRuntimeFunc
	UpdateRuntime UpdateStatRuntimeFunc
}

type CSVColumnDef struct {
	Key  string
	Name string
}

type NodeTypeDef struct {
	Code         uint16
	Name         string
	Metrics      []MetricDef
	Stats        []StatDef
	CSVColumns   []CSVColumnDef
	TickToMillis TickToMillisFunc
	Compute      map[string]ComputeMetricFunc
}

var NodeTypeMap = map[uint16]NodeTypeDef{
	0xDC01: {
		Code: 0xDC01,
		Name: "CM01",
		Metrics: []MetricDef{
			{Key: "voltage", Name: "Voltage", Unit: "V", Digit: 1, Color: "#3377ff", Kind: MetricSource},
			{Key: "current", Name: "Current", Unit: "A", Digit: 1, Color: "#d1dc00", Kind: MetricSource},
			{Key: "power", Name: "Power", Unit: "W", Digit: 1, Color: "#cd0000", Kind: MetricDerived},
		},
		Stats: []StatDef{newEnergyStatDef("power")},
		CSVColumns: []CSVColumnDef{
			{Key: "voltage", Name: "voltage"},
			{Key: "current", Name: "current"},
			{Key: "power", Name: "power"},
		},
		TickToMillis: func(deltaTick uint64) int64 { return int64(deltaTick) },
		Compute: map[string]ComputeMetricFunc{
			"power": func(values map[string]float32) float32 {
				return values["voltage"] * values["current"]
			},
		},
	},
	0xDC02: {
		Code: 0xDC02,
		Name: "CM02",
		Metrics: []MetricDef{
			{Key: "voltage", Name: "Voltage", Unit: "V", Digit: 1, Color: "#3377ff", Kind: MetricSource},
			{Key: "controlCurrent", Name: "Control Current", Unit: "A", Digit: 2, Color: "#d1dc00", Kind: MetricSource},
			{Key: "driverCurrent", Name: "Driver Current", Unit: "A", Digit: 2, Color: "#7be495", Kind: MetricSource},
			{Key: "controlPower", Name: "Control Power", Unit: "W", Digit: 1, Color: "#f59e0b", Kind: MetricDerived},
			{Key: "driverPower", Name: "Driver Power", Unit: "W", Digit: 1, Color: "#ef4444", Kind: MetricDerived},
			{Key: "totalPower", Name: "Total Power", Unit: "W", Digit: 1, Color: "#a855f7", Kind: MetricDerived},
		},
		Stats: []StatDef{newEnergyStatDef("totalPower")},
		CSVColumns: []CSVColumnDef{
			{Key: "voltage", Name: "voltage"},
			{Key: "controlCurrent", Name: "control_current"},
			{Key: "driverCurrent", Name: "driver_current"},
			{Key: "controlPower", Name: "control_power"},
			{Key: "driverPower", Name: "driver_power"},
			{Key: "totalPower", Name: "total_power"},
		},
		TickToMillis: func(deltaTick uint64) int64 { return int64(deltaTick) },
		Compute: map[string]ComputeMetricFunc{
			"controlPower": func(values map[string]float32) float32 {
				return values["voltage"] * values["controlCurrent"]
			},
			"driverPower": func(values map[string]float32) float32 {
				return values["voltage"] * values["driverCurrent"]
			},
			"totalPower": func(values map[string]float32) float32 {
				return values["voltage"] * (values["controlCurrent"] + values["driverCurrent"])
			},
		},
	},
}

func newEnergyStatDef(powerKey string) StatDef {
	return StatDef{
		Key:   "energyWh",
		Name:  "Energy",
		Unit:  "Wh",
		Digit: 3,
		CreateRuntime: func() StatRuntime {
			return StatRuntime{"value": float64(0), "lastTs": int64(0), "lastPower": float64(0), "started": false}
		},
		ResetRuntime: func(runtime StatRuntime) {
			runtime["value"] = float64(0)
			runtime["lastTs"] = int64(0)
			runtime["lastPower"] = float64(0)
			runtime["started"] = false
		},
		UpdateRuntime: func(runtime StatRuntime, ts int64, values map[string]float32) {
			power := float64(values[powerKey])
			started, _ := runtime["started"].(bool)
			if !started {
				runtime["lastTs"] = ts
				runtime["lastPower"] = power
				runtime["started"] = true
				return
			}
			lastTs, _ := runtime["lastTs"].(int64)
			lastPower, _ := runtime["lastPower"].(float64)
			if ts > lastTs {
				value, _ := runtime["value"].(float64)
				value += lastPower * float64(ts-lastTs) / 3600000.0
				runtime["value"] = value
			}
			runtime["lastTs"] = ts
			runtime["lastPower"] = power
		},
	}
}

type ScanData struct {
	Mac   [6]uint8
	Leds  uint8
	Type  uint16
	Index uint32
	Tick  uint64
	Data  []uint8
}

type MetricSeries struct {
	Key          string
	Name         string
	Unit         string
	Digit        int
	Color        string
	Kind         MetricKind
	CurrentValue float32
	AxisY        []float32
}

type NodeStatValue struct {
	Key   string
	Name  string
	Unit  string
	Digit int
	Value float64
}

type Node struct {
	Name     string
	TypeCode uint16
	TypeName string
	Source   string
	Battery  int
	RSSI     int
	Mac      [6]uint8
	Leds     [5]bool
	AxisX    []int64
	Metrics  []*MetricSeries
	Stats    []*NodeStatValue

	FirstTick    uint64
	FirstLocalTs int64
	TimeSynced   bool
	lastIdx      uint32
	startIdx     uint32
	startTime    time.Time
	LastDeviceTs uint64

	statRuntime map[string]StatRuntime
}

type Nodes map[string]*Node

var nodes Nodes

func init() {
	nodes = make(Nodes)
}

func (ns *Nodes) Clear() {
	*ns = make(Nodes)
}

func (n *Node) InitFrom(data ScanData) {
	n.startTime = time.Now()
	n.startIdx = data.Index
	n.TypeCode = data.Type
	if n.Source == "" {
		n.Source = "external"
	}
	if def, ok := NodeTypeMap[data.Type]; ok {
		n.TypeName = def.Name
		n.Metrics = make([]*MetricSeries, 0, len(def.Metrics))
		for _, metricDef := range def.Metrics {
			n.Metrics = append(n.Metrics, &MetricSeries{
				Key:   metricDef.Key,
				Name:  metricDef.Name,
				Unit:  metricDef.Unit,
				Digit: metricDef.Digit,
				Color: metricDef.Color,
				Kind:  metricDef.Kind,
			})
		}
		n.Stats = make([]*NodeStatValue, 0, len(def.Stats))
		n.statRuntime = make(map[string]StatRuntime, len(def.Stats))
		for _, statDef := range def.Stats {
			runtime := StatRuntime{}
			if statDef.CreateRuntime != nil {
				runtime = statDef.CreateRuntime()
			}
			n.statRuntime[statDef.Key] = runtime
			n.Stats = append(n.Stats, &NodeStatValue{Key: statDef.Key, Name: statDef.Name, Unit: statDef.Unit, Digit: statDef.Digit})
		}
	}
	n.Mac = data.Mac
}

// ResetTypeStats resets only type-specific statistic states.
// The generic node model intentionally does not know how each statistic works.
func (n *Node) ResetTypeStats() {
	def, ok := NodeTypeMap[n.TypeCode]
	if !ok {
		return
	}
	for _, statDef := range def.Stats {
		runtime := n.statRuntime[statDef.Key]
		if runtime == nil && statDef.CreateRuntime != nil {
			runtime = statDef.CreateRuntime()
			n.statRuntime[statDef.Key] = runtime
		}
		if statDef.ResetRuntime != nil {
			statDef.ResetRuntime(runtime)
		}
		if stat := n.findStat(statDef.Key); stat != nil {
			stat.Value = 0
		}
	}
}

func (n *Node) findStat(key string) *NodeStatValue {
	for _, stat := range n.Stats {
		if stat.Key == key {
			return stat
		}
	}
	return nil
}

func (n *Node) NormalizeTimestamp(tick uint64, hostNow int64) int64 {
	def, ok := NodeTypeMap[n.TypeCode]
	if !ok || def.TickToMillis == nil {
		return hostNow
	}
	if !n.TimeSynced {
		n.FirstTick = tick
		n.FirstLocalTs = hostNow
		n.TimeSynced = true
		return hostNow
	}
	return n.FirstLocalTs + def.TickToMillis(tick-n.FirstTick)
}

func (n *Node) AppendFrame(ts int64, values map[string]float32) {
	if len(n.AxisX) > 0 && n.AxisX[len(n.AxisX)-1] == ts {
		for _, metric := range n.Metrics {
			value := values[metric.Key]
			metric.CurrentValue = value
			if len(metric.AxisY) > 0 {
				metric.AxisY[len(metric.AxisY)-1] = value
			}
		}
		return
	}
	n.AxisX = append(n.AxisX, ts)
	for _, metric := range n.Metrics {
		value := values[metric.Key]
		metric.CurrentValue = value
		metric.AxisY = append(metric.AxisY, value)
	}
	if len(n.AxisX) > 512 {
		n.AxisX = n.AxisX[len(n.AxisX)-512:]
		for _, metric := range n.Metrics {
			if len(metric.AxisY) > 512 {
				metric.AxisY = metric.AxisY[len(metric.AxisY)-512:]
			}
		}
	}
	statsManager.OnNodeFrame(n, ts, values)
	recorderManager.OnNodeFrame(n, ts, values)
}

func (n *Node) UpdateFrom(data ScanData) {
	n.lastIdx = data.Index
	_ = data
}

type StatsManager struct {
	mu      sync.Mutex
	active  bool
	started time.Time
}

var statsManager = &StatsManager{}

// Global statistics only control the lifecycle.
// The actual accumulation algorithm stays inside the node type stat definition.
func (m *StatsManager) Start() {
	m.mu.Lock()
	m.active = true
	m.started = time.Now()
	m.mu.Unlock()
	for _, node := range nodes {
		node.ResetTypeStats()
	}
	if app != nil {
		app.NewDataNotify()
	}
}

func (m *StatsManager) Stop() {
	m.mu.Lock()
	m.active = false
	m.mu.Unlock()
	if app != nil {
		app.NewDataNotify()
	}
}

func (m *StatsManager) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

func (m *StatsManager) OnNodeFrame(node *Node, ts int64, values map[string]float32) {
	m.mu.Lock()
	active := m.active
	m.mu.Unlock()
	if !active {
		return
	}
	def, ok := NodeTypeMap[node.TypeCode]
	if !ok {
		return
	}
	for _, statDef := range def.Stats {
		runtime := node.statRuntime[statDef.Key]
		if runtime == nil && statDef.CreateRuntime != nil {
			runtime = statDef.CreateRuntime()
			node.statRuntime[statDef.Key] = runtime
		}
		if statDef.UpdateRuntime != nil {
			statDef.UpdateRuntime(runtime, ts, values)
		}
		if stat := node.findStat(statDef.Key); stat != nil {
			if value, ok := runtime["value"].(float64); ok {
				stat.Value = value
			}
		}
	}
}

type NodeRecorder struct {
	file   *os.File
	writer *csv.Writer
	buf    [][]string
	def    NodeTypeDef
}

type RecorderManager struct {
	mu          sync.Mutex
	active      bool
	dir         string
	recorders   map[string]*NodeRecorder
	flushTicker *time.Ticker
	stopCh      chan struct{}
}

var recorderManager = &RecorderManager{recorders: map[string]*NodeRecorder{}}

// Recording uses buffered CSV writes so frequent telemetry updates do not turn
// into a sync-heavy per-frame disk workload.
func (m *RecorderManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active {
		return nil
	}
	dir := filepath.Join(".", "log-"+time.Now().Format("20060102150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	m.active = true
	m.dir = dir
	m.recorders = map[string]*NodeRecorder{}
	m.stopCh = make(chan struct{})
	m.flushTicker = time.NewTicker(time.Second)
	for key, node := range nodes {
		_ = m.ensureRecorderLocked(key, node)
	}
	go m.flushLoop(m.stopCh, m.flushTicker)
	return nil
}

func (m *RecorderManager) Stop() {
	m.mu.Lock()
	if !m.active {
		m.mu.Unlock()
		return
	}
	m.active = false
	stopCh := m.stopCh
	if m.flushTicker != nil {
		m.flushTicker.Stop()
	}
	m.stopCh = nil
	m.flushTicker = nil
	for key, rec := range m.recorders {
		m.flushRecorderLocked(rec)
		_ = rec.file.Close()
		delete(m.recorders, key)
	}
	m.mu.Unlock()
	if stopCh != nil {
		close(stopCh)
	}
	if app != nil {
		app.NewDataNotify()
	}
}

func (m *RecorderManager) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

func (m *RecorderManager) OnNodeFrame(node *Node, ts int64, values map[string]float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active {
		return
	}
	rec := m.ensureRecorderLocked(MacStr(node.Mac), node)
	if rec == nil {
		return
	}
	row := []string{time.UnixMilli(ts).Format(time.RFC3339Nano)}
	for _, col := range rec.def.CSVColumns {
		row = append(row, strconv.FormatFloat(float64(values[col.Key]), 'f', -1, 32))
	}
	rec.buf = append(rec.buf, row)
	if len(rec.buf) >= 32 {
		m.flushRecorderLocked(rec)
	}
}

func (m *RecorderManager) ensureRecorderLocked(key string, node *Node) *NodeRecorder {
	if rec, ok := m.recorders[key]; ok {
		return rec
	}
	def, ok := NodeTypeMap[node.TypeCode]
	if !ok {
		return nil
	}
	fileName := sanitizeFileName(node.Name)
	if fileName == "" {
		fileName = key
	}
	filePath := filepath.Join(m.dir, fileName+"-"+key+".csv")
	f, err := os.Create(filePath)
	if err != nil {
		return nil
	}
	writer := csv.NewWriter(f)
	header := []string{"local_timestamp"}
	for _, col := range def.CSVColumns {
		header = append(header, col.Name)
	}
	_ = writer.Write(header)
	writer.Flush()
	rec := &NodeRecorder{file: f, writer: writer, def: def}
	m.recorders[key] = rec
	return rec
}

func (m *RecorderManager) flushLoop(stopCh chan struct{}, ticker *time.Ticker) {
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			m.mu.Lock()
			for _, rec := range m.recorders {
				m.flushRecorderLocked(rec)
			}
			m.mu.Unlock()
		}
	}
}

func (m *RecorderManager) flushRecorderLocked(rec *NodeRecorder) {
	if len(rec.buf) == 0 {
		return
	}
	for _, row := range rec.buf {
		_ = rec.writer.Write(row)
	}
	rec.writer.Flush()
	rec.buf = rec.buf[:0]
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}

func (n *Node) MetricMap() map[string]*MetricSeries {
	metrics := make(map[string]*MetricSeries, len(n.Metrics))
	for _, metric := range n.Metrics {
		metrics[metric.Key] = metric
	}
	return metrics
}

func (n *Node) ApplyMetricRecords(records map[uint64]map[string]float32, hostNow int64) bool {
	if len(records) == 0 {
		return false
	}
	keys := make([]uint64, 0, len(records))
	for ts := range records {
		if ts <= n.LastDeviceTs {
			continue
		}
		keys = append(keys, ts)
	}
	if len(keys) == 0 {
		return false
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, tick := range keys {
		values := records[tick]
		if def, ok := NodeTypeMap[n.TypeCode]; ok {
			for key, compute := range def.Compute {
				values[key] = compute(values)
			}
		}
		ts := n.NormalizeTimestamp(tick, hostNow)
		n.AppendFrame(ts, values)
		n.LastDeviceTs = tick
	}
	return true
}

func OnScanData(d ScanData) {
	var macStr string = MacStr(d.Mac)
	if _, ok := nodes[macStr]; !ok {
		nodes[macStr] = &Node{}
		nodes[macStr].InitFrom(d)
	}
	nodes[macStr].UpdateFrom(d)
}

func MacStr(mac [6]byte) string {
	var macStr string
	for i := 0; i < 6; i++ {
		macStr += fmt.Sprintf("%02x", mac[i])
	}
	return macStr
}

func HasExternalNodes() bool {
	for _, node := range nodes {
		if node != nil && node.Source == "external" {
			return true
		}
	}
	return false
}
