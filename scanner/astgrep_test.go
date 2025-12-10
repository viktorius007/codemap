package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAstGrepAnalyzer(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}

	// Test Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(goFile, []byte(`package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
}

func helper(x int) int {
	return x * 2
}
`), 0644)

	analysis, err := analyzer.AnalyzeFile(goFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if analysis == nil {
		t.Fatal("Expected analysis, got nil")
	}

	// Check functions
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["main"] {
		t.Error("Expected main function")
	}
	if !funcs["helper"] {
		t.Error("Expected helper function")
	}

	// Check imports
	imports := make(map[string]bool)
	for _, i := range analysis.Imports {
		imports[i] = true
	}
	if !imports["fmt"] {
		t.Errorf("Expected fmt import, got: %v", analysis.Imports)
	}
	if !imports["os"] {
		t.Errorf("Expected os import, got: %v", analysis.Imports)
	}
}

func TestAstGrepPython(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}

	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")
	os.WriteFile(pyFile, []byte(`import os
from pathlib import Path

def hello(name):
    print(f"Hello {name}")

def greet():
    pass
`), 0644)

	analysis, err := analyzer.AnalyzeFile(pyFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["hello"] {
		t.Errorf("Expected hello function, got: %v", analysis.Functions)
	}
	if !funcs["greet"] {
		t.Errorf("Expected greet function, got: %v", analysis.Functions)
	}

	imports := make(map[string]bool)
	for _, i := range analysis.Imports {
		imports[i] = true
	}
	if !imports["os"] {
		t.Errorf("Expected os import, got: %v", analysis.Imports)
	}
}

func TestExtractStructName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"type Foo struct { Name string }", "go", "Foo"},
		{"type Bar struct{}", "go", "Bar"},
		{"type MyStruct struct {\n\tField int\n}", "go", "MyStruct"},
		{"", "go", ""},
	}
	for _, tt := range tests {
		got := extractStructName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractStructName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractInterfaceName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"type Reader interface { Read(p []byte) (n int, err error) }", "go", "Reader"},
		{"type Writer interface{}", "go", "Writer"},
		{"", "go", ""},
	}
	for _, tt := range tests {
		got := extractInterfaceName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractInterfaceName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractMethodName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"func (s *Server) Start() error { }", "go", "Start"},
		{"func (c Client) Do() { }", "go", "Do"},
		{"func (r *Reader) Read(p []byte) (n int, err error) { }", "go", "Read"},
		{"", "go", ""},
	}
	for _, tt := range tests {
		got := extractMethodName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractMethodName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractConstantNames(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected []string
	}{
		{"const Foo = 1", "go", []string{"Foo"}},
		{"const Bar string = \"hello\"", "go", []string{"Bar"}},
		{"const (\n\tA = 1\n\tB = 2\n)", "go", []string{"A", "B"}},
		{"const (\n\tX\n\tY\n\tZ\n)", "go", []string{"X", "Y", "Z"}},
		{"", "go", []string{}},
	}
	for _, tt := range tests {
		got := extractConstantNames(tt.input, tt.lang)
		if len(got) != len(tt.expected) {
			t.Errorf("extractConstantNames(%q, %q) = %v, want %v",
				tt.input, tt.lang, got, tt.expected)
			continue
		}
		for i, name := range got {
			if name != tt.expected[i] {
				t.Errorf("extractConstantNames(%q, %q)[%d] = %q, want %q",
					tt.input, tt.lang, i, name, tt.expected[i])
			}
		}
	}
}

func TestExtractTypeName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"type ID int", "go", "ID"},
		{"type StringMap map[string]string", "go", "StringMap"},
		{"", "go", ""},
	}
	for _, tt := range tests {
		got := extractTypeName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractTypeName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractVarNames(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected []string
	}{
		{"var globalVar = \"test\"", "go", []string{"globalVar"}},
		{"var count int = 10", "go", []string{"count"}},
		{"var name string", "go", []string{"name"}},
		{"var (\n\tmultiVar1 = 1\n\tmultiVar2 = 2\n)", "go", []string{"multiVar1", "multiVar2"}},
		{"var (\n\tX int\n\tY string\n)", "go", []string{"X", "Y"}},
		{"", "go", []string{}},
	}
	for _, tt := range tests {
		got := extractVarNames(tt.input, tt.lang)
		if len(got) != len(tt.expected) {
			t.Errorf("extractVarNames(%q, %q) = %v, want %v",
				tt.input, tt.lang, got, tt.expected)
			continue
		}
		for i, name := range got {
			if name != tt.expected[i] {
				t.Errorf("extractVarNames(%q, %q)[%d] = %q, want %q",
					tt.input, tt.lang, i, name, tt.expected[i])
			}
		}
	}
}

func TestGoSymbolsExtraction(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(goFile, []byte(`package main

import "fmt"

const Version = "1.0"

const (
	Red = iota
	Green
	Blue
)

var globalVar = "test"

var (
	multiVar1 = 1
	multiVar2 = 2
)

type MyStruct struct {
	Name string
}

type MyInterface interface {
	Do()
}

type ID int

func main() {
	fmt.Println("hello")
}

func (m *MyStruct) Do() {
	// method implementation
}
`), 0644)

	analysis, err := analyzer.AnalyzeFile(goFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Check functions
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["main"] {
		t.Error("Expected main function")
	}

	// Check structs
	structs := make(map[string]bool)
	for _, s := range analysis.Structs {
		structs[s] = true
	}
	if !structs["MyStruct"] {
		t.Errorf("Expected MyStruct struct, got: %v", analysis.Structs)
	}

	// Check interfaces
	interfaces := make(map[string]bool)
	for _, i := range analysis.Interfaces {
		interfaces[i] = true
	}
	if !interfaces["MyInterface"] {
		t.Errorf("Expected MyInterface interface, got: %v", analysis.Interfaces)
	}

	// Check methods
	methods := make(map[string]bool)
	for _, m := range analysis.Methods {
		methods[m] = true
	}
	if !methods["Do"] {
		t.Errorf("Expected Do method, got: %v", analysis.Methods)
	}

	// Check constants
	constants := make(map[string]bool)
	for _, c := range analysis.Constants {
		constants[c] = true
	}
	if !constants["Version"] {
		t.Errorf("Expected Version constant, got: %v", analysis.Constants)
	}
	if !constants["Red"] || !constants["Green"] || !constants["Blue"] {
		t.Errorf("Expected Red, Green, Blue constants, got: %v", analysis.Constants)
	}

	// Check type aliases
	types := make(map[string]bool)
	for _, ty := range analysis.Types {
		types[ty] = true
	}
	if !types["ID"] {
		t.Errorf("Expected ID type alias, got: %v", analysis.Types)
	}

	// Check vars
	vars := make(map[string]bool)
	for _, v := range analysis.Vars {
		vars[v] = true
	}
	if !vars["globalVar"] {
		t.Errorf("Expected globalVar var, got: %v", analysis.Vars)
	}
	if !vars["multiVar1"] || !vars["multiVar2"] {
		t.Errorf("Expected multiVar1, multiVar2 vars, got: %v", analysis.Vars)
	}
}
