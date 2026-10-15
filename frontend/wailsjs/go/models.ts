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
	export class CodeRequest {
	    language: string;
	    source: string;
	    db: string;
	    coll: string;
	    query: query.Request;
	    stages: aggregate.Stage[];
	    sql: string;
	
	    static createFrom(source: any = {}) {
	        return new CodeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.source = source["source"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.query = this.convertValues(source["query"], query.Request);
	        this.stages = this.convertValues(source["stages"], aggregate.Stage);
	        this.sql = source["sql"];
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
	export class SQLRequest {
	    connId: string;
	    db: string;
	    sql: string;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new SQLRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.sql = source["sql"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class SQLTranslation {
	    kind: string;
	    collection: string;
	    shell: string;
	
	    static createFrom(source: any = {}) {
	        return new SQLTranslation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.collection = source["collection"];
	        this.shell = source["shell"];
	    }
	}
	export class SQLResult {
	    translation: SQLTranslation;
	    page: model.Page;
	
	    static createFrom(source: any = {}) {
	        return new SQLResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.translation = this.convertValues(source["translation"], SQLTranslation);
	        this.page = this.convertValues(source["page"], model.Page);
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

export namespace codegen {
	
	export class Language {
	    id: string;
	    label: string;
	    ext: string;
	
	    static createFrom(source: any = {}) {
	        return new Language(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.ext = source["ext"];
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

export namespace explain {
	
	export class Detail {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Detail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class Diagnostic {
	    level: string;
	    title: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	    }
	}
	export class Summary {
	    executionTimeMs?: number;
	    nReturned?: number;
	    docsExamined?: number;
	    keysExamined?: number;
	    indexesUsed: string[];
	    collScan: boolean;
	    inMemorySort: boolean;
	    covered: boolean;
	    rejectedPlans: number;
	    engine: string;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.executionTimeMs = source["executionTimeMs"];
	        this.nReturned = source["nReturned"];
	        this.docsExamined = source["docsExamined"];
	        this.keysExamined = source["keysExamined"];
	        this.indexesUsed = source["indexesUsed"];
	        this.collScan = source["collScan"];
	        this.inMemorySort = source["inMemorySort"];
	        this.covered = source["covered"];
	        this.rejectedPlans = source["rejectedPlans"];
	        this.engine = source["engine"];
	    }
	}
	export class Stage {
	    name: string;
	    details: Detail[];
	    nReturned?: number;
	    docsExamined?: number;
	    keysExamined?: number;
	    timeMs?: number;
	    children: Stage[];
	
	    static createFrom(source: any = {}) {
	        return new Stage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.details = this.convertValues(source["details"], Detail);
	        this.nReturned = source["nReturned"];
	        this.docsExamined = source["docsExamined"];
	        this.keysExamined = source["keysExamined"];
	        this.timeMs = source["timeMs"];
	        this.children = this.convertValues(source["children"], Stage);
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
	export class Plan {
	    namespace: string;
	    root?: Stage;
	    pipeline: Stage[];
	    summary: Summary;
	    diagnostics: Diagnostic[];
	    raw: string;
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.root = this.convertValues(source["root"], Stage);
	        this.pipeline = this.convertValues(source["pipeline"], Stage);
	        this.summary = this.convertValues(source["summary"], Summary);
	        this.diagnostics = this.convertValues(source["diagnostics"], Diagnostic);
	        this.raw = source["raw"];
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
	export class Request {
	    kind: string;
	    query: query.Request;
	    connId: string;
	    db: string;
	    coll: string;
	    stages: aggregate.Stage[];
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.query = this.convertValues(source["query"], query.Request);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.stages = this.convertValues(source["stages"], aggregate.Stage);
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

export namespace jobs {
	
	export class Job {
	    id: string;
	    kind: string;
	    title: string;
	    state: string;
	    done: number;
	    total: number;
	    unit: string;
	    message: string;
	    error?: string;
	    result?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.unit = source["unit"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.result = source["result"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
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

export namespace library {
	
	export class QueryPayload {
	    filter: string;
	    projection: string;
	    sort: string;
	    skip: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new QueryPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filter = source["filter"];
	        this.projection = source["projection"];
	        this.sort = source["sort"];
	        this.skip = source["skip"];
	        this.limit = source["limit"];
	    }
	}
	export class Entry {
	    id: string;
	    kind: string;
	    connName: string;
	    db: string;
	    coll: string;
	    summary: string;
	    query?: QueryPayload;
	    stages?: aggregate.Stage[];
	    text?: string;
	    // Go type: time
	    at: any;
	    durationMs: number;
	    ok: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.connName = source["connName"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.summary = source["summary"];
	        this.query = this.convertValues(source["query"], QueryPayload);
	        this.stages = this.convertValues(source["stages"], aggregate.Stage);
	        this.text = source["text"];
	        this.at = this.convertValues(source["at"], null);
	        this.durationMs = source["durationMs"];
	        this.ok = source["ok"];
	        this.error = source["error"];
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
	export class Item {
	    id: string;
	    kind: string;
	    name: string;
	    description: string;
	    db: string;
	    coll: string;
	    query?: QueryPayload;
	    stages?: aggregate.Stage[];
	    text?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.query = this.convertValues(source["query"], QueryPayload);
	        this.stages = this.convertValues(source["stages"], aggregate.Stage);
	        this.text = source["text"];
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

export namespace schema {
	
	export class TypeStat {
	    type: string;
	    count: number;
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new TypeStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.count = source["count"];
	        this.percent = source["percent"];
	    }
	}
	export class Field {
	    path: string;
	    name: string;
	    depth: number;
	    count: number;
	    presence: number;
	    types: TypeStat[];
	    elementTypes?: TypeStat[];
	    minLength?: number;
	    maxLength?: number;
	    avgLength?: number;
	    min?: string;
	    max?: string;
	    distinct: number;
	    distinctCapped: boolean;
	    unique: boolean;
	    samples: string[];
	
	    static createFrom(source: any = {}) {
	        return new Field(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.depth = source["depth"];
	        this.count = source["count"];
	        this.presence = source["presence"];
	        this.types = this.convertValues(source["types"], TypeStat);
	        this.elementTypes = this.convertValues(source["elementTypes"], TypeStat);
	        this.minLength = source["minLength"];
	        this.maxLength = source["maxLength"];
	        this.avgLength = source["avgLength"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.distinct = source["distinct"];
	        this.distinctCapped = source["distinctCapped"];
	        this.unique = source["unique"];
	        this.samples = source["samples"];
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
	export class Report {
	    sampled: number;
	    estimatedDocs: number;
	    fields: Field[];
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sampled = source["sampled"];
	        this.estimatedDocs = source["estimatedDocs"];
	        this.fields = this.convertValues(source["fields"], Field);
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
	export class Request {
	    connId: string;
	    db: string;
	    coll: string;
	    sampleSize: number;
	    filter: string;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.sampleSize = source["sampleSize"];
	        this.filter = source["filter"];
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

export namespace transfer {
	
	export class ColumnPreview {
	    name: string;
	    inferred: string;
	    samples: string[];
	
	    static createFrom(source: any = {}) {
	        return new ColumnPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.inferred = source["inferred"];
	        this.samples = source["samples"];
	    }
	}
	export class ColumnSpec {
	    name: string;
	    type: string;
	    skip: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColumnSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.skip = source["skip"];
	    }
	}
	export class ExportOptions {
	    connId: string;
	    db: string;
	    coll: string;
	    filter: string;
	    projection: string;
	    sort: string;
	    skip: number;
	    limit: number;
	    path: string;
	    format: string;
	    jsonMode: string;
	    canonical: boolean;
	    pretty: boolean;
	    delimiter: string;
	    noHeader: boolean;
	    fields: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExportOptions(source);
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
	        this.path = source["path"];
	        this.format = source["format"];
	        this.jsonMode = source["jsonMode"];
	        this.canonical = source["canonical"];
	        this.pretty = source["pretty"];
	        this.delimiter = source["delimiter"];
	        this.noHeader = source["noHeader"];
	        this.fields = source["fields"];
	    }
	}
	export class ImportOptions {
	    connId: string;
	    db: string;
	    coll: string;
	    path: string;
	    format: string;
	    delimiter: string;
	    columns: ColumnSpec[];
	    keepBlanks: boolean;
	    mode: string;
	    dropFirst: boolean;
	    stopOnError: boolean;
	    batchSize: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.path = source["path"];
	        this.format = source["format"];
	        this.delimiter = source["delimiter"];
	        this.columns = this.convertValues(source["columns"], ColumnSpec);
	        this.keepBlanks = source["keepBlanks"];
	        this.mode = source["mode"];
	        this.dropFirst = source["dropFirst"];
	        this.stopOnError = source["stopOnError"];
	        this.batchSize = source["batchSize"];
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
	export class Preview {
	    format: string;
	    sizeBytes: number;
	    documents: model.Document[];
	    columns: ColumnPreview[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.sizeBytes = source["sizeBytes"];
	        this.documents = this.convertValues(source["documents"], model.Document);
	        this.columns = this.convertValues(source["columns"], ColumnPreview);
	        this.error = source["error"];
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

