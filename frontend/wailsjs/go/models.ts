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

}

