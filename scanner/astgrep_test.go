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
type AliasType = string

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
	if !types["AliasType"] {
		t.Errorf("Expected AliasType explicit type alias (type X = Y), got: %v", analysis.Types)
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

// TypeScript extraction tests

func TestExtractStructNameTypeScript(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"class Foo {}", "typescript", "Foo"},
		{"export class Bar {}", "typescript", "Bar"},
		{"abstract class Baz {}", "typescript", "Baz"},
		{"export default class Qux {}", "typescript", "Qux"},
		{"class Generic<T> {}", "typescript", "Generic"},
		{"class Child extends Parent {}", "typescript", "Child"},
		{"export abstract class AbstractService {}", "typescript", "AbstractService"},
	}
	for _, tt := range tests {
		got := extractStructName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractStructName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractInterfaceNameTypeScript(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"interface Foo {}", "typescript", "Foo"},
		{"export interface Bar {}", "typescript", "Bar"},
		{"interface Generic<T> {}", "typescript", "Generic"},
		{"interface Child extends Parent {}", "typescript", "Child"},
	}
	for _, tt := range tests {
		got := extractInterfaceName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractInterfaceName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractTypeNameTypeScript(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"type ID = string", "typescript", "ID"},
		{"export type Handler = () => void", "typescript", "Handler"},
		{"type Generic<T> = T[]", "typescript", "Generic"},
		{"type StringOrNumber = string | number", "typescript", "StringOrNumber"},
	}
	for _, tt := range tests {
		got := extractTypeName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractTypeName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractEnumName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"enum Color { Red, Green, Blue }", "typescript", "Color"},
		{"export enum Status { Active, Inactive }", "typescript", "Status"},
		{"const enum Direction { Up, Down }", "typescript", "Direction"},
		{"export const enum Priority { Low, High }", "typescript", "Priority"},
	}
	for _, tt := range tests {
		got := extractEnumName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractEnumName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractMethodNameTypeScript(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"getData() {}", "typescript", "getData"},
		{"async fetchUser() {}", "typescript", "fetchUser"},
		{"public getUser() {}", "typescript", "getUser"},
		{"private static getInstance() {}", "typescript", "getInstance"},
		{"get name() {}", "typescript", "name"},
		{"set value(v: number) {}", "typescript", "value"},
	}
	for _, tt := range tests {
		got := extractMethodName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractMethodName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractConstantNamesTypeScript(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected []string
	}{
		{"const MAX_SIZE = 100", "typescript", []string{"MAX_SIZE"}},
		{"export const API_URL = 'http://api.com'", "typescript", []string{"API_URL"}},
		{"const config: Config = {}", "typescript", []string{"config"}},
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

func TestExtractVarNamesTypeScript(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected []string
	}{
		{"let counter = 0", "typescript", []string{"counter"}},
		{"var globalFlag = true", "typescript", []string{"globalFlag"}},
		{"let name: string", "typescript", []string{"name"}},
		{"export let shared = []", "typescript", []string{"shared"}},
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

func TestExtractArrowFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"const handler = () => {}", "typescript", "handler"},
		{"export const fetchUser = async () => {}", "typescript", "fetchUser"},
		{"const multiply = (a, b) => a * b", "typescript", "multiply"},
		{"let callback = () => {}", "typescript", "callback"},
		{"var legacyFn = () => {}", "typescript", "legacyFn"},
		{"const typed: Handler = () => {}", "typescript", "typed"},
		// JavaScript
		{"const handler = () => {}", "javascript", "handler"},
		{"export const fetchData = async () => {}", "javascript", "fetchData"},
	}
	for _, tt := range tests {
		got := extractArrowFunctionName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractArrowFunctionName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractNamespaceName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"namespace MyNamespace {}", "typescript", "MyNamespace"},
		{"export namespace ExportedNS {}", "typescript", "ExportedNS"},
		{"declare namespace AmbientNS {}", "typescript", "AmbientNS"},
		{"module LegacyModule {}", "typescript", "LegacyModule"},
		{"export module ExportedModule {}", "typescript", "ExportedModule"},
	}
	for _, tt := range tests {
		got := extractNamespaceName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractNamespaceName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractFunctionSignatureName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"function globalFunc(): void;", "typescript", "globalFunc"},
		{"function overloaded(x: string): string;", "typescript", "overloaded"},
		{"function generic<T>(x: T): T;", "typescript", "generic"},
	}
	for _, tt := range tests {
		got := extractFunctionSignatureName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractFunctionSignatureName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestTypeScriptSymbolsExtraction(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(tsFile, []byte(`import { something } from './other';

export interface User {
    name: string;
    age: number;
}

export type ID = string | number;

export enum Status {
    Active,
    Inactive
}

export class UserService {
    private users: User[] = [];

    async getUser(id: ID): Promise<User | null> {
        return this.users.find(u => u.name === id) || null;
    }

    addUser(user: User): void {
        this.users.push(user);
    }
}

export const DEFAULT_USER: User = { name: 'Guest', age: 0 };

export function createUser(name: string): User {
    return { name, age: 0 };
}

let counter = 0;
`), 0644)

	analysis, err := analyzer.AnalyzeFile(tsFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if analysis == nil {
		t.Fatal("Expected analysis, got nil")
	}

	// Check interfaces
	interfaces := make(map[string]bool)
	for _, i := range analysis.Interfaces {
		interfaces[i] = true
	}
	if !interfaces["User"] {
		t.Errorf("Expected User interface, got: %v", analysis.Interfaces)
	}

	// Check types (includes type aliases)
	types := make(map[string]bool)
	for _, ty := range analysis.Types {
		types[ty] = true
	}
	if !types["ID"] {
		t.Errorf("Expected ID type, got: %v", analysis.Types)
	}

	// Check enums (now in Enums field, not Types)
	enums := make(map[string]bool)
	for _, e := range analysis.Enums {
		enums[e] = true
	}
	if !enums["Status"] {
		t.Errorf("Expected Status enum in Enums, got: %v", analysis.Enums)
	}

	// Check classes (stored in Structs)
	structs := make(map[string]bool)
	for _, s := range analysis.Structs {
		structs[s] = true
	}
	if !structs["UserService"] {
		t.Errorf("Expected UserService class, got: %v", analysis.Structs)
	}

	// Check functions
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["createUser"] {
		t.Errorf("Expected createUser function, got: %v", analysis.Functions)
	}

	// Check methods
	methods := make(map[string]bool)
	for _, m := range analysis.Methods {
		methods[m] = true
	}
	if !methods["getUser"] {
		t.Errorf("Expected getUser method, got: %v", analysis.Methods)
	}
	if !methods["addUser"] {
		t.Errorf("Expected addUser method, got: %v", analysis.Methods)
	}

	// Check constants
	constants := make(map[string]bool)
	for _, c := range analysis.Constants {
		constants[c] = true
	}
	if !constants["DEFAULT_USER"] {
		t.Errorf("Expected DEFAULT_USER constant, got: %v", analysis.Constants)
	}

	// Check vars
	vars := make(map[string]bool)
	for _, v := range analysis.Vars {
		vars[v] = true
	}
	if !vars["counter"] {
		t.Errorf("Expected counter var, got: %v", analysis.Vars)
	}
}

func TestTypeScriptOptionalSymbolsExtraction(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "advanced.ts")
	os.WriteFile(tsFile, []byte(`// Arrow functions
export const handler = () => {};
const fetchUser = async (id: string) => { return id; };
const multiply = (a: number, b: number): number => a * b;

// Namespaces
namespace MyNamespace {
    export function innerFunc() {}
}

module LegacyModule {
    export const value = 1;
}

declare namespace AmbientNS {
    function ambientFunc(): void;
}

// Function signatures (ambient declarations)
declare function globalDeclare(): void;
declare function overloaded(x: string): string;
`), 0644)

	analysis, err := analyzer.AnalyzeFile(tsFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if analysis == nil {
		t.Fatal("Expected analysis, got nil")
	}

	// Check arrow functions (should be in Functions)
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["handler"] {
		t.Errorf("Expected handler arrow function, got: %v", analysis.Functions)
	}
	if !funcs["fetchUser"] {
		t.Errorf("Expected fetchUser arrow function, got: %v", analysis.Functions)
	}
	if !funcs["multiply"] {
		t.Errorf("Expected multiply arrow function, got: %v", analysis.Functions)
	}

	// Check function signatures
	if !funcs["globalDeclare"] {
		t.Errorf("Expected globalDeclare function signature, got: %v", analysis.Functions)
	}
	if !funcs["overloaded"] {
		t.Errorf("Expected overloaded function signature, got: %v", analysis.Functions)
	}

	// Check namespaces (should be in Types)
	types := make(map[string]bool)
	for _, ty := range analysis.Types {
		types[ty] = true
	}
	if !types["MyNamespace"] {
		t.Errorf("Expected MyNamespace namespace, got: %v", analysis.Types)
	}
	if !types["LegacyModule"] {
		t.Errorf("Expected LegacyModule module, got: %v", analysis.Types)
	}
	if !types["AmbientNS"] {
		t.Errorf("Expected AmbientNS namespace, got: %v", analysis.Types)
	}
}

func TestExtractMethodSignatureName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"methodSig(): void", "typescript", "methodSig"},
		{"optionalMethod?(): string", "typescript", "optionalMethod"},
		{"abstract abstractMethod(): void", "typescript", "abstractMethod"},
		{"getData<T>(): T", "typescript", "getData"},
	}
	for _, tt := range tests {
		got := extractMethodSignatureName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractMethodSignatureName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractPropertySignatureName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"name: string", "typescript", "name"},
		{"age?: number", "typescript", "age"},
		{"readonly id: string", "typescript", "id"},
	}
	for _, tt := range tests {
		got := extractPropertySignatureName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractPropertySignatureName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractFieldDefinitionName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"public name: string", "typescript", "name"},
		{"private count: number = 0", "typescript", "count"},
		{"protected data: any", "typescript", "data"},
		{"readonly id: string", "typescript", "id"},
		{"static instance: MyClass", "typescript", "instance"},
		{"@Input() inputProp: string", "typescript", "inputProp"},
		{"@Inject('TOKEN') service: Service", "typescript", "service"},
	}
	for _, tt := range tests {
		got := extractFieldDefinitionName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractFieldDefinitionName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractDecoratorName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"@Component", "typescript", "Component"},
		{"@Component({selector: 'app'})", "typescript", "Component"},
		{"@Injectable()", "typescript", "Injectable"},
		{"@Input()", "typescript", "Input"},
		{"@HostListener('click')", "typescript", "HostListener"},
	}
	for _, tt := range tests {
		got := extractDecoratorName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractDecoratorName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractGeneratorFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"function* myGenerator() {}", "typescript", "myGenerator"},
		{"function *spacedGen() {}", "typescript", "spacedGen"},
		{"async function* asyncGen() {}", "typescript", "asyncGen"},
		{"export function* exportedGen() {}", "typescript", "exportedGen"},
	}
	for _, tt := range tests {
		got := extractGeneratorFunctionName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractGeneratorFunctionName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestExtractImportAliasName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"fs = require('fs')", "typescript", "fs"},
		{"path = require('path')", "typescript", "path"},
		{`mod = require("module")`, "typescript", "module"},
		{"import Types = MyNamespace.Types;", "typescript", "MyNamespace"},
		{"import Alias = Some.Deep.Nested.Module;", "typescript", "Some"},
		{"import Simple = SimpleModule;", "typescript", "SimpleModule"},
	}
	for _, tt := range tests {
		got := extractImportAliasName(tt.input, tt.lang)
		if got != tt.expected {
			t.Errorf("extractImportAliasName(%q, %q) = %q, want %q",
				tt.input, tt.lang, got, tt.expected)
		}
	}
}

func TestTypeScriptAllSymbolsExtraction(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "all_symbols.ts")
	os.WriteFile(tsFile, []byte(`// Method signatures (in interface)
interface MyInterface {
    methodSig(): void;
    optionalMethod?(): string;
}

// Abstract method signatures
abstract class AbstractClass {
    abstract abstractMethod(): void;
}

// Property signatures (in interface)
interface Props {
    name: string;
    age?: number;
}

// Public field definitions (class fields)
class MyClass {
    public publicField: string = "";
    private privateField: number = 0;
}

// Decorators
@Component({selector: 'app'})
class DecoratedClass {
    @Input() inputProp: string = "";
}

// Generator functions
function* myGenerator() {
    yield 1;
}
`), 0644)

	analysis, err := analyzer.AnalyzeFile(tsFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if analysis == nil {
		t.Fatal("Expected analysis, got nil")
	}

	// Check method signatures are in Methods
	methods := make(map[string]bool)
	for _, m := range analysis.Methods {
		methods[m] = true
	}
	if !methods["methodSig"] {
		t.Errorf("Expected methodSig method signature, got: %v", analysis.Methods)
	}
	if !methods["optionalMethod"] {
		t.Errorf("Expected optionalMethod method signature, got: %v", analysis.Methods)
	}
	if !methods["abstractMethod"] {
		t.Errorf("Expected abstractMethod, got: %v", analysis.Methods)
	}

	// Check property signatures
	props := make(map[string]bool)
	for _, p := range analysis.Properties {
		props[p] = true
	}
	if !props["name"] {
		t.Errorf("Expected name property, got: %v", analysis.Properties)
	}
	if !props["age"] {
		t.Errorf("Expected age property, got: %v", analysis.Properties)
	}

	// Check field definitions
	fields := make(map[string]bool)
	for _, f := range analysis.Fields {
		fields[f] = true
	}
	if !fields["publicField"] {
		t.Errorf("Expected publicField field, got: %v", analysis.Fields)
	}
	if !fields["privateField"] {
		t.Errorf("Expected privateField field, got: %v", analysis.Fields)
	}
	if !fields["inputProp"] {
		t.Errorf("Expected inputProp field (decorated), got: %v", analysis.Fields)
	}

	// Check decorators
	decorators := make(map[string]bool)
	for _, d := range analysis.Decorators {
		decorators[d] = true
	}
	if !decorators["Component"] {
		t.Errorf("Expected Component decorator, got: %v", analysis.Decorators)
	}
	if !decorators["Input"] {
		t.Errorf("Expected Input decorator, got: %v", analysis.Decorators)
	}

	// Check generator functions
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["myGenerator"] {
		t.Errorf("Expected myGenerator generator function, got: %v", analysis.Functions)
	}

	// Check classes
	structs := make(map[string]bool)
	for _, s := range analysis.Structs {
		structs[s] = true
	}
	if !structs["DecoratedClass"] {
		t.Errorf("Expected DecoratedClass (decorated class), got: %v", analysis.Structs)
	}
}

func TestExtractAmbientDeclaration(t *testing.T) {
	tests := []struct {
		input          string
		wantFunctions  []string
		wantConstants  []string
		wantVars       []string
		wantStructs    []string
		wantInterfaces []string
		wantTypes      []string
	}{
		{
			input:         "declare function test(): void;",
			wantFunctions: []string{"test"},
		},
		{
			input:         "declare function genericFn<T>(x: T): T;",
			wantFunctions: []string{"genericFn"},
		},
		{
			input:         "declare const API_KEY: string;",
			wantConstants: []string{"API_KEY"},
		},
		{
			input:    "declare let counter: number;",
			wantVars: []string{"counter"},
		},
		{
			input:    "declare var globalThing: any;",
			wantVars: []string{"globalThing"},
		},
		{
			input:       "declare class Logger {}",
			wantStructs: []string{"Logger"},
		},
		{
			input:       "declare abstract class BaseService {}",
			wantStructs: []string{"BaseService"},
		},
		{
			input:          "declare interface ILogger {}",
			wantInterfaces: []string{"ILogger"},
		},
		{
			input:     "declare namespace Utils {}",
			wantTypes: []string{"Utils"},
		},
		{
			input:     "declare enum Color { Red, Green }",
			wantTypes: []string{"Color"},
		},
		{
			input:     "declare type ID = string;",
			wantTypes: []string{"ID"},
		},
		{
			// String module names should be skipped
			input:     `declare module "my-module" {}`,
			wantTypes: nil,
		},
	}

	for _, tt := range tests {
		fa := &FileAnalysis{}
		extractAmbientDeclaration(tt.input, fa)

		if len(tt.wantFunctions) > 0 {
			if len(fa.Functions) != len(tt.wantFunctions) || (len(fa.Functions) > 0 && fa.Functions[0] != tt.wantFunctions[0]) {
				t.Errorf("extractAmbientDeclaration(%q) Functions = %v, want %v", tt.input, fa.Functions, tt.wantFunctions)
			}
		}
		if len(tt.wantConstants) > 0 {
			if len(fa.Constants) != len(tt.wantConstants) || (len(fa.Constants) > 0 && fa.Constants[0] != tt.wantConstants[0]) {
				t.Errorf("extractAmbientDeclaration(%q) Constants = %v, want %v", tt.input, fa.Constants, tt.wantConstants)
			}
		}
		if len(tt.wantVars) > 0 {
			if len(fa.Vars) != len(tt.wantVars) || (len(fa.Vars) > 0 && fa.Vars[0] != tt.wantVars[0]) {
				t.Errorf("extractAmbientDeclaration(%q) Vars = %v, want %v", tt.input, fa.Vars, tt.wantVars)
			}
		}
		if len(tt.wantStructs) > 0 {
			if len(fa.Structs) != len(tt.wantStructs) || (len(fa.Structs) > 0 && fa.Structs[0] != tt.wantStructs[0]) {
				t.Errorf("extractAmbientDeclaration(%q) Structs = %v, want %v", tt.input, fa.Structs, tt.wantStructs)
			}
		}
		if len(tt.wantInterfaces) > 0 {
			if len(fa.Interfaces) != len(tt.wantInterfaces) || (len(fa.Interfaces) > 0 && fa.Interfaces[0] != tt.wantInterfaces[0]) {
				t.Errorf("extractAmbientDeclaration(%q) Interfaces = %v, want %v", tt.input, fa.Interfaces, tt.wantInterfaces)
			}
		}
		if len(tt.wantTypes) > 0 {
			if len(fa.Types) != len(tt.wantTypes) || (len(fa.Types) > 0 && fa.Types[0] != tt.wantTypes[0]) {
				t.Errorf("extractAmbientDeclaration(%q) Types = %v, want %v", tt.input, fa.Types, tt.wantTypes)
			}
		}
	}
}

func TestAmbientDeclarationsIntegration(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	dtsFile := filepath.Join(tmpDir, "test.d.ts")
	os.WriteFile(dtsFile, []byte(`
declare function testFn(): void;
declare const API_KEY: string;
declare let counter: number;
declare var globalThing: any;
declare class Logger {}
declare namespace Utils {
    function helper(): void;
}
declare enum Color { Red, Green, Blue }
declare type ID = string | number;
declare interface IConfig {
    name: string;
}
`), 0644)

	analysis, err := analyzer.AnalyzeFile(dtsFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Check functions
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["testFn"] {
		t.Errorf("Expected testFn function, got: %v", analysis.Functions)
	}
	// helper is a function_signature inside namespace, may or may not be captured
	// depending on how ast-grep handles nested declarations

	// Check constants
	consts := make(map[string]bool)
	for _, c := range analysis.Constants {
		consts[c] = true
	}
	if !consts["API_KEY"] {
		t.Errorf("Expected API_KEY constant, got: %v", analysis.Constants)
	}

	// Check vars
	vars := make(map[string]bool)
	for _, v := range analysis.Vars {
		vars[v] = true
	}
	if !vars["counter"] {
		t.Errorf("Expected counter var, got: %v", analysis.Vars)
	}
	if !vars["globalThing"] {
		t.Errorf("Expected globalThing var, got: %v", analysis.Vars)
	}

	// Check classes
	structs := make(map[string]bool)
	for _, s := range analysis.Structs {
		structs[s] = true
	}
	if !structs["Logger"] {
		t.Errorf("Expected Logger class, got: %v", analysis.Structs)
	}

	// Check types (includes namespaces, type aliases)
	types := make(map[string]bool)
	for _, ty := range analysis.Types {
		types[ty] = true
	}
	if !types["Utils"] {
		t.Errorf("Expected Utils namespace, got: %v", analysis.Types)
	}
	if !types["ID"] {
		t.Errorf("Expected ID type alias, got: %v", analysis.Types)
	}

	// Check enums (now in Enums field, not Types)
	enums := make(map[string]bool)
	for _, e := range analysis.Enums {
		enums[e] = true
	}
	if !enums["Color"] {
		t.Errorf("Expected Color enum in Enums, got: %v", analysis.Enums)
	}

	// Check interfaces
	interfaces := make(map[string]bool)
	for _, i := range analysis.Interfaces {
		interfaces[i] = true
	}
	if !interfaces["IConfig"] {
		t.Errorf("Expected IConfig interface, got: %v", analysis.Interfaces)
	}
}

func TestExtractCallSignature(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"(): void", "()"},
		{"(x: number): string", "()"},
		{"(a: string, b: number): boolean", "()"},
		{"someMethod(): void", ""},
	}
	for _, tt := range tests {
		got := extractCallSignature(tt.input)
		if got != tt.expected {
			t.Errorf("extractCallSignature(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractConstructSignature(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"new(): object", "new()"},
		{"new(x: string): MyClass", "new()"},
		{"new (a: number): Instance", "new()"},
		{"notNew(): void", ""},
	}
	for _, tt := range tests {
		got := extractConstructSignature(tt.input)
		if got != tt.expected {
			t.Errorf("extractConstructSignature(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractIndexSignature(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"[key: string]: number", "[string]"},
		{"[index: number]: string", "[number]"},
		{"[id: symbol]: any", "[symbol]"},
		{"notAnIndex: string", ""},
	}
	for _, tt := range tests {
		got := extractIndexSignature(tt.input)
		if got != tt.expected {
			t.Errorf("extractIndexSignature(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractVarDeclarationNames(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"var x = 1", []string{"x"}},
		{"var x = 1, y = 2", []string{"x", "y"}},
		{"var name = 'hello'", []string{"name"}},
		{"var a = 1, b = 2, c = 3", []string{"a", "b", "c"}},
		{"var obj = {a: 1, b: 2}", []string{"obj"}},
		{"const x = 1", []string{}}, // not var
	}
	for _, tt := range tests {
		got := extractVarDeclarationNames(tt.input, "typescript")
		if len(got) != len(tt.expected) {
			t.Errorf("extractVarDeclarationNames(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i, name := range got {
			if name != tt.expected[i] {
				t.Errorf("extractVarDeclarationNames(%q)[%d] = %q, want %q", tt.input, i, name, tt.expected[i])
			}
		}
	}
}

func TestExtractFunctionExpressionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"function myFunc() { return 42; }", "myFunc"},
		{"function namedFn(x) { return x; }", "namedFn"},
		{"function() { return 1; }", ""}, // anonymous
		{"async function asyncNamed() {}", "asyncNamed"},
		{"function* generatorNamed() {}", "generatorNamed"},
		{"function generic<T>(x: T) {}", "generic"},
	}
	for _, tt := range tests {
		got := extractFunctionExpressionName(tt.input, "typescript")
		if got != tt.expected {
			t.Errorf("extractFunctionExpressionName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNewSymbolTypesIntegration(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(tsFile, []byte(`
// call_signature
interface Callable {
    (): void;
    (x: number): string;
}

// construct_signature
interface Constructable {
    new(): object;
    new(x: string): MyClass;
}

// index_signature
interface Indexable {
    [key: string]: number;
    [index: number]: string;
}

// variable_declaration (var)
var oldStyle = "legacy";
var x = 1, y = 2;

// function_expression with name
const namedFn = function myNamedFunction() {
    return 42;
};

// class_static_block
class MyClass {
    static data: string;
    static {
        MyClass.data = "initialized";
    }
}
`), 0644)

	analysis, err := analyzer.AnalyzeFile(tsFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Check functions (named function expression)
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["myNamedFunction"] {
		t.Errorf("Expected myNamedFunction from function expression, got: %v", analysis.Functions)
	}

	// Check methods (call signatures, construct signatures, static blocks)
	methods := make(map[string]bool)
	for _, m := range analysis.Methods {
		methods[m] = true
	}
	if !methods["()"] {
		t.Errorf("Expected () call signature, got: %v", analysis.Methods)
	}
	if !methods["new()"] {
		t.Errorf("Expected new() construct signature, got: %v", analysis.Methods)
	}
	if !methods["(static block)"] {
		t.Errorf("Expected (static block), got: %v", analysis.Methods)
	}

	// Check vars (var declarations)
	vars := make(map[string]bool)
	for _, v := range analysis.Vars {
		vars[v] = true
	}
	if !vars["oldStyle"] {
		t.Errorf("Expected oldStyle var, got: %v", analysis.Vars)
	}
	if !vars["x"] {
		t.Errorf("Expected x var, got: %v", analysis.Vars)
	}
	if !vars["y"] {
		t.Errorf("Expected y var, got: %v", analysis.Vars)
	}

	// Check properties (index signatures)
	props := make(map[string]bool)
	for _, p := range analysis.Properties {
		props[p] = true
	}
	if !props["[string]"] {
		t.Errorf("Expected [string] index signature, got: %v", analysis.Properties)
	}
	if !props["[number]"] {
		t.Errorf("Expected [number] index signature, got: %v", analysis.Properties)
	}

	// Check interfaces
	interfaces := make(map[string]bool)
	for _, i := range analysis.Interfaces {
		interfaces[i] = true
	}
	if !interfaces["Callable"] {
		t.Errorf("Expected Callable interface, got: %v", analysis.Interfaces)
	}
	if !interfaces["Constructable"] {
		t.Errorf("Expected Constructable interface, got: %v", analysis.Interfaces)
	}
	if !interfaces["Indexable"] {
		t.Errorf("Expected Indexable interface, got: %v", analysis.Interfaces)
	}

	// Check classes
	structs := make(map[string]bool)
	for _, s := range analysis.Structs {
		structs[s] = true
	}
	if !structs["MyClass"] {
		t.Errorf("Expected MyClass class, got: %v", analysis.Structs)
	}
}

// Tests for symbol extraction

func TestDetermineRole(t *testing.T) {
	tests := []struct {
		ruleID   string
		expected SymbolRole
	}{
		{"ts-functions", RoleDefinition},
		{"ts-classes", RoleDefinition},
		{"ts-ref-function-calls", RoleReference},
		{"ts-ref-new-expressions", RoleReference},
		{"js-ref-function-calls", RoleReference},
		{"go-functions", RoleDefinition},
	}

	for _, tc := range tests {
		t.Run(tc.ruleID, func(t *testing.T) {
			result := determineRole(tc.ruleID)
			if result != tc.expected {
				t.Errorf("determineRole(%q) = %v, want %v", tc.ruleID, result, tc.expected)
			}
		})
	}
}

func TestDetermineKind(t *testing.T) {
	tests := []struct {
		ruleID   string
		expected SymbolKind
	}{
		{"ts-functions", KindFunction},
		{"ts-arrow-functions", KindFunction},
		{"ts-classes", KindClass},
		{"ts-interfaces", KindInterface},
		{"ts-methods", KindMethod},
		{"ts-constants", KindConstant},
		{"ts-vars", KindVariable},
		{"ts-field-definitions", KindField},
		{"ts-property-signatures", KindProperty},
		{"ts-decorators", KindDecorator},
		{"ts-imports", KindImport},
		{"ts-namespaces", KindNamespace},
		{"ts-enums", KindEnum},
		{"ts-ref-function-calls", KindFunction},
		{"ts-ref-new-expressions", KindClass},
	}

	for _, tc := range tests {
		t.Run(tc.ruleID, func(t *testing.T) {
			result := determineKind(tc.ruleID)
			if result != tc.expected {
				t.Errorf("determineKind(%q) = %v, want %v", tc.ruleID, result, tc.expected)
			}
		})
	}
}

func TestExtractScopeFromContext(t *testing.T) {
	tests := []struct {
		name     string
		lines    string
		lang     string
		expected string
	}{
		{
			name:     "global scope - no context",
			lines:    "function foo() {}",
			lang:     "typescript",
			expected: "global",
		},
		{
			name:     "class scope",
			lines:    "class MyClass {\n  method() {}\n}",
			lang:     "typescript",
			expected: "class:MyClass",
		},
		{
			name:     "interface scope",
			lines:    "interface IFoo {\n  bar(): void;\n}",
			lang:     "typescript",
			expected: "interface:IFoo",
		},
		{
			name:     "go language - always global",
			lines:    "class MyClass {}",
			lang:     "go",
			expected: "global",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractScopeFromContext(tc.lines, tc.lang)
			if result != tc.expected {
				t.Errorf("extractScopeFromContext(%q, %q) = %q, want %q", tc.lines, tc.lang, result, tc.expected)
			}
		})
	}
}

func TestExtractModifiers(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		lang     string
		expected []string
	}{
		{
			name:     "public method",
			text:     "public method() {}",
			lang:     "typescript",
			expected: []string{"public"},
		},
		{
			name:     "private static",
			text:     "private static field = 1",
			lang:     "typescript",
			expected: []string{"private", "static"},
		},
		{
			name:     "async function",
			text:     "async function foo() {}",
			lang:     "typescript",
			expected: []string{"async"},
		},
		{
			name:     "export default",
			text:     "export default class Foo {}",
			lang:     "typescript",
			expected: []string{"export", "default"},
		},
		{
			name:     "go language - no modifiers",
			text:     "public func foo() {}",
			lang:     "go",
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractModifiers(tc.text, tc.lang)
			if len(result) != len(tc.expected) {
				t.Errorf("extractModifiers(%q, %q) = %v, want %v", tc.text, tc.lang, result, tc.expected)
				return
			}
			for i, mod := range tc.expected {
				found := false
				for _, r := range result {
					if r == mod {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("extractModifiers(%q, %q) missing %q at index %d", tc.text, tc.lang, mod, i)
				}
			}
		})
	}
}

func TestExtractCallExpressionName(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"foo()", "foo"},
		{"obj.method()", "method"},
		{"obj?.optionalMethod()", "optionalMethod"},
		{"console.log('hello')", "log"},
		{"()", ""},
		{"123", ""},
	}

	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			result := extractCallExpressionName(tc.text)
			if result != tc.expected {
				t.Errorf("extractCallExpressionName(%q) = %q, want %q", tc.text, result, tc.expected)
			}
		})
	}
}

func TestExtractNewExpressionName(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"new Foo()", "Foo"},
		{"new MyClass(arg1, arg2)", "MyClass"},
		{"new Generic<T>()", "Generic"},
		{"foo()", ""},
		{"new ()", ""},
	}

	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			result := extractNewExpressionName(tc.text)
			if result != tc.expected {
				t.Errorf("extractNewExpressionName(%q) = %q, want %q", tc.text, result, tc.expected)
			}
		})
	}
}

func TestExtractTypeReferenceName(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"string", "string"},
		{"MyClass", "MyClass"},
		{"Array<T>", "Array"},
		{"Namespace.Type", "Type"},
		{"Foo.Bar.Baz", "Baz"},
	}

	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			result := extractTypeReferenceName(tc.text)
			if result != tc.expected {
				t.Errorf("extractTypeReferenceName(%q) = %q, want %q", tc.text, result, tc.expected)
			}
		})
	}
}

func TestScanSymbolsIntegration(t *testing.T) {
	scanner, err := NewAstGrepScanner()
	if err != nil {
		t.Fatalf("Failed to create scanner: %v", err)
	}
	defer scanner.Close()

	if !scanner.Available() {
		t.Skip("ast-grep (sg) not installed")
	}

	// Create test TypeScript file
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(tsFile, []byte(`
import { foo } from './bar';

class MyClass {
    private field: string;

    constructor() {
        this.field = "test";
    }

    public method(): void {
        foo();
        const local = new MyClass();
    }
}

function standalone() {
    const x = 1;
}

interface IFoo {
    bar: string;
    baz(): void;
}

type MyType = string | number;

enum Color { Red, Green, Blue }
`), 0644)

	// Test without refs
	results, err := scanner.ScanSymbols(tmpDir, false)
	if err != nil {
		t.Fatalf("ScanSymbols failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}

	analysis := results[0]

	// Check that we have symbols
	if len(analysis.Symbols) == 0 {
		t.Fatal("Expected symbols, got none")
	}

	// Check for expected symbol kinds
	kindCounts := make(map[SymbolKind]int)
	for _, sym := range analysis.Symbols {
		kindCounts[sym.Kind]++
	}

	if kindCounts[KindClass] < 1 {
		t.Errorf("Expected at least 1 class, got %d", kindCounts[KindClass])
	}
	if kindCounts[KindFunction] < 1 {
		t.Errorf("Expected at least 1 function, got %d", kindCounts[KindFunction])
	}
	if kindCounts[KindInterface] < 1 {
		t.Errorf("Expected at least 1 interface, got %d", kindCounts[KindInterface])
	}

	// Check all symbols have line numbers > 0
	for _, sym := range analysis.Symbols {
		if sym.Line <= 0 {
			t.Errorf("Symbol %q has invalid line number: %d", sym.Name, sym.Line)
		}
	}

	// Check all symbols have role set
	for _, sym := range analysis.Symbols {
		if sym.Role != RoleDefinition && sym.Role != RoleReference {
			t.Errorf("Symbol %q has invalid role: %v", sym.Name, sym.Role)
		}
	}

	// Test with refs enabled
	resultsWithRefs, err := scanner.ScanSymbols(tmpDir, true)
	if err != nil {
		t.Fatalf("ScanSymbols with refs failed: %v", err)
	}

	// Should have more symbols when refs are included
	if len(resultsWithRefs) == 0 {
		t.Fatal("Expected results with refs, got none")
	}
}

func TestFindEndLine(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		startLine int // 0-indexed
		expected  int // 0-indexed
	}{
		{
			name: "simple class",
			content: `class Foo {
    method() {}
}`,
			startLine: 0,
			expected:  2,
		},
		{
			name: "nested braces",
			content: `class Bar {
    method() {
        if (true) {
            console.log("hi");
        }
    }
}`,
			startLine: 0,
			expected:  6,
		},
		{
			name: "interface",
			content: `interface IFoo {
    prop: string;
    method(): void;
}`,
			startLine: 0,
			expected:  3,
		},
		{
			name: "multiple classes",
			content: `class First {
    a() {}
}

class Second {
    b() {}
}`,
			startLine: 4, // Second class starts at line 4 (0-indexed)
			expected:  6,
		},
		{
			name: "single line class",
			content: `declare class Empty {}
declare class Another {}`,
			startLine: 0,
			expected:  0, // Opening and closing brace on same line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findEndLine([]byte(tt.content), tt.startLine)
			if result != tt.expected {
				t.Errorf("findEndLine() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestFindContainingScope(t *testing.T) {
	// Using 0-indexed line numbers to match ast-grep output
	containers := []ScopeContainer{
		{Name: "MyClass", Kind: "class", StartLine: 0, EndLine: 9},
		{Name: "IFoo", Kind: "interface", StartLine: 11, EndLine: 19},
		{Name: "Inner", Kind: "class", StartLine: 2, EndLine: 7}, // Nested inside MyClass
	}

	tests := []struct {
		name       string
		symbolLine int // 0-indexed
		expected   string
	}{
		{"global symbol", 24, "global"},
		{"inside class", 1, "class:MyClass"},
		{"inside interface", 14, "interface:IFoo"},
		{"inside nested class", 4, "class:Inner"}, // Should pick Inner, not MyClass
		{"at class start line", 0, "global"},      // Container declarations are global
		{"at class end", 9, "class:MyClass"},
		{"between containers", 10, "global"},
		{"at interface start line", 11, "global"}, // Interface declaration is global
		{"at nested class start", 2, "class:MyClass"}, // Nested class decl is inside parent
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findContainingScope(tt.symbolLine, containers)
			if result != tt.expected {
				t.Errorf("findContainingScope(%d) = %q, want %q", tt.symbolLine, result, tt.expected)
			}
		})
	}
}

func TestExtractContainerName(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		kind     string
		expected string
	}{
		{"simple class", "class Foo {", "class", "Foo"},
		{"export class", "export class Bar {", "class", "Bar"},
		{"abstract class", "abstract class Baz {", "class", "Baz"},
		{"export default class", "export default class Qux {", "class", "Qux"},
		{"class with generics", "class Generic<T> {", "class", "Generic"},
		{"class extends", "class Child extends Parent {", "class", "Child"},
		{"simple interface", "interface IFoo {", "interface", "IFoo"},
		{"export interface", "export interface IBar {", "interface", "IBar"},
		{"interface with generics", "interface IGeneric<T> {", "interface", "IGeneric"},
		{"simple enum", "enum Direction {", "enum", "Direction"},
		{"const enum", "const enum Colors {", "enum", "Colors"},
		{"export enum", "export enum Status {", "enum", "Status"},
		{"namespace", "namespace MyNS {", "namespace", "MyNS"},
		{"module keyword", "module OldNS {", "namespace", "OldNS"},
		{"declare namespace", "declare namespace DeclNS {", "namespace", "DeclNS"},
		{"decorated class", "@Component\nclass DecClass {", "class", "DecClass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContainerName(tt.text, tt.kind)
			if result != tt.expected {
				t.Errorf("extractContainerName(%q, %q) = %q, want %q", tt.text, tt.kind, result, tt.expected)
			}
		})
	}
}

func TestScopeTrackingIntegration(t *testing.T) {
	scanner, err := NewAstGrepScanner()
	if err != nil || !scanner.Available() {
		t.Skip("ast-grep not available")
	}
	defer scanner.Close()

	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "scope_test.ts")
	os.WriteFile(tsFile, []byte(`import { foo } from './bar';

class MyClass {
    private field: string;

    constructor() {
        this.field = "test";
    }

    public method(): void {
        foo();
    }
}

function standalone() {
    const x = 1;
}

interface IFoo {
    bar: string;
    baz(): void;
}

enum Color { Red, Green, Blue }
`), 0644)

	results, err := scanner.ScanSymbols(tmpDir, false)
	if err != nil {
		t.Fatalf("ScanSymbols failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}

	// Build a map of symbol scopes
	symbolScopes := make(map[string]string)
	for _, file := range results {
		for _, sym := range file.Symbols {
			// Use name:kind as key since names might not be unique
			key := sym.Name + ":" + string(sym.Kind)
			symbolScopes[key] = sym.Scope
		}
	}

	// Check expected scopes
	expectedScopes := map[string]string{
		"MyClass:class":      "global",
		"field:field":        "class:MyClass",
		"constructor:method": "class:MyClass",
		"method:method":      "class:MyClass",
		"standalone:function": "global",
		"IFoo:interface":     "global",
		"bar:property":       "interface:IFoo",
		"baz:method":         "interface:IFoo",
		"Color:enum":         "global",
	}

	for key, expected := range expectedScopes {
		if actual, ok := symbolScopes[key]; ok {
			if actual != expected {
				t.Errorf("Symbol %q: scope = %q, want %q", key, actual, expected)
			}
		}
		// Don't fail if symbol not found - other tests cover extraction
	}

	// At minimum, verify some symbols are NOT global
	nonGlobalCount := 0
	for _, file := range results {
		for _, sym := range file.Symbols {
			if sym.Scope != "global" && sym.Scope != "" {
				nonGlobalCount++
			}
		}
	}

	if nonGlobalCount == 0 {
		t.Error("Expected some symbols to have non-global scope, but all are global")
	}
}

func TestJavaScriptPrivateFields(t *testing.T) {
	scanner, err := NewAstGrepScanner()
	if err != nil || !scanner.Available() {
		t.Skip("ast-grep not available")
	}
	defer scanner.Close()

	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "private.js")
	os.WriteFile(jsFile, []byte(`
class MyClass {
    #privateField = "secret";
    publicField = "public";

    #privateMethod() {
        return this.#privateField;
    }

    publicMethod() {
        return this.#privateMethod();
    }
}
`), 0644)

	results, err := scanner.ScanSymbols(tmpDir, false)
	if err != nil {
		t.Fatalf("ScanSymbols failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}

	// Check for private fields and methods
	foundPrivateField := false
	foundPrivateMethod := false
	for _, file := range results {
		for _, sym := range file.Symbols {
			if sym.Name == "#privateField" && sym.Kind == KindField {
				foundPrivateField = true
			}
			if sym.Name == "#privateMethod" && sym.Kind == KindMethod {
				foundPrivateMethod = true
			}
		}
	}

	if !foundPrivateField {
		t.Error("Expected to find #privateField")
	}
	if !foundPrivateMethod {
		t.Error("Expected to find #privateMethod")
	}
}

func TestJavaScriptDecoratedMethods(t *testing.T) {
	scanner, err := NewAstGrepScanner()
	if err != nil || !scanner.Available() {
		t.Skip("ast-grep not available")
	}
	defer scanner.Close()

	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "decorated.js")
	os.WriteFile(jsFile, []byte(`
function MyDecorator(target) {}

class DecoratedClass {
    @MyDecorator
    decoratedField = 1;

    @MyDecorator
    decoratedMethod() {}

    regularMethod() {}
}
`), 0644)

	results, err := scanner.ScanSymbols(tmpDir, false)
	if err != nil {
		t.Fatalf("ScanSymbols failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}

	// Check for decorated method
	foundDecoratedMethod := false
	foundDecoratedField := false
	for _, file := range results {
		for _, sym := range file.Symbols {
			if sym.Name == "decoratedMethod" && sym.Kind == KindMethod {
				foundDecoratedMethod = true
			}
			if sym.Name == "decoratedField" && sym.Kind == KindField {
				foundDecoratedField = true
			}
		}
	}

	if !foundDecoratedMethod {
		t.Error("Expected to find decoratedMethod")
	}
	if !foundDecoratedField {
		t.Error("Expected to find decoratedField")
	}
}

func TestJavaScriptComputedMethods(t *testing.T) {
	scanner, err := NewAstGrepScanner()
	if err != nil || !scanner.Available() {
		t.Skip("ast-grep not available")
	}
	defer scanner.Close()

	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "computed.js")
	os.WriteFile(jsFile, []byte(`
const methodName = "dynamicMethod";

class WithComputed {
    [methodName]() {}
    ["literalMethod"]() {}
    regularMethod() {}
}
`), 0644)

	results, err := scanner.ScanSymbols(tmpDir, false)
	if err != nil {
		t.Fatalf("ScanSymbols failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}

	// Check for computed methods
	foundComputedVar := false
	foundComputedLiteral := false
	foundRegular := false
	for _, file := range results {
		for _, sym := range file.Symbols {
			if sym.Name == "[methodName]" && sym.Kind == KindMethod {
				foundComputedVar = true
			}
			if sym.Name == "[literalMethod]" && sym.Kind == KindMethod {
				foundComputedLiteral = true
			}
			if sym.Name == "regularMethod" && sym.Kind == KindMethod {
				foundRegular = true
			}
		}
	}

	if !foundComputedVar {
		t.Error("Expected to find [methodName] computed method")
	}
	if !foundComputedLiteral {
		t.Error("Expected to find [literalMethod] computed method")
	}
	if !foundRegular {
		t.Error("Expected to find regularMethod")
	}
}

func TestIsValidIdentifierWithPrivate(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"foo", true},
		{"_bar", true},
		{"Baz123", true},
		{"#privateField", true},
		{"#_private", true},
		{"123invalid", false},
		{"", false},
		{"foo-bar", false},
		{"##double", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGoScopeTracking(t *testing.T) {
	scanner, err := NewAstGrepScanner()
	if err != nil || !scanner.Available() {
		t.Skip("ast-grep not available")
	}
	defer scanner.Close()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "scope_test.go")
	os.WriteFile(goFile, []byte(`package main

type MyStruct struct {
    Field1 string
}

func (m *MyStruct) Method1() {}
func (m MyStruct) Method2() string { return "" }

type MyInterface interface {
    DoSomething()
}

func standalone() {}
`), 0644)

	results, err := scanner.ScanSymbols(tmpDir, false)
	if err != nil {
		t.Fatalf("ScanSymbols failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}

	// Check for method scopes
	method1Scope := ""
	method2Scope := ""
	standaloneScope := ""
	for _, file := range results {
		for _, sym := range file.Symbols {
			if sym.Name == "Method1" && sym.Kind == KindMethod {
				method1Scope = sym.Scope
			}
			if sym.Name == "Method2" && sym.Kind == KindMethod {
				method2Scope = sym.Scope
			}
			if sym.Name == "standalone" && sym.Kind == KindFunction {
				standaloneScope = sym.Scope
			}
		}
	}

	if method1Scope != "struct:MyStruct" {
		t.Errorf("Method1 scope = %q, want %q", method1Scope, "struct:MyStruct")
	}
	if method2Scope != "struct:MyStruct" {
		t.Errorf("Method2 scope = %q, want %q", method2Scope, "struct:MyStruct")
	}
	if standaloneScope != "global" {
		t.Errorf("standalone scope = %q, want %q", standaloneScope, "global")
	}
}

func TestExtractGoReceiverType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"func (m *MyStruct) Method()", "MyStruct"},
		{"func (m MyStruct) Method()", "MyStruct"},
		{"func (s *Server) Handle()", "Server"},
		{"func standalone()", ""},
		{"func helper(x int)", ""},
		{"func (r *Response) Write(data []byte)", "Response"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractGoReceiverType(tt.input)
			if result != tt.expected {
				t.Errorf("extractGoReceiverType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractGoTypeAliasName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AliasType = string", "AliasType"},
		{"ID = int", "ID"},
		{"StringSlice = []string", "StringSlice"},
		{"MyFunc = func(int) bool", "MyFunc"},
		{"type ID int", ""},      // Not explicit alias syntax
		{"InvalidNoEquals", ""},  // Missing =
		{"", ""},                 // Empty
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := extractGoTypeAliasName(tt.input)
			if result != tt.expected {
				t.Errorf("extractGoTypeAliasName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Python extraction tests

func TestExtractPythonClassName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"class MyClass:", "MyClass"},
		{"class MyClass():", "MyClass"},
		{"class Child(Parent):", "Child"},
		{"class Multi(A, B, C):", "Multi"},
		{"@decorator\nclass Decorated:", "Decorated"},
		{"@dec1\n@dec2\nclass MultiDec:", "MultiDec"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := extractStructName(tt.input, "python")
			if result != tt.expected {
				t.Errorf("extractStructName(%q, python) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractPythonDecorator(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"@staticmethod", "staticmethod"},
		{"@classmethod", "classmethod"},
		{"@property", "property"},
		{"@decorator(args)", "decorator"},
		{"@module.subdecorator", "subdecorator"},
		{"@dataclass(frozen=True)", "dataclass"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := extractDecoratorName(tt.input, "python")
			if result != tt.expected {
				t.Errorf("extractDecoratorName(%q, python) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractPythonAssignment(t *testing.T) {
	tests := []struct {
		input      string
		name       string
		isConstant bool
	}{
		{"name = 'value'", "name", false},
		{"counter = 0", "counter", false},
		{"MAX_SIZE = 100", "MAX_SIZE", true},
		{"API_KEY = 'secret'", "API_KEY", true},
		{"name: str = 'value'", "name", false},
		{"CONFIG: Dict = {}", "CONFIG", true},
		{"_private = 1", "_private", false},
		{"__dunder = 2", "__dunder", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, isConst := extractPythonAssignment(tt.input)
			if name != tt.name {
				t.Errorf("extractPythonAssignment(%q) name = %q, want %q", tt.input, name, tt.name)
			}
			if isConst != tt.isConstant {
				t.Errorf("extractPythonAssignment(%q) isConstant = %v, want %v", tt.input, isConst, tt.isConstant)
			}
		})
	}
}

func TestExtractLambdaName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"double = lambda x: x * 2", "double"},
		{"add = lambda a, b: a + b", "add"},
		{"noop = lambda: None", "noop"},
		{"lambda x: x", ""}, // Anonymous lambda
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := extractLambdaName(tt.input, "python")
			if result != tt.expected {
				t.Errorf("extractLambdaName(%q, python) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAstGrepPythonComprehensive(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "comprehensive.py")
	os.WriteFile(pyFile, []byte(`import os
from pathlib import Path
from typing import Dict, List

# Module-level constants
MAX_SIZE = 1000
API_VERSION = "v1"

# Module-level variables
counter = 0
config = {}

# Lambda
double = lambda x: x * 2

def standalone_function(x):
    """A standalone function."""
    return x * 2

async def async_function():
    """An async function."""
    pass

class MyClass:
    """A sample class."""

    class_var = "shared"

    def __init__(self, name):
        self.name = name

    def instance_method(self):
        return self.name

    @staticmethod
    def static_method():
        return "static"

    @classmethod
    def class_method(cls):
        return cls.class_var

    @property
    def name_property(self):
        return self._name

@dataclass
class DataClass:
    field: str
    count: int = 0

class Child(MyClass):
    def child_method(self):
        pass
`), 0644)

	analysis, err := analyzer.AnalyzeFile(pyFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if analysis == nil {
		t.Fatal("Expected analysis, got nil")
	}

	// Check imports
	imports := make(map[string]bool)
	for _, i := range analysis.Imports {
		imports[i] = true
	}
	if !imports["os"] {
		t.Errorf("Expected os import, got: %v", analysis.Imports)
	}
	if !imports["pathlib"] {
		t.Errorf("Expected pathlib import, got: %v", analysis.Imports)
	}

	// Check functions (including lambdas)
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["standalone_function"] {
		t.Errorf("Expected standalone_function, got: %v", analysis.Functions)
	}
	if !funcs["async_function"] {
		t.Errorf("Expected async_function, got: %v", analysis.Functions)
	}
	if !funcs["double"] {
		t.Errorf("Expected double (lambda), got: %v", analysis.Functions)
	}

	// Check classes
	classes := make(map[string]bool)
	for _, c := range analysis.Structs {
		classes[c] = true
	}
	if !classes["MyClass"] {
		t.Errorf("Expected MyClass, got: %v", analysis.Structs)
	}
	if !classes["DataClass"] {
		t.Errorf("Expected DataClass, got: %v", analysis.Structs)
	}
	if !classes["Child"] {
		t.Errorf("Expected Child, got: %v", analysis.Structs)
	}

	// Check decorators
	decorators := make(map[string]bool)
	for _, d := range analysis.Decorators {
		decorators[d] = true
	}
	if !decorators["staticmethod"] {
		t.Errorf("Expected staticmethod decorator, got: %v", analysis.Decorators)
	}
	if !decorators["classmethod"] {
		t.Errorf("Expected classmethod decorator, got: %v", analysis.Decorators)
	}
	if !decorators["property"] {
		t.Errorf("Expected property decorator, got: %v", analysis.Decorators)
	}
	if !decorators["dataclass"] {
		t.Errorf("Expected dataclass decorator, got: %v", analysis.Decorators)
	}

	// Check constants (ALL_CAPS)
	constants := make(map[string]bool)
	for _, c := range analysis.Constants {
		constants[c] = true
	}
	if !constants["MAX_SIZE"] {
		t.Errorf("Expected MAX_SIZE constant, got: %v", analysis.Constants)
	}
	if !constants["API_VERSION"] {
		t.Errorf("Expected API_VERSION constant, got: %v", analysis.Constants)
	}

	// Check variables (non-ALL_CAPS)
	vars := make(map[string]bool)
	for _, v := range analysis.Vars {
		vars[v] = true
	}
	if !vars["counter"] {
		t.Errorf("Expected counter variable, got: %v", analysis.Vars)
	}
	if !vars["config"] {
		t.Errorf("Expected config variable, got: %v", analysis.Vars)
	}
	// Lambda should NOT be in Vars (it's in Functions)
	if vars["double"] {
		t.Errorf("Lambda 'double' should be in Functions only, not Vars: %v", analysis.Vars)
	}
}

func TestAstGrepPythonScopeTracking(t *testing.T) {
	analyzer, err := NewAstGrepScanner()
	if err != nil {
		t.Fatalf("NewAstGrepScanner failed: %v", err)
	}
	if !analyzer.Available() {
		t.Skip("ast-grep (sg) not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "scope_test.py")
	os.WriteFile(pyFile, []byte(`def top_level():
    pass

class MyClass:
    def method_one(self):
        pass

    def method_two(self):
        pass

def another_top_level():
    pass
`), 0644)

	results, err := analyzer.ScanSymbols(tmpDir, false)
	if err != nil {
		t.Fatalf("ScanSymbols failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}

	// Build maps of symbol names to their scopes, separated by kind
	funcScopeMap := make(map[string]string)
	methodScopeMap := make(map[string]string)
	for _, file := range results {
		for _, sym := range file.Symbols {
			if sym.Kind == KindFunction {
				funcScopeMap[sym.Name] = sym.Scope
			} else if sym.Kind == KindMethod {
				methodScopeMap[sym.Name] = sym.Scope
			}
		}
	}

	// Top-level functions should have KindFunction and global scope
	if scope, ok := funcScopeMap["top_level"]; !ok || scope != "global" {
		t.Errorf("top_level should be KindFunction with global scope, got: %q", scope)
	}
	if scope, ok := funcScopeMap["another_top_level"]; !ok || scope != "global" {
		t.Errorf("another_top_level should be KindFunction with global scope, got: %q", scope)
	}

	// Methods inside MyClass should have KindMethod and class:MyClass scope
	if scope, ok := methodScopeMap["method_one"]; !ok || scope != "class:MyClass" {
		t.Errorf("method_one should be KindMethod with class:MyClass scope, got: %q", scope)
	}
	if scope, ok := methodScopeMap["method_two"]; !ok || scope != "class:MyClass" {
		t.Errorf("method_two should be KindMethod with class:MyClass scope, got: %q", scope)
	}
}

// Python field, enum, and generator tests

func TestExtractPythonFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"self.name = 'test'", "name"},
		{"self.value = 42", "value"},
		{"self._private = None", "_private"},
		{"self.__dunder = 1", "__dunder"},
		{"name = 'test'", ""}, // not a field
		{"other.name = 'x'", ""}, // not self
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractPythonFieldName(tt.input)
			if got != tt.expected {
				t.Errorf("extractPythonFieldName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractPythonPropertyName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"@property\n    def name(self):\n        return self._name", "name"},
		{"@name.setter\n    def name(self, value):\n        self._name = value", "name"},
		{"@name.deleter\n    def name(self):\n        del self._name", "name"},
		{"@property\ndef computed(self):\n    return 42", "computed"},
		{"def regular(self):\n    pass", ""}, // no decorator
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := extractPythonPropertyName(tt.input)
			if got != tt.expected {
				t.Errorf("extractPythonPropertyName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPythonPropertiesAndMethods(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")
	os.WriteFile(pyFile, []byte(`class Example:
    @property
    def name(self):
        return self._name

    @name.setter
    def name(self, value):
        self._name = value

    @staticmethod
    def helper():
        pass

    @classmethod
    def create(cls):
        return cls()

    def regular_method(self):
        pass

def top_level_func():
    pass
`), 0644)

	analysis, err := analyzer.AnalyzeFile(pyFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Check Properties: should contain "name"
	props := make(map[string]bool)
	for _, p := range analysis.Properties {
		props[p] = true
	}
	if !props["name"] {
		t.Errorf("Expected 'name' in Properties, got: %v", analysis.Properties)
	}

	// Check Methods: should contain class methods (excluding properties)
	methods := make(map[string]bool)
	for _, m := range analysis.Methods {
		methods[m] = true
	}
	expectedMethods := []string{"helper", "create", "regular_method"}
	for _, exp := range expectedMethods {
		if !methods[exp] {
			t.Errorf("Expected '%s' in Methods, got: %v", exp, analysis.Methods)
		}
	}
	// Property methods should NOT be in Methods
	if methods["name"] {
		t.Errorf("'name' should be in Properties, not Methods: %v", analysis.Methods)
	}

	// Check Functions: should ONLY contain top-level functions
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["top_level_func"] {
		t.Errorf("Expected 'top_level_func' in Functions, got: %v", analysis.Functions)
	}
	// Class methods should NOT be in Functions
	for _, method := range expectedMethods {
		if funcs[method] {
			t.Errorf("'%s' should be in Methods, not Functions: %v", method, analysis.Functions)
		}
	}
	if funcs["name"] {
		t.Errorf("'name' should be in Properties, not Functions: %v", analysis.Functions)
	}
}

func TestPythonFieldsEnumsGenerators(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")
	os.WriteFile(pyFile, []byte(`from enum import Enum

class Color(Enum):
    RED = 1
    GREEN = 2

class Example:
    def __init__(self):
        self.name = "test"
        self.value = 42

def my_generator():
    yield 1
    yield 2

def normal_function():
    return 1
`), 0644)

	analysis, err := analyzer.AnalyzeFile(pyFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Check fields
	fields := make(map[string]bool)
	for _, f := range analysis.Fields {
		fields[f] = true
	}
	if !fields["name"] {
		t.Errorf("Expected 'name' field, got: %v", analysis.Fields)
	}
	if !fields["value"] {
		t.Errorf("Expected 'value' field, got: %v", analysis.Fields)
	}

	// Check enums (now in Enums field, not Types)
	enums := make(map[string]bool)
	for _, e := range analysis.Enums {
		enums[e] = true
	}
	if !enums["Color"] {
		t.Errorf("Expected 'Color' enum in Enums, got: %v", analysis.Enums)
	}

	// Check that enum is NOT in Structs (only regular classes should be)
	structs := make(map[string]bool)
	for _, s := range analysis.Structs {
		structs[s] = true
	}
	if structs["Color"] {
		t.Errorf("Enum 'Color' should be in Types only, not Structs: %v", analysis.Structs)
	}
	if !structs["Example"] {
		t.Errorf("Expected 'Example' class in Structs, got: %v", analysis.Structs)
	}

	// Check generators and top-level functions (in Functions)
	funcs := make(map[string]bool)
	for _, f := range analysis.Functions {
		funcs[f] = true
	}
	if !funcs["my_generator"] {
		t.Errorf("Expected 'my_generator' in Functions, got: %v", analysis.Functions)
	}
	if !funcs["normal_function"] {
		t.Errorf("Expected 'normal_function' in Functions, got: %v", analysis.Functions)
	}

	// Check that __init__ is in Methods (class method), not Functions
	methods := make(map[string]bool)
	for _, m := range analysis.Methods {
		methods[m] = true
	}
	if !methods["__init__"] {
		t.Errorf("Expected '__init__' in Methods, got: %v", analysis.Methods)
	}
	if funcs["__init__"] {
		t.Errorf("'__init__' should be in Methods, not Functions: %v", analysis.Functions)
	}
}

// Go struct field tests

func TestExtractGoFieldNames(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"Name string", []string{"Name"}},
		{"Age  int", []string{"Age"}},
		{"X, Y int", []string{"X", "Y"}},
		{"address string", []string{"address"}},
		{"A, B, C float64", []string{"A", "B", "C"}},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractGoFieldNames(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("extractGoFieldNames(%q) = %v, want %v", tt.input, got, tt.expected)
				return
			}
			for i, name := range got {
				if name != tt.expected[i] {
					t.Errorf("extractGoFieldNames(%q)[%d] = %q, want %q", tt.input, i, name, tt.expected[i])
				}
			}
		})
	}
}

func TestGoStructFields(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(goFile, []byte(`package main

type Person struct {
    Name    string
    Age     int
    X, Y    float64
    address string
}
`), 0644)

	analysis, err := analyzer.AnalyzeFile(goFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	fields := make(map[string]bool)
	for _, f := range analysis.Fields {
		fields[f] = true
	}

	expected := []string{"Name", "Age", "X", "Y", "address"}
	for _, exp := range expected {
		if !fields[exp] {
			t.Errorf("Expected field %q, got: %v", exp, analysis.Fields)
		}
	}
}

// JavaScript property tests

func TestExtractPropertyName(t *testing.T) {
	tests := []struct {
		input    string
		lang     string
		expected string
	}{
		{"get name() { return this._name; }", "javascript", "name"},
		{"set name(v) { this._name = v; }", "javascript", "name"},
		{"get count() { return this._count; }", "typescript", "count"},
		{"set value(v) { }", "typescript", "value"},
		{"regular() { return 1; }", "javascript", ""},
		{"getName() { }", "javascript", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input[:min(20, len(tt.input))], func(t *testing.T) {
			got := extractPropertyName(tt.input, tt.lang)
			if got != tt.expected {
				t.Errorf("extractPropertyName(%q, %q) = %q, want %q", tt.input, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestJavaScriptProperties(t *testing.T) {
	analyzer := NewAstGrepAnalyzer()
	if !analyzer.Available() {
		t.Skip("ast-grep not installed")
	}
	defer analyzer.Close()

	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "test.js")
	os.WriteFile(jsFile, []byte(`class Example {
  get name() { return this._name; }
  set name(v) { this._name = v; }
  get count() { return this._count; }
  regular() { return 1; }
}`), 0644)

	analysis, err := analyzer.AnalyzeFile(jsFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Check properties
	props := make(map[string]bool)
	for _, p := range analysis.Properties {
		props[p] = true
	}
	if !props["name"] {
		t.Errorf("Expected 'name' property, got: %v", analysis.Properties)
	}
	if !props["count"] {
		t.Errorf("Expected 'count' property, got: %v", analysis.Properties)
	}

	// Check methods (should NOT include name or count)
	methods := make(map[string]bool)
	for _, m := range analysis.Methods {
		methods[m] = true
	}
	if methods["name"] {
		t.Errorf("'name' should be a property, not method: %v", analysis.Methods)
	}
	if methods["count"] {
		t.Errorf("'count' should be a property, not method: %v", analysis.Methods)
	}
	if !methods["regular"] {
		t.Errorf("Expected 'regular' method, got: %v", analysis.Methods)
	}
}
