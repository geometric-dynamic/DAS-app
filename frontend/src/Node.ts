export interface TypeMetric {
	Key: string
	Name: string
	Unit: string
	Digit: number
	Color: string
	Kind: "source" | "derived"
	CurrentValue: number
	AxisY: number[]
}

export interface TypeStat {
	Key: string
	Name: string
	Unit: string
	Digit: number
	Value: number
}

export interface TypeNode {
	Name: string
	TypeCode: number
	TypeName: string
	Source: "sim" | "external"
	Battery: number
	RSSI: number
	Mac: number[]
	Leds: number[]
	AxisX: number[]
	Metrics: TypeMetric[]
	Stats: TypeStat[]
}

export interface TypeDiscoveredNode {
	nodeKey: string
	name: string
	typeCode: number
	typeName: string
	ip: string
	mac: number[]
	online: boolean
	connected: boolean
	lastSeen: number
}

export interface TypeAppState {
	Simulating: boolean
	HasExternalNodes: boolean
}

export interface TypeFrontendState {
	Nodes: Record<string, TypeNode>
	DiscoveredNodes: Record<string, TypeDiscoveredNode>
	AppState: TypeAppState
}
