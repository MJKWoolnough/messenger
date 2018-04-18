package main

import (
	"fmt"
	"time"

	"github.com/MJKWoolnough/errors"
	"github.com/robertkrimen/otto"
	"github.com/robertkrimen/otto/registry"
	xmlpath "gopkg.in/xmlpath.v2"
)

func init() {
	registry.Register(runtime)
}

func runtime() string {
	return `
var setUserData = function() {},
setDTSGToken = function() {},
setSiteData = function() {},
setCookieData = function() {},
setSprinkleName = function() {},
setValue = function() {},
requireObj = {
	lastID: "",
        guard: function(a) {
                return a;
        },
	handle: function() {},
        handleDefines: function(data, ) {
		if (this.lastID === "ServerJSDefine") {
			for (var i = 0; i < data.length; i++) {
				var o = data[i][2];
				switch (data[i][0]) {
				case "CurrentUserInitialData":
					setUserData(o["USER_ID"], o["NAME"], o["SHORT_NAME"]);
					break;
				case "DTSGInitialData":
					setDTSGToken(o["token"]);
					break;
				case "SiteData":
					setSiteData(o["server_revision"], o["pkg_cohort_key"], o["pkg_cohort"], o["be_key"], o["be_mode"]);
					break;
				case "SprinkleConfig":
					setSprinkleName(o["param_name"]);
					break;
				}
			}
		}
	},
        handleServerJS: function(a) {
               setValue(JSON.stringify(a));
        }
},
requireConstructor = function(){},
requireLazy = function() {},
require = function(id) {
	requireObj.lastID = id;
	switch (id) {
	case "ServerJS":
		return requireConstructor;
	case "BigPipe":
		return bigPipeConstructor;
	default:
	        return requireObj;
	}
},
bigPipe = {
        onPageletArrive: function (data) {
		if (data && data["jsmods"] && data["jsmods"]["require"]) {
                	var r = data["jsmods"]["require"];
			for (var i = 0; i < r.length; i++) {
				if (r[i][0] === "CookieCore") {
					setCookieValue(r[i][3][1]);
					break;
				}
			}
		}
        },
	setPageID: function() {},
	beforePageletArrive: function() {},
},
bigPipeConstructor = function() {},
Document = {},
Element = {},
HTMLElement = {},
HTMLInputElement = {},
HTMLTextAreaElement = {},
Range = {},
MouseEvent = {},
CSSStyleDeclaration = {},
window = {
	Document: {},
	Element: {},
	HTMLElement: {},
	HTMLInputElement: {},
	HTMLTextAreaElement: {},
	Range: {},
	MouseEvent: {},
	CSSStyleDeclaration: {},
};
requireConstructor.prototype = requireObj;
bigPipeConstructor.prototype = bigPipe;
`
}

type jsFuncs map[string]func(call otto.FunctionCall) otto.Value

const jsHalt errors.Error = "took too long"

func runCode(funcs jsFuncs, scripts *xmlpath.Iter) (err error) {
	defer func() {
		if errp := recover(); errp != nil {
			if errp == jsHalt {
				err = jsHalt
			} else {
				panic(errp)
			}
		}
	}()
	vm := otto.New()
	for name, fn := range funcs {
		vm.Set(name, fn)
	}
	vm.Interrupt = make(chan func(), 1)
	reset := make(chan bool, 1)
	const resetTime = time.Second
	go func() {
		timer := time.NewTimer(resetTime)
		for {
			select {
			case <-timer.C:
				vm.Interrupt <- func() {
					panic(jsHalt)
				}
			case cont := <-reset:
				if !cont {
					timer.Stop()
					return
				}
				timer.Reset(resetTime)
			}
		}
	}()
	for scripts.Next() {
		scr := scripts.Node().String()
		_, err = vm.Run(scr)
		reset <- true
		if err != nil {
			fmt.Println(scr)
			if strerr, ok := err.(interface {
				String() string
			}); ok {
				return errors.Error(strerr.String())
			}
			return err
		}
	}
	close(reset)
	close(vm.Interrupt)

	return nil
}
