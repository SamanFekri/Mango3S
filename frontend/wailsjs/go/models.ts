export namespace aggregate {
	
	export class Stage {
	    operator: string;
	    body: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Stage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operator = source["operator"];
	        this.body = source["body"];
	        this.enabled = source["enabled"];
	    }
	}
	export class Request {
	    connId: string;
	    db: string;
	    coll: string;
	    stages: Stage[];
	    upTo: number;
	    page: number;
	    pageSize: number;
	    allowDiskUse: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.stages = this.convertValues(source["stages"], Stage);
	        this.upTo = source["upTo"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.allowDiskUse = source["allowDiskUse"];
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

export namespace api {
	
	export class AppInfo {
	    version: string;
	    configDir: string;
	    logFile: string;
	    secretBackend: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.configDir = source["configDir"];
	        this.logFile = source["logFile"];
	        this.secretBackend = source["secretBackend"];
	    }
	}
	export class ProfileInput {
	    id: string;
	    name: string;
	    uri: string;
	    username: string;
	    password: string;
	    clearPassword: boolean;
	    authSource: string;
	    authMechanism: string;
	    connectTimeoutSeconds: number;
	    readOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.uri = source["uri"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.clearPassword = source["clearPassword"];
	        this.authSource = source["authSource"];
	        this.authMechanism = source["authMechanism"];
	        this.connectTimeoutSeconds = source["connectTimeoutSeconds"];
	        this.readOnly = source["readOnly"];
	    }
	}

}

export namespace config {
	
	export class Profile {
	    id: string;
	    name: string;
	    uri: string;
	    username?: string;
	    authSource?: string;
	    authMechanism?: string;
	    connectTimeoutSeconds: number;
	    readOnly: boolean;
	    hasPassword: boolean;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.uri = source["uri"];
	        this.username = source["username"];
	        this.authSource = source["authSource"];
	        this.authMechanism = source["authMechanism"];
	        this.connectTimeoutSeconds = source["connectTimeoutSeconds"];
	        this.readOnly = source["readOnly"];
	        this.hasPassword = source["hasPassword"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class Settings {
	    theme: string;
	    pageSize: number;
	    queryTimeoutSeconds: number;
	    confirmDeletes: boolean;
	    defaultView: string;
	    editorFontSize: number;
	    logLevel: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.pageSize = source["pageSize"];
	        this.queryTimeoutSeconds = source["queryTimeoutSeconds"];
	        this.confirmDeletes = source["confirmDeletes"];
	        this.defaultView = source["defaultView"];
	        this.editorFontSize = source["editorFontSize"];
	        this.logLevel = source["logLevel"];
	    }
	}

}

export namespace explorer {
	
	export class CollectionInfo {
	    name: string;
	    type: string;
	    readOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CollectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.readOnly = source["readOnly"];
	    }
	}
	export class IndexInfo {
	    name: string;
	    keys: string;
	    unique: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IndexInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.keys = source["keys"];
	        this.unique = source["unique"];
	    }
	}
	export class CollectionStats {
	    count: number;
	    size: number;
	    avgObjSize: number;
	    storageSize: number;
	    totalIndexSize: number;
	    indexes: IndexInfo[];
	
	    static createFrom(source: any = {}) {
	        return new CollectionStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.size = source["size"];
	        this.avgObjSize = source["avgObjSize"];
	        this.storageSize = source["storageSize"];
	        this.totalIndexSize = source["totalIndexSize"];
	        this.indexes = this.convertValues(source["indexes"], IndexInfo);
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
	export class DatabaseInfo {
	    name: string;
	    sizeOnDisk: number;
	    empty: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.sizeOnDisk = source["sizeOnDisk"];
	        this.empty = source["empty"];
	    }
	}

}

export namespace model {
	
	export class Document {
	    id: string;
	    text: string;
	    json: string;
	
	    static createFrom(source: any = {}) {
	        return new Document(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.text = source["text"];
	        this.json = source["json"];
	    }
	}
	export class Page {
	    documents: Document[];
	    page: number;
	    pageSize: number;
	    hasMore: boolean;
	    durationMs: number;
	    notice?: string;
	
	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.documents = this.convertValues(source["documents"], Document);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.hasMore = source["hasMore"];
	        this.durationMs = source["durationMs"];
	        this.notice = source["notice"];
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

export namespace mongox {
	
	export class SessionInfo {
	    id: string;
	    name: string;
	    readOnly: boolean;
	    serverVersion: string;
	    topology: string;
	    // Go type: time
	    connectedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.readOnly = source["readOnly"];
	        this.serverVersion = source["serverVersion"];
	        this.topology = source["topology"];
	        this.connectedAt = this.convertValues(source["connectedAt"], null);
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
	export class TestResult {
	    serverVersion: string;
	    topology: string;
	    latencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverVersion = source["serverVersion"];
	        this.topology = source["topology"];
	        this.latencyMs = source["latencyMs"];
	    }
	}

}

export namespace query {
	
	export class Request {
	    connId: string;
	    db: string;
	    coll: string;
	    filter: string;
	    projection: string;
	    sort: string;
	    skip: number;
	    limit: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.filter = source["filter"];
	        this.projection = source["projection"];
	        this.sort = source["sort"];
	        this.skip = source["skip"];
	        this.limit = source["limit"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}

}

export namespace shell {
	
	export class Output {
	    statement: string;
	    kind: string;
	    text?: string;
	    documents?: model.Document[];
	    hasMore: boolean;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Output(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statement = source["statement"];
	        this.kind = source["kind"];
	        this.text = source["text"];
	        this.documents = this.convertValues(source["documents"], model.Document);
	        this.hasMore = source["hasMore"];
	        this.durationMs = source["durationMs"];
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
	export class Result {
	    outputs: Output[];
	    currentDb: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputs = this.convertValues(source["outputs"], Output);
	        this.currentDb = source["currentDb"];
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

