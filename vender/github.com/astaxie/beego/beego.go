// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package beego

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// VERSION represent beego web framework version.
	VERSION = "1.10.1"

	// DEV is for develop
	DEV = "dev"
	// PROD is for production
	PROD = "prod"
)

// Map shortcut
type M map[string]interface{}

// Hook function to run
type hookfunc func() error

var (
	hooks = make([]hookfunc, 0) //hook function slice to store the hookfunc
)

// AddAPPStartHook is used to register the hookfunc
// The hookfuncs will run in beego.Run()
// such as initiating session , starting middleware , building template, starting admin control and so on.
func AddAPPStartHook(hf ...hookfunc) {
	hooks = append(hooks, hf...)
}

// Run beego application.
// beego.Run() default run on HttpPort
// beego.Run("localhost")
// beego.Run(":8089")
// beego.Run("127.0.0.1:8089")
func Run(params ...string) {

	InitBeforeHTTPRun()

	if len(params) > 0 && params[0] != "" {
		strs := strings.Split(params[0], ":")
		if len(strs) > 0 && strs[0] != "" {
			BConfig.Listen.HTTPAddr = strs[0]
		}
		if len(strs) > 1 && strs[1] != "" {
			BConfig.Listen.HTTPPort, _ = strconv.Atoi(strs[1])
		}

		BConfig.Listen.Domains = params
	}

	BeeApp.Run()
}

// RunWithMiddleWares Run beego application with middlewares.
func RunWithMiddleWares(addr string, mws ...MiddleWare) {
	InitBeforeHTTPRun()

	strs := strings.Split(addr, ":")
	if len(strs) > 0 && strs[0] != "" {
		BConfig.Listen.HTTPAddr = strs[0]
		BConfig.Listen.Domains = []string{strs[0]}
	}
	if len(strs) > 1 && strs[1] != "" {
		BConfig.Listen.HTTPPort, _ = strconv.Atoi(strs[1])
	}

	BeeApp.Run(mws...)
}

func InitBeforeHTTPRun() {
	//init hooks
	AddAPPStartHook(
		registerMime,
		registerDefaultErrorHandler,
		registerSession,
		registerTemplate,
		registerAdmin,
		registerGzip,
	)

	for _, hk := range hooks {
		if err := hk(); err != nil {
			panic(err)
		}
	}
}

// TestBeegoInit is for test package init
func TestBeegoInit(ap string) {
	path := filepath.Join(ap, "conf", "app.conf")
	os.Chdir(ap)
	InitBeegoBeforeTest(path)
}

// InitBeegoBeforeTest is for test package init
func InitBeegoBeforeTest(appConfigPath string) {
	if err := LoadAppConfig(appConfigProvider, appConfigPath); err != nil {
		panic(err)
	}
	BConfig.RunMode = "test"
	InitBeforeHTTPRun()
}
