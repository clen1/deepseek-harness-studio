export namespace main {
	
	export class Config {
	    registryId: string;
	    registryUrl: string;
	    proxyMode: string;
	    proxyUrl: string;
	    autoOpen: boolean;
	    autoStart: boolean;
	    installChannel: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.registryId = source["registryId"];
	        this.registryUrl = source["registryUrl"];
	        this.proxyMode = source["proxyMode"];
	        this.proxyUrl = source["proxyUrl"];
	        this.autoOpen = source["autoOpen"];
	        this.autoStart = source["autoStart"];
	        this.installChannel = source["installChannel"];
	    }
	}
	export class JobState {
	    type: string;
	    phase: string;
	    title: string;
	    message: string;
	    progress: number;
	    startedAt: number;
	    finishedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new JobState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.phase = source["phase"];
	        this.title = source["title"];
	        this.message = source["message"];
	        this.progress = source["progress"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	    }
	}
	export class LogEntry {
	    id: number;
	    time: string;
	    source: string;
	    level: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.source = source["source"];
	        this.level = source["level"];
	        this.text = source["text"];
	    }
	}
	export class RegistryPreset {
	    id: string;
	    name: string;
	    url: string;
	    description: string;
	    recommended: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RegistryPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.description = source["description"];
	        this.recommended = source["recommended"];
	    }
	}
	export class RegistryResult {
	    id: string;
	    ok: boolean;
	    latencyMs: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RegistryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ok = source["ok"];
	        this.latencyMs = source["latencyMs"];
	        this.message = source["message"];
	    }
	}
	export class Status {
	    installed: boolean;
	    version: string;
	    nodeReady: boolean;
	    nodeVersion: string;
	    service: string;
	    servicePid: number;
	    platform: string;
	    architecture: string;
	    installPath: string;
	    serviceUrl: string;
	    job: JobState;
	    config: Config;
	    registries: RegistryPreset[];
	    logs: LogEntry[];
	    downloadSupport: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.nodeReady = source["nodeReady"];
	        this.nodeVersion = source["nodeVersion"];
	        this.service = source["service"];
	        this.servicePid = source["servicePid"];
	        this.platform = source["platform"];
	        this.architecture = source["architecture"];
	        this.installPath = source["installPath"];
	        this.serviceUrl = source["serviceUrl"];
	        this.job = this.convertValues(source["job"], JobState);
	        this.config = this.convertValues(source["config"], Config);
	        this.registries = this.convertValues(source["registries"], RegistryPreset);
	        this.logs = this.convertValues(source["logs"], LogEntry);
	        this.downloadSupport = source["downloadSupport"];
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

