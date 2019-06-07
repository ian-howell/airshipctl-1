// +build ignore

package main

import (
	"bytes"
	"go/format"
	"io"
	"log"
	"os"
	"text/template"
)

func main() {
	typeList := []string{
		// Expressions:
		"ArrayType",
		"BadExpr",
		"BasicLit",
		"BinaryExpr",
		"CallExpr",
		"ChanType",
		"CompositeLit",
		"Ellipsis",
		"FuncLit",
		"FuncType",
		"Ident",
		"IndexExpr",
		"InterfaceType",
		"KeyValueExpr",
		"MapType",
		"ParenExpr",
		"SelectorExpr",
		"SliceExpr",
		"StarExpr",
		"StructType",
		"TypeAssertExpr",
		"UnaryExpr",

		// Statements:
		"AssignStmt",
		"BadStmt",
		"BlockStmt",
		"BranchStmt",
		"CaseClause",
		"CommClause",
		"DeclStmt",
		"DeferStmt",
		"EmptyStmt",
		"ExprStmt",
		"ForStmt",
		"GoStmt",
		"IfStmt",
		"IncDecStmt",
		"LabeledStmt",
		"RangeStmt",
		"ReturnStmt",
		"SelectStmt",
		"SendStmt",
		"SwitchStmt",
		"TypeSwitchStmt",

		// Others:
		"Comment",
		"CommentGroup",
		"FieldList",
		"File",
		"Package",
	}

	astcastFile, err := os.Create("astcast.go")
	if err != nil {
		log.Fatal(err)
	}
	writeCode(astcastFile, typeList)
	astcastTestFile, err := os.Create("astcast_test.go")
	if err != nil {
		log.Fatal(err)
	}
	writeTests(astcastTestFile, typeList)
}

func generateCode(tmplText string, typeList []string) []byte {
	tmpl := template.Must(template.New("code").Parse(tmplText))
	var code bytes.Buffer
	tmpl.Execute(&code, typeList)
	prettyCode, err := format.Source(code.Bytes())
	if err != nil {
		panic(err)
	}
	return prettyCode
}

func writeCode(output io.Writer, typeList []string) {
	code := generateCode(`// Code generated by astcast_generate.go; DO NOT EDIT

// Package astcast wraps type assertion operations in such way that you don't have
// to worry about nil pointer results anymore.
package astcast

import (
	"go/ast"
)

// A set of sentinel nil-like values that are returned
// by all "casting" functions in case of failed type assertion.
var (
{{ range . }}
Nil{{.}} = &ast.{{.}}{}
{{- end }}
)

{{ range . }}
// To{{.}} returns x as a non-nil *ast.{{.}}.
// If ast.Node actually has such dynamic type, the result is
// identical to normal type assertion. In case if it has
// different type, the returned value is Nil{{.}}.
func To{{.}}(x ast.Node) *ast.{{.}} {
	if x, ok := x.(*ast.{{.}}); ok {
		return x
	}
	return Nil{{.}}
}
{{ end }}
`, typeList)
	output.Write(code)
}

func writeTests(output io.Writer, typeList []string) {
	code := generateCode(`// Code generated by astcast_generate.go; DO NOT EDIT

package astcast

import (
	"go/ast"
	"testing"
)

{{ range . }}
func TestTo{{.}}(t *testing.T) {
	// Test successfull cast.
	if x := To{{.}}(&ast.{{.}}{}); x == Nil{{.}} || x == nil {
		t.Error("expected successfull cast, got nil")
	}
	// Test nil cast.
	if x := To{{.}}(nil); x != Nil{{.}} {
		t.Error("nil node didn't resulted in a sentinel value return")
	}
	// Test unsuccessfull cast.
	{{- if (eq . "Ident") }}
		if x := To{{.}}(&ast.CallExpr{}); x != Nil{{.}} || x == nil {
			t.Errorf("expected unsuccessfull cast to return nil sentinel")
		}
	{{- else }}
		if x := To{{.}}(&ast.Ident{}); x != Nil{{.}} || x == nil {
			t.Errorf("expected unsuccessfull cast to return nil sentinel")
		}
	{{- end }}
}
{{ end }}
`, typeList)
	output.Write(code)
}
