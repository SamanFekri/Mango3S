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

export namespace ai {
	
	export class CLIInfo {
	    name: string;
	    found: boolean;
	    path: string;
	    version: string;
	    signedIn?: boolean;
	    account: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CLIInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.found = source["found"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.signedIn = source["signedIn"];
	        this.account = source["account"];
	        this.message = source["message"];
	    }
	}
	export class Check {
	    name: string;
	    ok: boolean;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Check(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ok = source["ok"];
	        this.detail = source["detail"];
	    }
	}
	export class FieldSummary {
	    path: string;
	    types: string[];
	    presence: number;
	
	    static createFrom(source: any = {}) {
	        return new FieldSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.types = source["types"];
	        this.presence = source["presence"];
	    }
	}
	export class Context {
	    collection: string;
	    fields: FieldSummary[];
	    samples: string[];
	
	    static createFrom(source: any = {}) {
	        return new Context(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.collection = source["collection"];
	        this.fields = this.convertValues(source["fields"], FieldSummary);
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
	
	export class LocalServer {
	    name: string;
	    url: string;
	    api: string;
	    models: string[];
	
	    static createFrom(source: any = {}) {
	        return new LocalServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.api = source["api"];
	        this.models = source["models"];
	    }
	}
	export class Proposal {
	    id: string;
	    kind: string;
	    db: string;
	    coll: string;
	    filter: string;
	    projection: string;
	    sort: string;
	    limit: number;
	    pipeline: aggregate.Stage[];
	    update: string;
	    explanation: string;
	    warnings: string[];
	    isWrite: boolean;
	    requiresConfirmation: boolean;
	    valid: boolean;
	    validation: Check[];
	    shell: string;
	    sent: Context;
	    model: string;
	    inputTokens: number;
	    outputTokens: number;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Proposal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.filter = source["filter"];
	        this.projection = source["projection"];
	        this.sort = source["sort"];
	        this.limit = source["limit"];
	        this.pipeline = this.convertValues(source["pipeline"], aggregate.Stage);
	        this.update = source["update"];
	        this.explanation = source["explanation"];
	        this.warnings = source["warnings"];
	        this.isWrite = source["isWrite"];
	        this.requiresConfirmation = source["requiresConfirmation"];
	        this.valid = source["valid"];
	        this.validation = this.convertValues(source["validation"], Check);
	        this.shell = source["shell"];
	        this.sent = this.convertValues(source["sent"], Context);
	        this.model = source["model"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
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
	    prompt: string;
	    mode: string;
	    includeSamples: boolean;
	    allowWrites: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.prompt = source["prompt"];
	        this.mode = source["mode"];
	        this.includeSamples = source["includeSamples"];
	        this.allowWrites = source["allowWrites"];
	    }
	}
	export class WritePreview {
	    proposalId: string;
	    shell: string;
	    affected: number;
	    sampleIds: string[];
	    confirmText: string;
	
	    static createFrom(source: any = {}) {
	        return new WritePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proposalId = source["proposalId"];
	        this.shell = source["shell"];
	        this.affected = source["affected"];
	        this.sampleIds = source["sampleIds"];
	        this.confirmText = source["confirmText"];
	    }
	}
	export class WriteResult {
	    matched: number;
	    modified: number;
	    deleted: number;
	    backupPath: string;
	
	    static createFrom(source: any = {}) {
	        return new WriteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.matched = source["matched"];
	        this.modified = source["modified"];
	        this.deleted = source["deleted"];
	        this.backupPath = source["backupPath"];
	    }
	}

}

export namespace api {
	
	export class AIProviders {
	    claudeCode: ai.CLIInfo;
	    codex: ai.CLIInfo;
	    localServers: ai.LocalServer[];
	
	    static createFrom(source: any = {}) {
	        return new AIProviders(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.claudeCode = this.convertValues(source["claudeCode"], ai.CLIInfo);
	        this.codex = this.convertValues(source["codex"], ai.CLIInfo);
	        this.localServers = this.convertValues(source["localServers"], ai.LocalServer);
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
	export class AIStatus {
	    enabled: boolean;
	    provider: string;
	    providerName: string;
	    ready: boolean;
	    detail: string;
	    model: string;
	    leavesDevice: boolean;
	    hasKey: boolean;
	    keySource: string;
	    allowSamples: boolean;
	    localHasKey: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.providerName = source["providerName"];
	        this.ready = source["ready"];
	        this.detail = source["detail"];
	        this.model = source["model"];
	        this.leavesDevice = source["leavesDevice"];
	        this.hasKey = source["hasKey"];
	        this.keySource = source["keySource"];
	        this.allowSamples = source["allowSamples"];
	        this.localHasKey = source["localHasKey"];
	    }
	}
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
	export class MCPInfo {
	    policy: mcpserver.Policy;
	    catalog: mcpserver.ToolInfo[];
	    command: string;
	    args: string[];
	    snippet: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.policy = this.convertValues(source["policy"], mcpserver.Policy);
	        this.catalog = this.convertValues(source["catalog"], mcpserver.ToolInfo);
	        this.command = source["command"];
	        this.args = source["args"];
	        this.snippet = source["snippet"];
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
	export class SecretInput {
	    value: string;
	    clear: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SecretInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.clear = source["clear"];
	    }
	}
	export class ProfileInput {
	    id: string;
	    name: string;
	    folder: string;
	    uri: string;
	    server: config.ServerConfig;
	    username: string;
	    password: string;
	    clearPassword: boolean;
	    authSource: string;
	    authMechanism: string;
	    authProperties: string;
	    connectTimeoutSeconds: number;
	    readOnly: boolean;
	    tls: config.TLSConfig;
	    ssh: config.SSHConfig;
	    proxy: config.ProxyConfig;
	    shell: config.ShellConfig;
	    tools: config.ToolsConfig;
	    advanced: config.AdvancedConfig;
	    secrets: Record<string, SecretInput>;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.folder = source["folder"];
	        this.uri = source["uri"];
	        this.server = this.convertValues(source["server"], config.ServerConfig);
	        this.username = source["username"];
	        this.password = source["password"];
	        this.clearPassword = source["clearPassword"];
	        this.authSource = source["authSource"];
	        this.authMechanism = source["authMechanism"];
	        this.authProperties = source["authProperties"];
	        this.connectTimeoutSeconds = source["connectTimeoutSeconds"];
	        this.readOnly = source["readOnly"];
	        this.tls = this.convertValues(source["tls"], config.TLSConfig);
	        this.ssh = this.convertValues(source["ssh"], config.SSHConfig);
	        this.proxy = this.convertValues(source["proxy"], config.ProxyConfig);
	        this.shell = this.convertValues(source["shell"], config.ShellConfig);
	        this.tools = this.convertValues(source["tools"], config.ToolsConfig);
	        this.advanced = this.convertValues(source["advanced"], config.AdvancedConfig);
	        this.secrets = this.convertValues(source["secrets"], SecretInput, true);
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
	
	
	export class TransformPreview {
	    planId: string;
	    preview: transform.Preview;
	    target: string;
	    confirmText: string;
	
	    static createFrom(source: any = {}) {
	        return new TransformPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.planId = source["planId"];
	        this.preview = this.convertValues(source["preview"], transform.Preview);
	        this.target = source["target"];
	        this.confirmText = source["confirmText"];
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
	export class TransformRequest {
	    spec: transform.Spec;
	    ops: reschema.Op[];
	    rules: masking.Rule[];
	    ruleSetId: string;
	
	    static createFrom(source: any = {}) {
	        return new TransformRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spec = this.convertValues(source["spec"], transform.Spec);
	        this.ops = this.convertValues(source["ops"], reschema.Op);
	        this.rules = this.convertValues(source["rules"], masking.Rule);
	        this.ruleSetId = source["ruleSetId"];
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

export namespace backup {
	
	export class Info {
	    mango3sBackup: number;
	    id: string;
	    // Go type: time
	    created: any;
	    operation: string;
	    description: string;
	    db: string;
	    coll: string;
	    path: string;
	    replaced: number;
	    deleted: number;
	    inserted: number;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mango3sBackup = source["mango3sBackup"];
	        this.id = source["id"];
	        this.created = this.convertValues(source["created"], null);
	        this.operation = source["operation"];
	        this.description = source["description"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.path = source["path"];
	        this.replaced = source["replaced"];
	        this.deleted = source["deleted"];
	        this.inserted = source["inserted"];
	        this.bytes = source["bytes"];
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

export namespace compare {
	
	export class CollectionRow {
	    name: string;
	    inSource: boolean;
	    inTarget: boolean;
	    sourceCount: number;
	    targetCount: number;
	    indexDiffs: string[];
	    compared: boolean;
	    identical: number;
	    changed: number;
	    onlySource: number;
	    onlyTarget: number;
	    fingerprint?: string;
	
	    static createFrom(source: any = {}) {
	        return new CollectionRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.inSource = source["inSource"];
	        this.inTarget = source["inTarget"];
	        this.sourceCount = source["sourceCount"];
	        this.targetCount = source["targetCount"];
	        this.indexDiffs = source["indexDiffs"];
	        this.compared = source["compared"];
	        this.identical = source["identical"];
	        this.changed = source["changed"];
	        this.onlySource = source["onlySource"];
	        this.onlyTarget = source["onlyTarget"];
	        this.fingerprint = source["fingerprint"];
	    }
	}
	export class DBOptions {
	    sourceConnId: string;
	    sourceDb: string;
	    targetConnId: string;
	    targetDb: string;
	    deep: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DBOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceConnId = source["sourceConnId"];
	        this.sourceDb = source["sourceDb"];
	        this.targetConnId = source["targetConnId"];
	        this.targetDb = source["targetDb"];
	        this.deep = source["deep"];
	    }
	}
	export class DBResult {
	    options: DBOptions;
	    collections: CollectionRow[];
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new DBResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.options = this.convertValues(source["options"], DBOptions);
	        this.collections = this.convertValues(source["collections"], CollectionRow);
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
	export class FieldChange {
	    path: string;
	    change: string;
	    source?: string;
	    target?: string;
	
	    static createFrom(source: any = {}) {
	        return new FieldChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.change = source["change"];
	        this.source = source["source"];
	        this.target = source["target"];
	    }
	}
	export class DocDiff {
	    id: string;
	    kind: string;
	    fields: FieldChange[];
	
	    static createFrom(source: any = {}) {
	        return new DocDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.fields = this.convertValues(source["fields"], FieldChange);
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
	export class Endpoint {
	    connId: string;
	    db: string;
	    coll: string;
	
	    static createFrom(source: any = {}) {
	        return new Endpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	    }
	}
	export class Entry {
	    key: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	    }
	}
	
	export class Options {
	    source: Endpoint;
	    target: Endpoint;
	    filter: string;
	    ignoreFields: string[];
	    ignoreFieldOrder: boolean;
	    sampleSize: number;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = this.convertValues(source["source"], Endpoint);
	        this.target = this.convertValues(source["target"], Endpoint);
	        this.filter = source["filter"];
	        this.ignoreFields = source["ignoreFields"];
	        this.ignoreFieldOrder = source["ignoreFieldOrder"];
	        this.sampleSize = source["sampleSize"];
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
	    options: Options;
	    sourceCount: number;
	    targetCount: number;
	    identical: number;
	    changed: number;
	    onlySource: number;
	    onlyTarget: number;
	    samples: DocDiff[];
	    fingerprint: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.options = this.convertValues(source["options"], Options);
	        this.sourceCount = source["sourceCount"];
	        this.targetCount = source["targetCount"];
	        this.identical = source["identical"];
	        this.changed = source["changed"];
	        this.onlySource = source["onlySource"];
	        this.onlyTarget = source["onlyTarget"];
	        this.samples = this.convertValues(source["samples"], DocDiff);
	        this.fingerprint = source["fingerprint"];
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

}

export namespace config {
	
	export class AdvancedConfig {
	    serverSelectionTimeoutSeconds: number;
	    operationTimeoutSeconds: number;
	    heartbeatSeconds: number;
	    localThresholdMs: number;
	    appName: string;
	    readPreference: string;
	    readPreferenceTags: string;
	    maxStalenessSeconds: number;
	    readConcern: string;
	    writeConcernW: string;
	    writeConcernJournal: string;
	    maxPoolSize: number;
	    minPoolSize: number;
	    maxConnecting: number;
	    maxIdleTimeSeconds: number;
	    retryWrites: string;
	    retryReads: string;
	    compressors: string[];
	
	    static createFrom(source: any = {}) {
	        return new AdvancedConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverSelectionTimeoutSeconds = source["serverSelectionTimeoutSeconds"];
	        this.operationTimeoutSeconds = source["operationTimeoutSeconds"];
	        this.heartbeatSeconds = source["heartbeatSeconds"];
	        this.localThresholdMs = source["localThresholdMs"];
	        this.appName = source["appName"];
	        this.readPreference = source["readPreference"];
	        this.readPreferenceTags = source["readPreferenceTags"];
	        this.maxStalenessSeconds = source["maxStalenessSeconds"];
	        this.readConcern = source["readConcern"];
	        this.writeConcernW = source["writeConcernW"];
	        this.writeConcernJournal = source["writeConcernJournal"];
	        this.maxPoolSize = source["maxPoolSize"];
	        this.minPoolSize = source["minPoolSize"];
	        this.maxConnecting = source["maxConnecting"];
	        this.maxIdleTimeSeconds = source["maxIdleTimeSeconds"];
	        this.retryWrites = source["retryWrites"];
	        this.retryReads = source["retryReads"];
	        this.compressors = source["compressors"];
	    }
	}
	export class HostPort {
	    host: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new HostPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	    }
	}
	export class ServerConfig {
	    mode: string;
	    type: string;
	    hosts: HostPort[];
	    replicaSet: string;
	    directConnection: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.type = source["type"];
	        this.hosts = this.convertValues(source["hosts"], HostPort);
	        this.replicaSet = source["replicaSet"];
	        this.directConnection = source["directConnection"];
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
	export class ParsedURI {
	    server: ServerConfig;
	    username: string;
	    hasPassword: boolean;
	    authSource: string;
	    authMechanism: string;
	    tls: boolean;
	    appName: string;
	    readPreference: string;
	    database: string;
	    unmapped: string[];
	
	    static createFrom(source: any = {}) {
	        return new ParsedURI(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = this.convertValues(source["server"], ServerConfig);
	        this.username = source["username"];
	        this.hasPassword = source["hasPassword"];
	        this.authSource = source["authSource"];
	        this.authMechanism = source["authMechanism"];
	        this.tls = source["tls"];
	        this.appName = source["appName"];
	        this.readPreference = source["readPreference"];
	        this.database = source["database"];
	        this.unmapped = source["unmapped"];
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
	export class ToolsConfig {
	    dir: string;
	    paths: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ToolsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.paths = source["paths"];
	    }
	}
	export class ShellConfig {
	    defaultDb: string;
	    initScript: string;
	    runInitScript: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ShellConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultDb = source["defaultDb"];
	        this.initScript = source["initScript"];
	        this.runInitScript = source["runInitScript"];
	    }
	}
	export class ProxyConfig {
	    type: string;
	    host: string;
	    port: number;
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	    }
	}
	export class SSHConfig {
	    enabled: boolean;
	    host: string;
	    port: number;
	    username: string;
	    authMethod: string;
	    privateKeyFile: string;
	    hostKey: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authMethod = source["authMethod"];
	        this.privateKeyFile = source["privateKeyFile"];
	        this.hostKey = source["hostKey"];
	    }
	}
	export class TLSConfig {
	    enabled: boolean;
	    caFile: string;
	    certKeyFile: string;
	    allowInvalidCertificates: boolean;
	    allowInvalidHostnames: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TLSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.caFile = source["caFile"];
	        this.certKeyFile = source["certKeyFile"];
	        this.allowInvalidCertificates = source["allowInvalidCertificates"];
	        this.allowInvalidHostnames = source["allowInvalidHostnames"];
	    }
	}
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
	    folder?: string;
	    server: ServerConfig;
	    authProperties?: string;
	    tls: TLSConfig;
	    ssh: SSHConfig;
	    proxy: ProxyConfig;
	    shell: ShellConfig;
	    tools: ToolsConfig;
	    advanced: AdvancedConfig;
	    storedSecrets?: string[];
	    // Go type: time
	    lastConnectedAt?: any;
	
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
	        this.folder = source["folder"];
	        this.server = this.convertValues(source["server"], ServerConfig);
	        this.authProperties = source["authProperties"];
	        this.tls = this.convertValues(source["tls"], TLSConfig);
	        this.ssh = this.convertValues(source["ssh"], SSHConfig);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.shell = this.convertValues(source["shell"], ShellConfig);
	        this.tools = this.convertValues(source["tools"], ToolsConfig);
	        this.advanced = this.convertValues(source["advanced"], AdvancedConfig);
	        this.storedSecrets = source["storedSecrets"];
	        this.lastConnectedAt = this.convertValues(source["lastConnectedAt"], null);
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
	    defaultViewChosen: boolean;
	    editorFontSize: number;
	    logLevel: string;
	    aiEnabled: boolean;
	    aiModel: string;
	    aiAllowSamples: boolean;
	    aiProvider: string;
	    aiTimeoutSeconds: number;
	    claudeCodePath: string;
	    claudeCodeModel: string;
	    codexPath: string;
	    codexModel: string;
	    localUrl: string;
	    localApi: string;
	    localModel: string;
	    schedulerEnabled: boolean;
	
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
	        this.defaultViewChosen = source["defaultViewChosen"];
	        this.editorFontSize = source["editorFontSize"];
	        this.logLevel = source["logLevel"];
	        this.aiEnabled = source["aiEnabled"];
	        this.aiModel = source["aiModel"];
	        this.aiAllowSamples = source["aiAllowSamples"];
	        this.aiProvider = source["aiProvider"];
	        this.aiTimeoutSeconds = source["aiTimeoutSeconds"];
	        this.claudeCodePath = source["claudeCodePath"];
	        this.claudeCodeModel = source["claudeCodeModel"];
	        this.codexPath = source["codexPath"];
	        this.codexModel = source["codexModel"];
	        this.localUrl = source["localUrl"];
	        this.localApi = source["localApi"];
	        this.localModel = source["localModel"];
	        this.schedulerEnabled = source["schedulerEnabled"];
	    }
	}
	
	

}

export namespace documents {
	
	export class FieldResult {
	    document: model.Document;
	    previous: string;
	    current: string;
	    existed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FieldResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.document = this.convertValues(source["document"], model.Document);
	        this.previous = source["previous"];
	        this.current = source["current"];
	        this.existed = source["existed"];
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
	export class FieldUpdate {
	    connId: string;
	    db: string;
	    coll: string;
	    id: string;
	    path: string[];
	    type: string;
	    text: string;
	    value?: string;
	    expected: string;
	    expectedMissing: boolean;
	    unset: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FieldUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.id = source["id"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.text = source["text"];
	        this.value = source["value"];
	        this.expected = source["expected"];
	        this.expectedMissing = source["expectedMissing"];
	        this.unset = source["unset"];
	    }
	}
	export class FieldValue {
	    exists: boolean;
	    type: string;
	    text: string;
	    expected: string;
	    editable: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new FieldValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.type = source["type"];
	        this.text = source["text"];
	        this.expected = source["expected"];
	        this.editable = source["editable"];
	        this.reason = source["reason"];
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
	    system: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CollectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.readOnly = source["readOnly"];
	        this.system = source["system"];
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

export namespace masking {
	
	export class MethodInfo {
	    id: string;
	    label: string;
	    types: string[];
	    deterministic: boolean;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new MethodInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.types = source["types"];
	        this.deterministic = source["deterministic"];
	        this.description = source["description"];
	    }
	}
	export class Rule {
	    field: string;
	    method: string;
	    keepLast: number;
	    maskChar: string;
	    value: string;
	    maxDays: number;
	    percent: number;
	    onUnsupported: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.field = source["field"];
	        this.method = source["method"];
	        this.keepLast = source["keepLast"];
	        this.maskChar = source["maskChar"];
	        this.value = source["value"];
	        this.maxDays = source["maxDays"];
	        this.percent = source["percent"];
	        this.onUnsupported = source["onUnsupported"];
	    }
	}
	export class RuleSet {
	    id: string;
	    name: string;
	    rules: Rule[];
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new RuleSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.rules = this.convertValues(source["rules"], Rule);
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

export namespace mcpserver {
	
	export class Policy {
	    enabled: boolean;
	    profiles: string[];
	    databases: string[];
	    tools: string[];
	    allowWrites: boolean;
	    maxDocs: number;
	    maxWriteDocs: number;
	    maskRuleSetId: string;
	    auditLog: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Policy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.profiles = source["profiles"];
	        this.databases = source["databases"];
	        this.tools = source["tools"];
	        this.allowWrites = source["allowWrites"];
	        this.maxDocs = source["maxDocs"];
	        this.maxWriteDocs = source["maxWriteDocs"];
	        this.maskRuleSetId = source["maskRuleSetId"];
	        this.auditLog = source["auditLog"];
	    }
	}
	export class ToolInfo {
	    name: string;
	    write: boolean;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.write = source["write"];
	        this.description = source["description"];
	    }
	}

}

export namespace migrate {
	
	export class Column {
	    name: string;
	    dbType: string;
	    nullable: boolean;
	    suggest: string;
	
	    static createFrom(source: any = {}) {
	        return new Column(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dbType = source["dbType"];
	        this.nullable = source["nullable"];
	        this.suggest = source["suggest"];
	    }
	}
	export class ColumnMapping {
	    column: string;
	    field: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new ColumnMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.column = source["column"];
	        this.field = source["field"];
	        this.type = source["type"];
	    }
	}
	export class Embed {
	    child: string;
	    parent: string;
	    childColumns: string[];
	    as: string;
	
	    static createFrom(source: any = {}) {
	        return new Embed(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.child = source["child"];
	        this.parent = source["parent"];
	        this.childColumns = source["childColumns"];
	        this.as = source["as"];
	    }
	}
	export class EmbedReport {
	    child: string;
	    parent: string;
	    as: string;
	    childRows: number;
	    embedded: number;
	    orphans: number;
	    countOk: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EmbedReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.child = source["child"];
	        this.parent = source["parent"];
	        this.as = source["as"];
	        this.childRows = source["childRows"];
	        this.embedded = source["embedded"];
	        this.orphans = source["orphans"];
	        this.countOk = source["countOk"];
	    }
	}
	export class ForeignKey {
	    columns: string[];
	    refTable: string;
	    refColumns: string[];
	
	    static createFrom(source: any = {}) {
	        return new ForeignKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.refTable = source["refTable"];
	        this.refColumns = source["refColumns"];
	    }
	}
	export class TableReport {
	    table: string;
	    collection: string;
	    rows: number;
	    inserted: number;
	    skipped: number;
	    errors: string[];
	    countOk: boolean;
	    sampleChecked: number;
	    sampleMismatches: string[];
	
	    static createFrom(source: any = {}) {
	        return new TableReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = source["table"];
	        this.collection = source["collection"];
	        this.rows = source["rows"];
	        this.inserted = source["inserted"];
	        this.skipped = source["skipped"];
	        this.errors = source["errors"];
	        this.countOk = source["countOk"];
	        this.sampleChecked = source["sampleChecked"];
	        this.sampleMismatches = source["sampleMismatches"];
	    }
	}
	export class Report {
	    runId: string;
	    tables: TableReport[];
	    embeds: EmbedReport[];
	    valid: boolean;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.tables = this.convertValues(source["tables"], TableReport);
	        this.embeds = this.convertValues(source["embeds"], EmbedReport);
	        this.valid = source["valid"];
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
	export class TableMapping {
	    table: string;
	    collection: string;
	    idStrategy: string;
	    columns: ColumnMapping[];
	    embedOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TableMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = source["table"];
	        this.collection = source["collection"];
	        this.idStrategy = source["idStrategy"];
	        this.columns = this.convertValues(source["columns"], ColumnMapping);
	        this.embedOnly = source["embedOnly"];
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
	export class Source {
	    driver: string;
	    dsn: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.driver = source["driver"];
	        this.dsn = source["dsn"];
	    }
	}
	export class Request {
	    source: Source;
	    connId: string;
	    db: string;
	    tables: TableMapping[];
	    embeds: Embed[];
	    onRowError: string;
	    batchSize: number;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = this.convertValues(source["source"], Source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.tables = this.convertValues(source["tables"], TableMapping);
	        this.embeds = this.convertValues(source["embeds"], Embed);
	        this.onRowError = source["onRowError"];
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
	export class Run {
	    id: string;
	    // Go type: time
	    created: any;
	    source: string;
	    connId: string;
	    db: string;
	    createdCollections: string[];
	    state: string;
	    summary: string;
	    report?: Report;
	
	    static createFrom(source: any = {}) {
	        return new Run(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created = this.convertValues(source["created"], null);
	        this.source = source["source"];
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.createdCollections = source["createdCollections"];
	        this.state = source["state"];
	        this.summary = source["summary"];
	        this.report = this.convertValues(source["report"], Report);
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
	
	export class Table {
	    name: string;
	    columns: Column[];
	    primaryKey: string[];
	    foreignKeys: ForeignKey[];
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new Table(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = this.convertValues(source["columns"], Column);
	        this.primaryKey = source["primaryKey"];
	        this.foreignKeys = this.convertValues(source["foreignKeys"], ForeignKey);
	        this.rows = source["rows"];
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
	    via?: string;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverVersion = source["serverVersion"];
	        this.topology = source["topology"];
	        this.latencyMs = source["latencyMs"];
	        this.via = source["via"];
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

export namespace recent {
	
	export class Entry {
	    db: string;
	    coll: string;
	    // Go type: time
	    lastOpened: any;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.lastOpened = this.convertValues(source["lastOpened"], null);
	        this.count = source["count"];
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

export namespace reschema {
	
	export class Op {
	    type: string;
	    field: string;
	    to: string;
	    value: string;
	    onlyIfMissing: boolean;
	    targetType: string;
	    separator: string;
	    fields: string[];
	
	    static createFrom(source: any = {}) {
	        return new Op(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.field = source["field"];
	        this.to = source["to"];
	        this.value = source["value"];
	        this.onlyIfMissing = source["onlyIfMissing"];
	        this.targetType = source["targetType"];
	        this.separator = source["separator"];
	        this.fields = source["fields"];
	    }
	}

}

export namespace scheduler {
	
	export class Retry {
	    maxAttempts: number;
	    backoffSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new Retry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxAttempts = source["maxAttempts"];
	        this.backoffSeconds = source["backoffSeconds"];
	    }
	}
	export class Run {
	    id: string;
	    taskId: string;
	    taskName: string;
	    trigger: string;
	    state: string;
	    attempts: number;
	    errors: string[];
	    result?: string;
	    jobId?: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Run(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.taskId = source["taskId"];
	        this.taskName = source["taskName"];
	        this.trigger = source["trigger"];
	        this.state = source["state"];
	        this.attempts = source["attempts"];
	        this.errors = source["errors"];
	        this.result = source["result"];
	        this.jobId = source["jobId"];
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
	export class Schedule {
	    kind: string;
	    everyMinutes: number;
	    cron: string;
	
	    static createFrom(source: any = {}) {
	        return new Schedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.everyMinutes = source["everyMinutes"];
	        this.cron = source["cron"];
	    }
	}
	export class Task {
	    id: string;
	    name: string;
	    type: string;
	    params: number[];
	    schedule: Schedule;
	    enabled: boolean;
	    retry: Retry;
	    misfire: string;
	    approvedHash: string;
	    // Go type: time
	    approvedAt?: any;
	    // Go type: time
	    nextRun: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.params = source["params"];
	        this.schedule = this.convertValues(source["schedule"], Schedule);
	        this.enabled = source["enabled"];
	        this.retry = this.convertValues(source["retry"], Retry);
	        this.misfire = source["misfire"];
	        this.approvedHash = source["approvedHash"];
	        this.approvedAt = this.convertValues(source["approvedAt"], null);
	        this.nextRun = this.convertValues(source["nextRun"], null);
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
	export class TaskType {
	    id: string;
	    label: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskType(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.description = source["description"];
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
	    pageable: boolean;
	    page: number;
	    pageSize: number;
	
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
	        this.pageable = source["pageable"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
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
	export class QuerySpec {
	    kind: string;
	    db: string;
	    coll: string;
	    filter: string;
	    projection: string;
	    sort: string;
	    skip: number;
	    limit: number;
	    stages: aggregate.Stage[];
	
	    static createFrom(source: any = {}) {
	        return new QuerySpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.filter = source["filter"];
	        this.projection = source["projection"];
	        this.sort = source["sort"];
	        this.skip = source["skip"];
	        this.limit = source["limit"];
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

export namespace syncer {
	
	export class Counts {
	    inserts: number;
	    replaces: number;
	    deletes: number;
	
	    static createFrom(source: any = {}) {
	        return new Counts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inserts = source["inserts"];
	        this.replaces = source["replaces"];
	        this.deletes = source["deletes"];
	    }
	}
	export class Options {
	    insert: boolean;
	    replace: boolean;
	    delete: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.insert = source["insert"];
	        this.replace = source["replace"];
	        this.delete = source["delete"];
	    }
	}
	export class Outcome {
	    planned: Counts;
	    applied: Counts;
	    skipped: Counts;
	    skippedIds: string[];
	    matchesPreview: boolean;
	    backupPath: string;
	    verifiedEqual: boolean;
	    remaining: Counts;
	    postFingerprint: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Outcome(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.planned = this.convertValues(source["planned"], Counts);
	        this.applied = this.convertValues(source["applied"], Counts);
	        this.skipped = this.convertValues(source["skipped"], Counts);
	        this.skippedIds = source["skippedIds"];
	        this.matchesPreview = source["matchesPreview"];
	        this.backupPath = source["backupPath"];
	        this.verifiedEqual = source["verifiedEqual"];
	        this.remaining = this.convertValues(source["remaining"], Counts);
	        this.postFingerprint = source["postFingerprint"];
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
	export class Preview {
	    planId: string;
	    target: string;
	    source: string;
	    inserts: number;
	    replaces: number;
	    deletes: number;
	    samples: compare.DocDiff[];
	    confirmText: string;
	    expiresInSec: number;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.planId = source["planId"];
	        this.target = source["target"];
	        this.source = source["source"];
	        this.inserts = source["inserts"];
	        this.replaces = source["replaces"];
	        this.deletes = source["deletes"];
	        this.samples = this.convertValues(source["samples"], compare.DocDiff);
	        this.confirmText = source["confirmText"];
	        this.expiresInSec = source["expiresInSec"];
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

export namespace tools {
	
	export class Info {
	    name: string;
	    purpose: string;
	    path: string;
	    source: string;
	    version: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.purpose = source["purpose"];
	        this.path = source["path"];
	        this.source = source["source"];
	        this.version = source["version"];
	        this.status = source["status"];
	        this.message = source["message"];
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

export namespace transform {
	
	export class Check {
	    name: string;
	    ok: boolean;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Check(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ok = source["ok"];
	        this.detail = source["detail"];
	    }
	}
	export class Sample {
	    id: string;
	    before: model.Document;
	    after: model.Document;
	    changed: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Sample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.before = this.convertValues(source["before"], model.Document);
	        this.after = this.convertValues(source["after"], model.Document);
	        this.changed = source["changed"];
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
	export class Preview {
	    total: number;
	    scanned: number;
	    changed: number;
	    errors: number;
	    errorSamples: string[];
	    samples: Sample[];
	    estimated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.scanned = source["scanned"];
	        this.changed = source["changed"];
	        this.errors = source["errors"];
	        this.errorSamples = source["errorSamples"];
	        this.samples = this.convertValues(source["samples"], Sample);
	        this.estimated = source["estimated"];
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
	    mode: string;
	    scanned: number;
	    changed: number;
	    written: number;
	    skipped: number;
	    drift: number;
	    errorSamples: string[];
	    backupPath?: string;
	    validation: Check[];
	    valid: boolean;
	    durationMs: number;
	    target: string;
	    createdTarget: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.scanned = source["scanned"];
	        this.changed = source["changed"];
	        this.written = source["written"];
	        this.skipped = source["skipped"];
	        this.drift = source["drift"];
	        this.errorSamples = source["errorSamples"];
	        this.backupPath = source["backupPath"];
	        this.validation = this.convertValues(source["validation"], Check);
	        this.valid = source["valid"];
	        this.durationMs = source["durationMs"];
	        this.target = source["target"];
	        this.createdTarget = source["createdTarget"];
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
	
	export class Spec {
	    connId: string;
	    db: string;
	    coll: string;
	    filter: string;
	    mode: string;
	    targetColl: string;
	    onError: string;
	
	    static createFrom(source: any = {}) {
	        return new Spec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connId = source["connId"];
	        this.db = source["db"];
	        this.coll = source["coll"];
	        this.filter = source["filter"];
	        this.mode = source["mode"];
	        this.targetColl = source["targetColl"];
	        this.onError = source["onError"];
	    }
	}

}

