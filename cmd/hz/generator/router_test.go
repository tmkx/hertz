/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func Test_checkDupRegister(t *testing.T) {
	type args struct {
		file      []byte
		insertReg string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "dup tab",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app\"\n)\n\nfunc register() {\n\tapp.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: true,
		},
		{
			name: "dup space",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app\"\n)\n\nfunc register() {\n   app.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: true,
		},
		{
			name: "not dup prefix",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app_2\"\n)\n\nfunc register() {\n\tapp_2.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: false,
		},
		{
			name: "not dup subfix",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/xapp\"\n)\n\nfunc register() {\n xapp.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkDupRegister(tt.args.file, tt.args.insertReg); got != tt.want {
				t.Errorf("checkDupRegister() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateMiddlewareRegCustomNames(t *testing.T) {
	for _, custom := range []bool{false, true} {
		name := "{{.MiddleWare}}Mw"
		handlerName := "_getllmresponseMw"
		duplicateHandlerName := "_getllmresponse0Mw"
		groupName := "_collectionuidMw"
		info := &Template{}
		if custom {
			name = "{{if ne .MiddleWare \"root\"}}_{{end}}{{untitle .RawName}}Mw"
			handlerName = "_getLLMResponseMw"
			duplicateHandlerName = "_getLLMResponse0Mw"
			groupName = "_collectionUidMw"
			info.UpdateBehavior.InsertKey = "func " + name + "("
		}
		gen := &HttpPackageGenerator{}
		gen.tplsInfo = map[string]*Template{middlewareTplName: {}, middlewareSingleTplName: info}
		gen.tpls = map[string]*template.Template{
			middlewareSingleTplName: template.Must(template.New("single").Funcs(funcMap).Parse("\nfunc " + name + "() []app.HandlerFunc { return nil }\n")),
		}
		path := filepath.Join(t.TempDir(), "middleware.go")
		// The handler exists, but renaming :uid to :collectionUid adds a group.
		source := "package service\nfunc _uidMw() {}\nfunc " + handlerName + "() []app.HandlerFunc { return authenticate() }\n"
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		duplicateHandler := &RouterNode{
			Path:              "/duplicate",
			HandlerMiddleware: "_getllmresponse0",
			Handler:           "handler.GetLLMResponse",
		}
		router := Router{Router: &RouterNode{
			Path: "/:collectionUid", GroupMiddleware: "_collectionuid", HandlerMiddleware: "_getllmresponse",
			Handler: "handler.GetLLMResponse", Children: childrenRouterInfo{duplicateHandler},
		}}
		for i := 0; i < 2; i++ {
			if err := gen.updateMiddlewareReg(router, middlewareTplName, path); err != nil {
				t.Fatal(err)
			}
			got := gen.files[len(gen.files)-1].Content
			if !strings.HasPrefix(got, source) {
				t.Fatal("existing middleware changed")
			}
			for _, fn := range []string{groupName, handlerName, duplicateHandlerName} {
				if strings.Count(got, "func "+fn+"()") != 1 {
					t.Fatalf("custom=%v: missing or duplicate %s: %s", custom, fn, got)
				}
			}
			if err := os.WriteFile(path, []byte(got), 0600); err != nil {
				t.Fatal(err)
			}
		}
		router.Router.Handler = "handler.NewMethod"
		router.Router.HandlerMiddleware = "_newmethod"
		if err := gen.updateMiddlewareReg(router, middlewareTplName, path); err != nil {
			t.Fatal(err)
		}
		want := "_newmethodMw"
		if custom {
			want = "_newMethodMw"
		}
		if !strings.Contains(gen.files[len(gen.files)-1].Content, "func "+want+"()") {
			t.Fatal("new method middleware missing")
		}
	}
}

func TestRawGroupName(t *testing.T) {
	for _, tc := range []struct{ path, middleware, want string }{
		{"/", "root", "root"},
		{"/:collectionUid", "_collectionuid", "collectionUid"},
		{"/:collectionUid", "_collectionuid0", "collectionUid0"},
		{"/userID", "_userid", "userID"},
		{"/*filePath", "__2afilepath", "filePath"},
	} {
		node := &RouterNode{Path: tc.path, GroupMiddleware: tc.middleware}
		if got := node.RawGroupName(); got != tc.want {
			t.Errorf("RawGroupName(%q, %q) = %q, want %q", tc.path, tc.middleware, got, tc.want)
		}
	}
}

func TestRawHandlerName(t *testing.T) {
	for _, tc := range []struct{ handler, middleware, want string }{
		{"handler.GetLLMResponse", "", "GetLLMResponse"},
		{"handler.GetLLMResponse", "_getllmresponse", "GetLLMResponse"},
		{"handler.GetLLMResponse", "_getllmresponse0", "GetLLMResponse0"},
		{"handler.GetLLMResponse", "_GetLLMResponse0", "GetLLMResponse0"},
	} {
		node := &RouterNode{Handler: tc.handler, HandlerMiddleware: tc.middleware}
		if got := node.RawHandlerName(); got != tc.want {
			t.Errorf("RawHandlerName(%q, %q) = %q, want %q",
				tc.handler, tc.middleware, got, tc.want)
		}
	}
}
