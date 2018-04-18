package main

import "github.com/robertkrimen/otto/registry"

func init() {
	registry.Register(runtime)
}

func runtime() string {
	return `
var requireObj = {
	lastID: "",
        guard: function(a) {
                return a;
        },
	handle: function() {},
        handleDefines: function() {
		
	},
        handleServerJS: function(a) {
               setValue(JSON.stringify(a));
        }
},
requireConstructor = function(){},
requireLazy = function() {},
require = function(id) {
	requireObj.lastID = id;
	if (id === "ServerJS") {
		return requireConstructor;
	} else {
	        return requireObj;
	}
},
bigPipe = {
        onPageletArrive: function (data) {
		if (data && data["jsmods"] && data["jsmods"]["require"]) {
                	var r = data["jsmods"]["require"];
			for (var i = 0; i < r.length; i++) {
				if (r[i][0] === "CookieCore") {
					setValue(r[i][3][1]);
					break;
				}
			}
		}
        },
	setPageID: function() {},
	beforePageletArrive: function() {},
},
bigPipeConstructor = function() {},
CavalryLogger = {
	setPageID: function() {}
},
window = {};
requireConstructor.prototype = requireObj;
bigPipeConstructor.prototype = bigPipe;
`
}
