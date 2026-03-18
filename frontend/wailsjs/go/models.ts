export namespace config {
	
	export class Settings {
	    leaderName: string;
	    ignoreKeywords: string[];
	    llmProvider: string;
	    apiKey: string;
	    ollamaModel: string;
	    embeddingModel: string;
	    searchTopK: number;
	    maxChunkMessages: number;
	    chunkOverlap: number;
	    useLeaderMicro: boolean;
	    useSemanticChunk: boolean;
	    semanticThreshold: number;
	    autoUpdate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.leaderName = source["leaderName"];
	        this.ignoreKeywords = source["ignoreKeywords"];
	        this.llmProvider = source["llmProvider"];
	        this.apiKey = source["apiKey"];
	        this.ollamaModel = source["ollamaModel"];
	        this.embeddingModel = source["embeddingModel"];
	        this.searchTopK = source["searchTopK"];
	        this.maxChunkMessages = source["maxChunkMessages"];
	        this.chunkOverlap = source["chunkOverlap"];
	        this.useLeaderMicro = source["useLeaderMicro"];
	        this.useSemanticChunk = source["useSemanticChunk"];
	        this.semanticThreshold = source["semanticThreshold"];
	        this.autoUpdate = source["autoUpdate"];
	    }
	}

}

export namespace main {
	
	export class ChatMessage {
	    time: string;
	    speaker: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.speaker = source["speaker"];
	        this.content = source["content"];
	    }
	}
	export class KeywordHit {
	    time: string;
	    date: string;
	    dateKey: string;
	    speaker: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new KeywordHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.date = source["date"];
	        this.dateKey = source["dateKey"];
	        this.speaker = source["speaker"];
	        this.content = source["content"];
	    }
	}
	export class KeywordResult {
	    hits: KeywordHit[];
	    total: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KeywordResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hits = this.convertValues(source["hits"], KeywordHit);
	        this.total = source["total"];
	        this.hasMore = source["hasMore"];
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
	export class SourceChunk {
	    text: string;
	    isLeader: boolean;
	    startTime: string;
	    dateKey: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new SourceChunk(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.isLeader = source["isLeader"];
	        this.startTime = source["startTime"];
	        this.dateKey = source["dateKey"];
	        this.score = source["score"];
	    }
	}
	export class SearchResult {
	    summary: string;
	    sources: SourceChunk[];
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.sources = this.convertValues(source["sources"], SourceChunk);
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
	
	export class UpdateInfo {
	    available: boolean;
	    version: string;
	    releaseUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.releaseUrl = source["releaseUrl"];
	    }
	}

}

