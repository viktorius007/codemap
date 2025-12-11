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
	if !types["Status"] {
		t.Errorf("Expected Status enum in Types, got: %v", analysis.Types)
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

	// Check types (includes enums, namespaces, type aliases)
	types := make(map[string]bool)
	for _, ty := range analysis.Types {
		types[ty] = true
	}
	if !types["Utils"] {
		t.Errorf("Expected Utils namespace, got: %v", analysis.Types)
	}
	if !types["Color"] {
		t.Errorf("Expected Color enum, got: %v", analysis.Types)
	}
	if !types["ID"] {
		t.Errorf("Expected ID type alias, got: %v", analysis.Types)
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
