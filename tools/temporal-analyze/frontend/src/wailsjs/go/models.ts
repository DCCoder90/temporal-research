export namespace main {
	
	export class AnalysisOptions {
	    Only: string[];
	    Exclude: string[];
	    OnlyHosts: string[];
	    ExcludeHosts: string[];
	    NoInterservice: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Only = source["Only"];
	        this.Exclude = source["Exclude"];
	        this.OnlyHosts = source["OnlyHosts"];
	        this.ExcludeHosts = source["ExcludeHosts"];
	        this.NoInterservice = source["NoInterservice"];
	    }
	}
	export class AnalysisResult {
	    PcapName: string;
	    Duration: number;
	    TotalBytes: number;
	    PacketCount: number;
	    GRPCCount: number;
	    FilterDesc: string;
	    FlowDiagram: string;
	    SeqDiagrams: string[];
	    TrafficSeq: string;
	    StatsMarkdown: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.PcapName = source["PcapName"];
	        this.Duration = source["Duration"];
	        this.TotalBytes = source["TotalBytes"];
	        this.PacketCount = source["PacketCount"];
	        this.GRPCCount = source["GRPCCount"];
	        this.FilterDesc = source["FilterDesc"];
	        this.FlowDiagram = source["FlowDiagram"];
	        this.SeqDiagrams = source["SeqDiagrams"];
	        this.TrafficSeq = source["TrafficSeq"];
	        this.StatsMarkdown = source["StatsMarkdown"];
	    }
	}
	export class QueryResult {
	    Columns: string[];
	    Rows: any[][];
	    RowCount: number;
	    Truncated: boolean;
	    SQLError: string;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Columns = source["Columns"];
	        this.Rows = source["Rows"];
	        this.RowCount = source["RowCount"];
	        this.Truncated = source["Truncated"];
	        this.SQLError = source["SQLError"];
	    }
	}

}

