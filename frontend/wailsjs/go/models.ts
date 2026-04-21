export namespace indexer {
	
	export class Entry {
	    path: string;
	    name: string;
	    dir: string;
	    ext: string;
	    isDir: boolean;
	    size: number;
	    // Go type: time
	    modTime: any;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.dir = source["dir"];
	        this.ext = source["ext"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.modTime = this.convertValues(source["modTime"], null);
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
	export class SearchRequest {
	    query: string;
	    limit: number;
	    typeFilter: string;
	    folderPrefix: string;
	    extension: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.limit = source["limit"];
	        this.typeFilter = source["typeFilter"];
	        this.folderPrefix = source["folderPrefix"];
	        this.extension = source["extension"];
	    }
	}
	export class Status {
	    state: string;
	    roots: string[];
	    indexedFiles: number;
	    indexedDirs: number;
	    errors: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    completedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.roots = source["roots"];
	        this.indexedFiles = source["indexedFiles"];
	        this.indexedDirs = source["indexedDirs"];
	        this.errors = source["errors"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
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

