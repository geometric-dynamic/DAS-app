export namespace main {
	
	export class AppState {
	    simulating: boolean;
	    hasExternalNodes: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.simulating = source["simulating"];
	        this.hasExternalNodes = source["hasExternalNodes"];
	    }
	}
	export class DiscoveredNode {
	    nodeKey: string;
	    name: string;
	    typeCode: number;
	    typeName: string;
	    ip: string;
	    mac: number[];
	    online: boolean;
	    connected: boolean;
	    lastSeen: number;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeKey = source["nodeKey"];
	        this.name = source["name"];
	        this.typeCode = source["typeCode"];
	        this.typeName = source["typeName"];
	        this.ip = source["ip"];
	        this.mac = source["mac"];
	        this.online = source["online"];
	        this.connected = source["connected"];
	        this.lastSeen = source["lastSeen"];
	    }
	}
	export class NodeStatValue {
	    Key: string;
	    Name: string;
	    Unit: string;
	    Digit: number;
	    Value: number;
	
	    static createFrom(source: any = {}) {
	        return new NodeStatValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.Name = source["Name"];
	        this.Unit = source["Unit"];
	        this.Digit = source["Digit"];
	        this.Value = source["Value"];
	    }
	}
	export class MetricSeries {
	    Key: string;
	    Name: string;
	    Unit: string;
	    Digit: number;
	    Color: string;
	    Kind: string;
	    CurrentValue: number;
	    AxisY: number[];
	
	    static createFrom(source: any = {}) {
	        return new MetricSeries(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.Name = source["Name"];
	        this.Unit = source["Unit"];
	        this.Digit = source["Digit"];
	        this.Color = source["Color"];
	        this.Kind = source["Kind"];
	        this.CurrentValue = source["CurrentValue"];
	        this.AxisY = source["AxisY"];
	    }
	}
	export class Node {
	    Name: string;
	    TypeCode: number;
	    TypeName: string;
	    Source: string;
	    Battery: number;
	    RSSI: number;
	    Mac: number[];
	    Leds: boolean[];
	    AxisX: number[];
	    Metrics: MetricSeries[];
	    Stats: NodeStatValue[];
	    FirstTick: number;
	    FirstLocalTs: number;
	    TimeSynced: boolean;
	    LastDeviceTs: number;
	
	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.TypeCode = source["TypeCode"];
	        this.TypeName = source["TypeName"];
	        this.Source = source["Source"];
	        this.Battery = source["Battery"];
	        this.RSSI = source["RSSI"];
	        this.Mac = source["Mac"];
	        this.Leds = source["Leds"];
	        this.AxisX = source["AxisX"];
	        this.Metrics = this.convertValues(source["Metrics"], MetricSeries);
	        this.Stats = this.convertValues(source["Stats"], NodeStatValue);
	        this.FirstTick = source["FirstTick"];
	        this.FirstLocalTs = source["FirstLocalTs"];
	        this.TimeSynced = source["TimeSynced"];
	        this.LastDeviceTs = source["LastDeviceTs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FrontendState {
	    nodes: Record<string, Node>;
	    discoveredNodes: Record<string, DiscoveredNode>;
	    appState: AppState;
	
	    static createFrom(source: any = {}) {
	        return new FrontendState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], Node, true);
	        this.discoveredNodes = this.convertValues(source["discoveredNodes"], DiscoveredNode, true);
	        this.appState = this.convertValues(source["appState"], AppState);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

