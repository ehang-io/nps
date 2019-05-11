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

package authz

import (
	"github.com/casbin/casbin"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/context"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/plugins/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testRequest(t *testing.T, handler *beego.ControllerRegister, user string, path string, method string, code int) {
	r, _ := http.NewRequest(method, path, nil)
	r.SetBasicAuth(user, "123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != code {
		t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, w.Code, code)
	}
}

func TestBasic(t *testing.T) {
	handler := beego.NewControllerRegister()

	handler.InsertFilter("*", beego.BeforeRouter, auth.Basic("alice", "123"))
	handler.InsertFilter("*", beego.BeforeRouter, NewAuthorizer(casbin.NewEnforcer("authz_model.conf", "authz_policy.csv")))

	handler.Any("*", func(ctx *context.Context) {
		ctx.Output.SetStatus(200)
	})

	testRequest(t, handler, "alice", "/dataset1/resource1", "GET", 200)
	testRequest(t, handler, "alice", "/dataset1/resource1", "POST", 200)
	testRequest(t, handler, "alice", "/dataset1/resource2", "GET", 200)
	testRequest(t, handler, "alice", "/dataset1/resource2", "POST", 403)
}

func TestPathWildcard(t *testing.T) {
	handler := beego.NewControllerRegister()

	handler.InsertFilter("*", beego.BeforeRouter, auth.Basic("bob", "123"))
	handler.InsertFilter("*", beego.BeforeRouter, NewAuthorizer(casbin.NewEnforcer("authz_model.conf", "authz_policy.csv")))

	handler.Any("*", func(ctx *context.Context) {
		ctx.Output.SetStatus(200)
	})

	testRequest(t, handler, "bob", "/dataset2/resource1", "GET", 200)
	testRequest(t, handler, "bob", "/dataset2/resource1", "POST", 200)
	testRequest(t, handler, "bob", "/dataset2/resource1", "DELETE", 200)
	testRequest(t, handler, "bob", "/dataset2/resource2", "GET", 200)
	testRequest(t, handler, "bob", "/dataset2/resource2", "POST", 403)
	testRequest(t, handler, "bob", "/dataset2/resource2", "DELETE", 403)

	testRequest(t, handler, "bob", "/dataset2/folder1/item1", "GET", 403)
	testRequest(t, handler, "bob", "/dataset2/folder1/item1", "POST", 200)
	testRequest(t, handler, "bob", "/dataset2/folder1/item1", "DELETE", 403)
	testRequest(t, handler, "bob", "/dataset2/folder1/item2", "GET", 403)
	testRequest(t, handler, "bob", "/dataset2/folder1/item2", "POST", 200)
	testRequest(t, handler, "bob", "/dataset2/folder1/item2", "DELETE", 403)
}

func TestRBAC(t *testing.T) {
	handler := beego.NewControllerRegister()

	handler.InsertFilter("*", beego.BeforeRouter, auth.Basic("cathy", "123"))
	e := casbin.NewEnforcer("authz_model.conf", "authz_policy.csv")
	handler.InsertFilter("*", beego.BeforeRouter, NewAuthorizer(e))

	handler.Any("*", func(ctx *context.Context) {
		ctx.Output.SetStatus(200)
	})

	// cathy can access all /dataset1/* resources via all methods because it has the dataset1_admin role.
	testRequest(t, handler, "cathy", "/dataset1/item", "GET", 200)
	testRequest(t, handler, "cathy", "/dataset1/item", "POST", 200)
	testRequest(t, handler, "cathy", "/dataset1/item", "DELETE", 200)
	testRequest(t, handler, "cathy", "/dataset2/item", "GET", 403)
	testRequest(t, handler, "cathy", "/dataset2/item", "POST", 403)
	testRequest(t, handler, "cathy", "/dataset2/item", "DELETE", 403)

	// delete all roles on user cathy, so cathy cannot access any resources now.
	e.DeleteRolesForUser("cathy")

	testRequest(t, handler, "cathy", "/dataset1/item", "GET", 403)
	testRequest(t, handler, "cathy", "/dataset1/item", "POST", 403)
	testRequest(t, handler, "cathy", "/dataset1/item", "DELETE", 403)
	testRequest(t, handler, "cathy", "/dataset2/item", "GET", 403)
	testRequest(t, handler, "cathy", "/dataset2/item", "POST", 403)
	testRequest(t, handler, "cathy", "/dataset2/item", "DELETE", 403)
}
