package scanner

import (
	"embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed sg-rules/*.yml
var sgRules embed.FS

// ScanMatch represents a match from sg scan JSON output
type ScanMatch struct {
	File   string `json:"file"`
	RuleID string `json:"ruleId"`
	Range  struct {
		Start struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"start"`
	} `json:"range"`
	Text          string `json:"text"`
	Lines         string `json:"lines"` // Full line(s) containing the match
	MetaVariables struct {
		Single map[string]struct {
			Text string `json:"text"`
		} `json:"single"`
	} `json:"metaVariables"`
}

// AstGrepScanner uses ast-grep with YAML rules for code analysis
type AstGrepScanner struct {
	rulesDir string
	binary   string // "sg" or "ast-grep", whichever is available
}

// NewAstGrepScanner creates a scanner, extracting rules to temp dir
func NewAstGrepScanner() (*AstGrepScanner, error) {
	// Find ast-grep binary (installed as "sg" via brew, "ast-grep" via cargo/pipx)
	binary := findAstGrepBinary()

	// Create temp directory for rules
	rulesDir, err := os.MkdirTemp("", "codemap-sg-rules-*")
	if err != nil {
		return nil, err
	}

	// Extract embedded rules
	entries, err := sgRules.ReadDir("sg-rules")
	if err != nil {
		os.RemoveAll(rulesDir)
		return nil, err
	}

	for _, entry := range entries {
		content, err := sgRules.ReadFile("sg-rules/" + entry.Name())
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(rulesDir, entry.Name()), content, 0644)
	}

	return &AstGrepScanner{rulesDir: rulesDir, binary: binary}, nil
}

// findAstGrepBinary checks for "ast-grep" first, then "sg"
// Note: Linux has a system "sg" command (setgroups), so we check ast-grep first
func findAstGrepBinary() string {
	if _, err := exec.LookPath("ast-grep"); err == nil {
		return "ast-grep"
	}
	if _, err := exec.LookPath("sg"); err == nil {
		return "sg"
	}
	return ""
}

// Close cleans up temp rules directory
func (s *AstGrepScanner) Close() {
	if s.rulesDir != "" {
		os.RemoveAll(s.rulesDir)
	}
}

// Available checks if ast-grep CLI is available (as "sg" or "ast-grep")
func (s *AstGrepScanner) Available() bool {
	return s.binary != ""
}

// ScanDirectory analyzes all files in a directory using sg scan
func (s *AstGrepScanner) ScanDirectory(root string) ([]FileAnalysis, error) {
	if !s.Available() {
		return nil, nil
	}

	// Combine all rules into one string with --- separators
	var rules []string
	entries, _ := os.ReadDir(s.rulesDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yml") && e.Name() != "sgconfig.yml" {
			content, err := os.ReadFile(filepath.Join(s.rulesDir, e.Name()))
			if err == nil {
				rules = append(rules, string(content))
			}
		}
	}
	inlineRules := strings.Join(rules, "\n---\n")

	cmd := exec.Command(s.binary, "scan", "--inline-rules", inlineRules, "--json", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// sg scan returns non-zero if no matches, check if output is valid JSON
		if len(out) == 0 || !strings.HasPrefix(string(out), "[") {
			return nil, nil
		}
	}

	var matches []ScanMatch
	if err := json.Unmarshal(out, &matches); err != nil {
		return nil, err
	}

	// Group matches by file
	fileMap := make(map[string]*FileAnalysis)

	for _, m := range matches {
		relPath, _ := filepath.Rel(root, m.File)
		if relPath == "" {
			relPath = m.File
		}

		if fileMap[relPath] == nil {
			lang := detectLangFromRuleID(m.RuleID)
			fileMap[relPath] = &FileAnalysis{
				Path:     relPath,
				Language: lang,
			}
		}

		if strings.HasSuffix(m.RuleID, "-imports") {
			// Use metaVariable PATH if available, otherwise fall back to text extraction
			var mod string
			if pathVar, ok := m.MetaVariables.Single["PATH"]; ok && pathVar.Text != "" {
				mod = pathVar.Text
			} else {
				mod = extractImportPath(m.Text)
			}
			if mod != "" {
				fileMap[relPath].Imports = append(fileMap[relPath].Imports, mod)
			}
		} else if strings.HasSuffix(m.RuleID, "-arrow-functions") {
			// Arrow functions: extract name from the full line (const name = () => {})
			// Must check before -functions since -arrow-functions ends with -functions
			name := extractArrowFunctionName(m.Lines, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Functions = append(fileMap[relPath].Functions, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-function-signatures") {
			// Function signatures (declare function foo(): void)
			// Must check before -functions since it ends with -functions
			name := extractFunctionSignatureName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Functions = append(fileMap[relPath].Functions, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-generator-functions") {
			// Generator functions (function* name())
			// Must check before -functions since it ends with -functions
			name := extractGeneratorFunctionName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Functions = append(fileMap[relPath].Functions, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-functions") {
			// Extract function name from text
			name := extractFunctionName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Functions = append(fileMap[relPath].Functions, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-structs") {
			name := extractStructName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Structs = append(fileMap[relPath].Structs, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-interfaces") {
			name := extractInterfaceName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Interfaces = append(fileMap[relPath].Interfaces, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-methods") {
			name := extractMethodName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Methods = append(fileMap[relPath].Methods, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-constants") {
			names := extractConstantNames(m.Text, fileMap[relPath].Language)
			fileMap[relPath].Constants = append(fileMap[relPath].Constants, names...)
		} else if strings.HasSuffix(m.RuleID, "-types") {
			name := extractTypeName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Types = append(fileMap[relPath].Types, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-vars") {
			names := extractVarNames(m.Text, fileMap[relPath].Language)
			fileMap[relPath].Vars = append(fileMap[relPath].Vars, names...)
		} else if strings.HasSuffix(m.RuleID, "-classes") {
			name := extractStructName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Structs = append(fileMap[relPath].Structs, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-enums") {
			name := extractEnumName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Types = append(fileMap[relPath].Types, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-lexical") {
			// TypeScript/JavaScript lexical declarations (const/let/var)
			// Distinguish between const (-> Constants) and let/var (-> Vars)
			text := strings.TrimSpace(m.Text)
			if strings.HasPrefix(text, "const ") {
				names := extractConstantNames(m.Text, fileMap[relPath].Language)
				fileMap[relPath].Constants = append(fileMap[relPath].Constants, names...)
			} else if strings.HasPrefix(text, "let ") || strings.HasPrefix(text, "var ") {
				names := extractVarNames(m.Text, fileMap[relPath].Language)
				fileMap[relPath].Vars = append(fileMap[relPath].Vars, names...)
			}
		} else if strings.HasSuffix(m.RuleID, "-namespaces") {
			name := extractNamespaceName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Types = append(fileMap[relPath].Types, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-method-signatures") {
			// Interface method signatures and abstract method signatures
			name := extractMethodSignatureName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Methods = append(fileMap[relPath].Methods, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-property-signatures") {
			// Interface property signatures
			name := extractPropertySignatureName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Properties = append(fileMap[relPath].Properties, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-field-definitions") {
			// Class field definitions
			name := extractFieldDefinitionName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Fields = append(fileMap[relPath].Fields, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-decorators") {
			// Decorators (@Component, @Injectable, etc.)
			name := extractDecoratorName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Decorators = append(fileMap[relPath].Decorators, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-import-aliases") {
			// Import aliases: import X = require('x') or import X = Y.Z
			name := extractImportAliasName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Imports = append(fileMap[relPath].Imports, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-ambient-declarations") {
			// TypeScript ambient declarations: declare function/const/let/var/class/namespace/module/enum
			extractAmbientDeclaration(m.Text, fileMap[relPath])
		} else if strings.HasSuffix(m.RuleID, "-call-signatures") {
			// TypeScript call signatures: (): ReturnType in interfaces
			sig := extractCallSignature(m.Text)
			if sig != "" {
				fileMap[relPath].Methods = append(fileMap[relPath].Methods, sig)
			}
		} else if strings.HasSuffix(m.RuleID, "-construct-signatures") {
			// TypeScript construct signatures: new(): Type in interfaces
			sig := extractConstructSignature(m.Text)
			if sig != "" {
				fileMap[relPath].Methods = append(fileMap[relPath].Methods, sig)
			}
		} else if strings.HasSuffix(m.RuleID, "-index-signatures") {
			// TypeScript index signatures: [key: Type]: Type in interfaces
			sig := extractIndexSignature(m.Text)
			if sig != "" {
				fileMap[relPath].Properties = append(fileMap[relPath].Properties, sig)
			}
		} else if strings.HasSuffix(m.RuleID, "-variable-declarations") {
			// var declarations (legacy)
			names := extractVarDeclarationNames(m.Text, fileMap[relPath].Language)
			fileMap[relPath].Vars = append(fileMap[relPath].Vars, names...)
		} else if strings.HasSuffix(m.RuleID, "-function-expressions") {
			// Named function expressions: const f = function myFunc() {}
			name := extractFunctionExpressionName(m.Text, fileMap[relPath].Language)
			if name != "" {
				fileMap[relPath].Functions = append(fileMap[relPath].Functions, name)
			}
		} else if strings.HasSuffix(m.RuleID, "-static-blocks") {
			// Class static blocks: static { ... }
			// These don't have names, but we track them as a special method
			fileMap[relPath].Methods = append(fileMap[relPath].Methods, "(static block)")
		}
	}

	// Convert map to slice and dedupe
	var results []FileAnalysis
	for _, a := range fileMap {
		a.Functions = dedupe(a.Functions)
		a.Imports = dedupe(a.Imports)
		a.Structs = dedupe(a.Structs)
		a.Interfaces = dedupe(a.Interfaces)
		a.Methods = dedupe(a.Methods)
		a.Constants = dedupe(a.Constants)
		a.Types = dedupe(a.Types)
		a.Vars = dedupe(a.Vars)
		a.Fields = dedupe(a.Fields)
		a.Properties = dedupe(a.Properties)
		a.Decorators = dedupe(a.Decorators)
		results = append(results, *a)
	}

	return results, nil
}

func detectLangFromRuleID(ruleID string) string {
	parts := strings.Split(ruleID, "-")
	if len(parts) > 0 {
		switch parts[0] {
		case "go":
			return "go"
		case "ts":
			return "typescript"
		case "js":
			return "javascript"
		case "py":
			return "python"
		case "rust":
			return "rust"
		case "java":
			return "java"
		case "ruby":
			return "ruby"
		case "swift":
			return "swift"
		case "kotlin":
			return "kotlin"
		case "c":
			return "c"
		case "cpp":
			return "cpp"
		case "bash":
			return "bash"
		}
	}
	return ""
}

func extractImportPath(text string) string {
	// Handle various import formats
	text = strings.TrimSpace(text)

	// C/C++: #include <header> or #include "header"
	if strings.HasPrefix(text, "#include") {
		// Try angle brackets first
		if start := strings.Index(text, "<"); start >= 0 {
			if end := strings.Index(text[start:], ">"); end > 0 {
				return text[start+1 : start+end]
			}
		}
	}

	// Bash: source ./file or . ./file
	if strings.HasPrefix(text, "source ") || strings.HasPrefix(text, ". ") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	// Find quoted strings (Go, TS/JS, Python, C/C++ with quotes)
	for _, q := range []string{`"`, `'`, "`"} {
		if idx := strings.Index(text, q); idx >= 0 {
			end := strings.Index(text[idx+1:], q)
			if end > 0 {
				return text[idx+1 : idx+1+end]
			}
		}
	}

	// Python: import foo
	if strings.HasPrefix(text, "import ") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	// Python: from foo import bar
	if strings.HasPrefix(text, "from ") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	// Rust: use foo::bar;
	if strings.HasPrefix(text, "use ") {
		text = strings.TrimPrefix(text, "use ")
		text = strings.TrimSuffix(text, ";")
		// Get root module
		if idx := strings.Index(text, "::"); idx > 0 {
			return text[:idx]
		}
		return strings.TrimSpace(text)
	}

	// Java: import foo.bar.Baz;
	if strings.HasPrefix(text, "import ") {
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSuffix(text, ";")
		text = strings.TrimSpace(text)
		// Get package (everything except last part)
		if idx := strings.LastIndex(text, "."); idx > 0 {
			return text[:idx]
		}
		return text
	}

	return ""
}

func extractFunctionName(text string, lang string) string {
	text = strings.TrimSpace(text)

	switch lang {
	case "go":
		// func Name(...) or func (r Receiver) Name(...)
		if strings.HasPrefix(text, "func ") {
			text = strings.TrimPrefix(text, "func ")
			// Skip receiver if present
			if strings.HasPrefix(text, "(") {
				if idx := strings.Index(text, ")"); idx > 0 {
					text = strings.TrimSpace(text[idx+1:])
				}
			}
			// Get function name (up to paren)
			if idx := strings.Index(text, "("); idx > 0 {
				return text[:idx]
			}
		}

	case "typescript", "javascript":
		// function name(...) or async function name(...)
		if strings.Contains(text, "function ") {
			idx := strings.Index(text, "function ") + 9
			text = text[idx:]
			if paren := strings.Index(text, "("); paren > 0 {
				return strings.TrimSpace(text[:paren])
			}
		}
		// Method definitions: [modifiers] name(...) or get/set name(...)
		if paren := strings.Index(text, "("); paren > 0 {
			name := strings.TrimSpace(text[:paren])
			// Strip TypeScript/JS modifiers
			for _, mod := range []string{"public ", "private ", "protected ", "static ", "readonly ", "async ", "get ", "set ", "override "} {
				name = strings.TrimPrefix(name, mod)
			}
			// Skip control flow keywords
			if name == "if" || name == "for" || name == "while" || name == "switch" || name == "catch" || name == "" {
				return ""
			}
			if isValidIdentifier(name) {
				return name
			}
		}

	case "python":
		// def name(...):
		if strings.HasPrefix(text, "def ") {
			text = strings.TrimPrefix(text, "def ")
			if paren := strings.Index(text, "("); paren > 0 {
				return text[:paren]
			}
		}

	case "rust":
		// fn name(...) or pub fn name(...)
		if idx := strings.Index(text, "fn "); idx >= 0 {
			text = text[idx+3:]
			if paren := strings.Index(text, "("); paren > 0 {
				name := text[:paren]
				// Handle generics
				if bracket := strings.Index(name, "<"); bracket > 0 {
					name = name[:bracket]
				}
				return name
			}
		}

	case "java":
		// public void name(...) - method declaration
		// Find last word before (
		if paren := strings.Index(text, "("); paren > 0 {
			before := strings.TrimSpace(text[:paren])
			parts := strings.Fields(before)
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}

	case "ruby":
		// def name or def name(...)
		if strings.HasPrefix(text, "def ") {
			text = strings.TrimPrefix(text, "def ")
			if paren := strings.Index(text, "("); paren > 0 {
				return text[:paren]
			}
			// No parens - take first word
			if space := strings.Index(text, " "); space > 0 {
				return text[:space]
			}
			if newline := strings.Index(text, "\n"); newline > 0 {
				return text[:newline]
			}
			return text
		}

	case "swift", "kotlin":
		// func name(...) or fun name(...)
		for _, prefix := range []string{"func ", "fun "} {
			if idx := strings.Index(text, prefix); idx >= 0 {
				text = text[idx+len(prefix):]
				if paren := strings.Index(text, "("); paren > 0 {
					name := text[:paren]
					if bracket := strings.Index(name, "<"); bracket > 0 {
						name = name[:bracket]
					}
					return strings.TrimSpace(name)
				}
			}
		}

	case "c", "cpp":
		// type name(...) - find last identifier before (
		if paren := strings.Index(text, "("); paren > 0 {
			before := strings.TrimSpace(text[:paren])
			// Handle pointers: int *foo -> foo
			before = strings.TrimLeft(before, "*")
			parts := strings.Fields(before)
			if len(parts) > 0 {
				name := parts[len(parts)-1]
				name = strings.TrimLeft(name, "*")
				if isValidIdentifier(name) {
					return name
				}
			}
		}

	case "bash":
		// function name() or name()
		text = strings.TrimPrefix(text, "function ")
		if paren := strings.Index(text, "("); paren > 0 {
			name := strings.TrimSpace(text[:paren])
			if isValidIdentifier(name) {
				return name
			}
		}
	}

	return ""
}

func extractStructName(text string, lang string) string {
	// Go: type Name struct { ... }
	if lang == "go" {
		if idx := strings.Index(text, "type "); idx >= 0 {
			text = text[idx+5:]
			if space := strings.IndexAny(text, " \t"); space > 0 {
				return strings.TrimSpace(text[:space])
			}
		}
	}
	// TypeScript/JavaScript: class Name { ... } or abstract class Name { ... }
	if lang == "typescript" || lang == "javascript" {
		text = strings.TrimSpace(text)
		// Skip decorators at the start (lines starting with @)
		for strings.HasPrefix(text, "@") {
			if newline := strings.Index(text, "\n"); newline > 0 {
				text = strings.TrimSpace(text[newline+1:])
			} else {
				break
			}
		}
		// Handle: export default class, export class, abstract class, class
		text = strings.TrimPrefix(text, "export ")
		text = strings.TrimPrefix(text, "default ")
		text = strings.TrimPrefix(text, "abstract ")
		if strings.HasPrefix(text, "class ") {
			text = strings.TrimPrefix(text, "class ")
			// Get class name (up to space, <, {, or newline)
			for i, c := range text {
				if c == ' ' || c == '<' || c == '{' || c == '\n' {
					return text[:i]
				}
			}
			return text // whole string if no delimiter found
		}
	}
	return ""
}

func extractInterfaceName(text string, lang string) string {
	// Go: type Name interface { ... }
	if lang == "go" {
		if idx := strings.Index(text, "type "); idx >= 0 {
			text = text[idx+5:]
			if space := strings.IndexAny(text, " \t"); space > 0 {
				return strings.TrimSpace(text[:space])
			}
		}
	}
	// TypeScript: interface Name { ... } or export interface Name { ... }
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "export ")
		if strings.HasPrefix(text, "interface ") {
			text = strings.TrimPrefix(text, "interface ")
			// Get interface name (up to space, <, {, or newline)
			for i, c := range text {
				if c == ' ' || c == '<' || c == '{' || c == '\n' {
					return text[:i]
				}
			}
			return text
		}
	}
	return ""
}

func extractMethodName(text string, lang string) string {
	// Go: func (r *Receiver) Name(...) ...
	if lang == "go" {
		if strings.HasPrefix(text, "func ") {
			text = strings.TrimPrefix(text, "func ")
			// Skip receiver: (r *Type)
			if strings.HasPrefix(text, "(") {
				if idx := strings.Index(text, ")"); idx > 0 {
					text = strings.TrimSpace(text[idx+1:])
				}
			}
			// Get method name
			if paren := strings.Index(text, "("); paren > 0 {
				return strings.TrimSpace(text[:paren])
			}
		}
	}
	// TypeScript/JavaScript: methodName(...) { ... } or async methodName(...) or get/set name()
	if lang == "typescript" || lang == "javascript" {
		text = strings.TrimSpace(text)
		// Strip modifiers (loop to handle multiple)
		for {
			found := false
			for _, mod := range []string{"public ", "private ", "protected ", "static ", "readonly ", "async ", "override ", "abstract "} {
				if strings.HasPrefix(text, mod) {
					text = strings.TrimPrefix(text, mod)
					found = true
				}
			}
			if !found {
				break
			}
		}
		// Handle getters/setters: get name() or set name()
		text = strings.TrimPrefix(text, "get ")
		text = strings.TrimPrefix(text, "set ")
		// Handle generator: *methodName()
		text = strings.TrimPrefix(text, "*")
		text = strings.TrimSpace(text)
		// Get method name (up to parenthesis or <)
		for i, c := range text {
			if c == '(' || c == '<' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					return name
				}
				break
			}
		}
	}
	return ""
}

func extractConstantNames(text string, lang string) []string {
	var names []string
	// Go: const Name = value OR const ( Name = value; Name2 = value2 )
	if lang == "go" {
		text = strings.TrimPrefix(text, "const ")
		text = strings.TrimSpace(text)
		// Handle const block: const ( ... )
		if strings.HasPrefix(text, "(") {
			// Parse const block - extract each identifier before = or newline
			text = strings.TrimPrefix(text, "(")
			text = strings.TrimSuffix(text, ")")
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "//") {
					continue
				}
				// Handle: Name = value or Name Type = value
				if eq := strings.Index(line, "="); eq > 0 {
					part := strings.TrimSpace(line[:eq])
					fields := strings.Fields(part)
					if len(fields) > 0 {
						name := fields[0]
						if isValidIdentifier(name) {
							names = append(names, name)
						}
					}
				} else {
					// Handle: Name (for iota patterns)
					fields := strings.Fields(line)
					if len(fields) > 0 {
						name := fields[0]
						if isValidIdentifier(name) {
							names = append(names, name)
						}
					}
				}
			}
		} else {
			// Single const: const Name = value or const Name Type = value
			if eq := strings.Index(text, "="); eq > 0 {
				part := strings.TrimSpace(text[:eq])
				fields := strings.Fields(part)
				if len(fields) > 0 {
					name := fields[0]
					if isValidIdentifier(name) {
						names = append(names, name)
					}
				}
			}
		}
	}
	// TypeScript/JavaScript: const name = ... or const name: Type = ...
	if lang == "typescript" || lang == "javascript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "export ")
		if strings.HasPrefix(text, "const ") {
			text = strings.TrimPrefix(text, "const ")
			// Skip destructuring: const { a, b } = ... or const [a, b] = ...
			if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
				return names
			}
			// Simple: const name = ... or const name: Type = ...
			for i, c := range text {
				if c == ' ' || c == ':' || c == '=' {
					name := strings.TrimSpace(text[:i])
					if isValidIdentifier(name) {
						names = append(names, name)
					}
					break
				}
			}
		}
	}
	return names
}

func extractTypeName(text string, lang string) string {
	// Go: same as struct extraction for type aliases
	if lang == "go" {
		return extractStructName(text, lang)
	}
	// TypeScript: type Name = ... or export type Name = ...
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "export ")
		if strings.HasPrefix(text, "type ") {
			text = strings.TrimPrefix(text, "type ")
			// Get type name (up to space, <, =)
			for i, c := range text {
				if c == ' ' || c == '<' || c == '=' {
					return strings.TrimSpace(text[:i])
				}
			}
			return text
		}
	}
	return ""
}

func extractEnumName(text string, lang string) string {
	// TypeScript: enum Name { ... } or export enum Name { ... } or const enum Name { ... }
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "export ")
		text = strings.TrimPrefix(text, "const ")
		text = strings.TrimPrefix(text, "declare ")
		if strings.HasPrefix(text, "enum ") {
			text = strings.TrimPrefix(text, "enum ")
			// Get enum name (up to space, {, or newline)
			for i, c := range text {
				if c == ' ' || c == '{' || c == '\n' {
					return text[:i]
				}
			}
			return text
		}
	}
	return ""
}

func extractArrowFunctionName(lines string, lang string) string {
	// TypeScript/JavaScript: const name = () => {} or const name = async () => {}
	// The `lines` field contains the full source line(s)
	if lang == "typescript" || lang == "javascript" {
		line := strings.TrimSpace(lines)
		// Handle export
		line = strings.TrimPrefix(line, "export ")
		// Must be const/let/var assignment
		for _, keyword := range []string{"const ", "let ", "var "} {
			if strings.HasPrefix(line, keyword) {
				line = strings.TrimPrefix(line, keyword)
				// Find the variable name (up to space, :, or =)
				for i, c := range line {
					if c == ' ' || c == ':' || c == '=' {
						name := strings.TrimSpace(line[:i])
						if isValidIdentifier(name) {
							return name
						}
						break
					}
				}
				break
			}
		}
	}
	return ""
}

func extractNamespaceName(text string, lang string) string {
	// TypeScript: namespace Name { ... } or module Name { ... }
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "export ")
		text = strings.TrimPrefix(text, "declare ")
		for _, keyword := range []string{"namespace ", "module "} {
			if strings.HasPrefix(text, keyword) {
				text = strings.TrimPrefix(text, keyword)
				// Skip string module names like module "foo" or module 'foo'
				if strings.HasPrefix(text, "\"") || strings.HasPrefix(text, "'") {
					return ""
				}
				// Get namespace name (up to space, {, or newline)
				for i, c := range text {
					if c == ' ' || c == '{' || c == '\n' {
						return text[:i]
					}
				}
				return text
			}
		}
	}
	return ""
}

func extractFunctionSignatureName(text string, lang string) string {
	// TypeScript: function name(...): Type; (ambient declaration)
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "function ") {
			text = strings.TrimPrefix(text, "function ")
			// Get function name (up to parenthesis or <)
			for i, c := range text {
				if c == '(' || c == '<' {
					name := strings.TrimSpace(text[:i])
					if isValidIdentifier(name) {
						return name
					}
					break
				}
			}
		}
	}
	return ""
}

func extractMethodSignatureName(text string, lang string) string {
	// TypeScript: methodName(): Type; or abstract methodName(): Type;
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "abstract ")
		// Get method name (up to parenthesis, <, or ?)
		for i, c := range text {
			if c == '(' || c == '<' || c == '?' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					return name
				}
				break
			}
		}
	}
	return ""
}

func extractPropertySignatureName(text string, lang string) string {
	// TypeScript: propertyName: Type; or propertyName?: Type;
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "readonly ")
		// Get property name (up to :, ?, or whitespace)
		for i, c := range text {
			if c == ':' || c == '?' || c == ' ' || c == '\t' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					return name
				}
				break
			}
		}
	}
	return ""
}

func extractFieldDefinitionName(text string, lang string) string {
	// TypeScript: [modifiers] fieldName: Type = value;
	if lang == "typescript" || lang == "javascript" {
		text = strings.TrimSpace(text)
		// Strip decorators (lines starting with @)
		for strings.HasPrefix(text, "@") {
			// Find end of decorator - either newline or closing paren followed by space
			if paren := strings.Index(text, ")"); paren > 0 {
				// Skip past the closing paren and any following whitespace
				text = strings.TrimSpace(text[paren+1:])
			} else if newline := strings.Index(text, "\n"); newline > 0 {
				text = strings.TrimSpace(text[newline+1:])
			} else if space := strings.Index(text, " "); space > 0 {
				// Simple decorator without parens: @readonly fieldName
				text = strings.TrimSpace(text[space+1:])
			} else {
				return ""
			}
		}
		// Strip modifiers
		for {
			found := false
			for _, mod := range []string{"public ", "private ", "protected ", "static ", "readonly ", "abstract ", "override ", "declare "} {
				if strings.HasPrefix(text, mod) {
					text = strings.TrimPrefix(text, mod)
					found = true
				}
			}
			if !found {
				break
			}
		}
		// Get field name (up to :, =, !, or whitespace)
		for i, c := range text {
			if c == ':' || c == '=' || c == '!' || c == ' ' || c == '\t' || c == ';' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					return name
				}
				break
			}
		}
	}
	return ""
}

func extractDecoratorName(text string, lang string) string {
	// TypeScript: @DecoratorName or @DecoratorName(args)
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "@") {
			text = strings.TrimPrefix(text, "@")
			// Get decorator name (up to parenthesis, newline, or whitespace)
			for i, c := range text {
				if c == '(' || c == '\n' || c == ' ' || c == '\t' {
					name := strings.TrimSpace(text[:i])
					if isValidIdentifier(name) {
						return name
					}
					break
				}
			}
			// No delimiter found - whole text is the name
			if isValidIdentifier(text) {
				return text
			}
		}
	}
	return ""
}

func extractGeneratorFunctionName(text string, lang string) string {
	// TypeScript/JavaScript: function* name() or async function* name()
	if lang == "typescript" || lang == "javascript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "export ")
		text = strings.TrimPrefix(text, "async ")
		if strings.HasPrefix(text, "function* ") || strings.HasPrefix(text, "function *") {
			// Find the * and skip past it
			idx := strings.Index(text, "*")
			if idx >= 0 {
				text = strings.TrimSpace(text[idx+1:])
				// Get function name (up to parenthesis or <)
				for i, c := range text {
					if c == '(' || c == '<' {
						name := strings.TrimSpace(text[:i])
						if isValidIdentifier(name) {
							return name
						}
						break
					}
				}
			}
		}
	}
	return ""
}

func extractImportAliasName(text string, lang string) string {
	// TypeScript: import X = require('x') or import X = Y.Z
	if lang == "typescript" {
		text = strings.TrimSpace(text)
		// Handle import_require_clause: X = require('module')
		if strings.Contains(text, "require(") {
			// Extract: X = require('module') -> module
			if idx := strings.Index(text, "require("); idx >= 0 {
				start := idx + 8 // len("require(")
				// Find the quoted module name
				for _, q := range []string{"'", "\"", "`"} {
					if qStart := strings.Index(text[start:], q); qStart >= 0 {
						qEnd := strings.Index(text[start+qStart+1:], q)
						if qEnd >= 0 {
							return text[start+qStart+1 : start+qStart+1+qEnd]
						}
					}
				}
			}
		}
		// Handle import_alias: import X = Y.Z -> Y.Z (or just Y)
		text = strings.TrimPrefix(text, "import ")
		if eq := strings.Index(text, "="); eq > 0 {
			// Get the right side of the assignment
			rhs := strings.TrimSpace(text[eq+1:])
			rhs = strings.TrimSuffix(rhs, ";")
			// Return the root module/namespace
			if dot := strings.Index(rhs, "."); dot > 0 {
				return rhs[:dot]
			}
			return rhs
		}
	}
	return ""
}

func extractVarNames(text string, lang string) []string {
	var names []string
	// Go: var Name = value OR var ( Name = value; Name2 = value2 )
	if lang == "go" {
		text = strings.TrimPrefix(text, "var ")
		text = strings.TrimSpace(text)
		// Handle var block: var ( ... )
		if strings.HasPrefix(text, "(") {
			// Parse var block - extract each identifier before = or type
			text = strings.TrimPrefix(text, "(")
			text = strings.TrimSuffix(text, ")")
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "//") {
					continue
				}
				// Handle: Name = value or Name Type = value or Name Type
				if eq := strings.Index(line, "="); eq > 0 {
					part := strings.TrimSpace(line[:eq])
					fields := strings.Fields(part)
					if len(fields) > 0 {
						name := fields[0]
						if isValidIdentifier(name) {
							names = append(names, name)
						}
					}
				} else {
					// Handle: Name Type (no initializer)
					fields := strings.Fields(line)
					if len(fields) > 0 {
						name := fields[0]
						if isValidIdentifier(name) {
							names = append(names, name)
						}
					}
				}
			}
		} else {
			// Single var: var Name = value or var Name Type = value or var Name Type
			if eq := strings.Index(text, "="); eq > 0 {
				part := strings.TrimSpace(text[:eq])
				fields := strings.Fields(part)
				if len(fields) > 0 {
					name := fields[0]
					if isValidIdentifier(name) {
						names = append(names, name)
					}
				}
			} else {
				// No initializer: var Name Type
				fields := strings.Fields(text)
				if len(fields) > 0 {
					name := fields[0]
					if isValidIdentifier(name) {
						names = append(names, name)
					}
				}
			}
		}
	}
	// TypeScript/JavaScript: let name = ... or var name = ...
	if lang == "typescript" || lang == "javascript" {
		text = strings.TrimSpace(text)
		text = strings.TrimPrefix(text, "export ")
		for _, keyword := range []string{"let ", "var "} {
			if strings.HasPrefix(text, keyword) {
				text = strings.TrimPrefix(text, keyword)
				// Skip destructuring: let { a, b } = ... or let [a, b] = ...
				if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
					return names
				}
				// Simple: let/var name = ... or let/var name: Type = ...
				for i, c := range text {
					if c == ' ' || c == ':' || c == '=' {
						name := strings.TrimSpace(text[:i])
						if isValidIdentifier(name) {
							names = append(names, name)
						}
						break
					}
				}
				break
			}
		}
	}
	return names
}

// extractAmbientDeclaration parses TypeScript ambient declarations (declare ...)
// and adds the extracted symbol to the appropriate field in FileAnalysis
func extractAmbientDeclaration(text string, fa *FileAnalysis) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "declare ") {
		return
	}
	text = strings.TrimPrefix(text, "declare ")

	// declare function name(...): ...
	if strings.HasPrefix(text, "function ") {
		text = strings.TrimPrefix(text, "function ")
		for i, c := range text {
			if c == '(' || c == '<' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					fa.Functions = append(fa.Functions, name)
				}
				return
			}
		}
		return
	}

	// declare const name: Type
	if strings.HasPrefix(text, "const ") {
		text = strings.TrimPrefix(text, "const ")
		for i, c := range text {
			if c == ':' || c == '=' || c == ';' || c == ' ' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					fa.Constants = append(fa.Constants, name)
				}
				return
			}
		}
		return
	}

	// declare let/var name: Type
	for _, keyword := range []string{"let ", "var "} {
		if strings.HasPrefix(text, keyword) {
			text = strings.TrimPrefix(text, keyword)
			for i, c := range text {
				if c == ':' || c == '=' || c == ';' || c == ' ' {
					name := strings.TrimSpace(text[:i])
					if isValidIdentifier(name) {
						fa.Vars = append(fa.Vars, name)
					}
					return
				}
			}
			return
		}
	}

	// declare class Name { ... }
	if strings.HasPrefix(text, "class ") {
		text = strings.TrimPrefix(text, "class ")
		for i, c := range text {
			if c == ' ' || c == '<' || c == '{' || c == '\n' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					fa.Structs = append(fa.Structs, name)
				}
				return
			}
		}
		return
	}

	// declare abstract class Name { ... }
	if strings.HasPrefix(text, "abstract class ") {
		text = strings.TrimPrefix(text, "abstract class ")
		for i, c := range text {
			if c == ' ' || c == '<' || c == '{' || c == '\n' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					fa.Structs = append(fa.Structs, name)
				}
				return
			}
		}
		return
	}

	// declare interface Name { ... }
	if strings.HasPrefix(text, "interface ") {
		text = strings.TrimPrefix(text, "interface ")
		for i, c := range text {
			if c == ' ' || c == '<' || c == '{' || c == '\n' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					fa.Interfaces = append(fa.Interfaces, name)
				}
				return
			}
		}
		return
	}

	// declare namespace/module Name { ... }
	for _, keyword := range []string{"namespace ", "module "} {
		if strings.HasPrefix(text, keyword) {
			text = strings.TrimPrefix(text, keyword)
			// Skip string module names like module "foo"
			if strings.HasPrefix(text, "\"") || strings.HasPrefix(text, "'") {
				return
			}
			for i, c := range text {
				if c == ' ' || c == '{' || c == '\n' {
					name := strings.TrimSpace(text[:i])
					if isValidIdentifier(name) {
						fa.Types = append(fa.Types, name)
					}
					return
				}
			}
			return
		}
	}

	// declare enum Name { ... }
	if strings.HasPrefix(text, "enum ") {
		text = strings.TrimPrefix(text, "enum ")
		for i, c := range text {
			if c == ' ' || c == '{' || c == '\n' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					fa.Types = append(fa.Types, name)
				}
				return
			}
		}
		return
	}

	// declare type Name = ...
	if strings.HasPrefix(text, "type ") {
		text = strings.TrimPrefix(text, "type ")
		for i, c := range text {
			if c == ' ' || c == '<' || c == '=' {
				name := strings.TrimSpace(text[:i])
				if isValidIdentifier(name) {
					fa.Types = append(fa.Types, name)
				}
				return
			}
		}
		return
	}
}

// extractCallSignature extracts call signature from TypeScript interface
// Input: "(): void" or "(x: number): string"
// Returns: "()" to indicate a callable
func extractCallSignature(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "(") {
		return "()"
	}
	return ""
}

// extractConstructSignature extracts construct signature from TypeScript interface
// Input: "new(): object" or "new(x: string): MyClass"
// Returns: "new()" to indicate a constructable
func extractConstructSignature(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "new(") || strings.HasPrefix(text, "new (") {
		return "new()"
	}
	return ""
}

// extractIndexSignature extracts index signature from TypeScript interface
// Input: "[key: string]: number" or "[index: number]: string"
// Returns: "[string]" or "[number]" to indicate the key type
func extractIndexSignature(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "[") {
		return ""
	}
	// Find the colon after the key name
	colonIdx := strings.Index(text, ":")
	if colonIdx < 0 {
		return ""
	}
	// Find the type between : and ]
	closeBracket := strings.Index(text, "]")
	if closeBracket < 0 || closeBracket < colonIdx {
		return ""
	}
	keyType := strings.TrimSpace(text[colonIdx+1 : closeBracket])
	return "[" + keyType + "]"
}

// extractVarDeclarationNames extracts variable names from var declarations
// Input: "var x = 1" or "var x = 1, y = 2"
func extractVarDeclarationNames(text string, lang string) []string {
	var names []string
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "var ") {
		return names
	}
	text = strings.TrimPrefix(text, "var ")

	// Handle multiple declarations: var x = 1, y = 2
	// Split by comma, but be careful of commas in values
	depth := 0
	start := 0
	for i, c := range text {
		if c == '(' || c == '{' || c == '[' {
			depth++
		} else if c == ')' || c == '}' || c == ']' {
			depth--
		} else if c == ',' && depth == 0 {
			name := extractSingleVarName(text[start:i])
			if name != "" {
				names = append(names, name)
			}
			start = i + 1
		}
	}
	// Handle last (or only) declaration
	if start < len(text) {
		name := extractSingleVarName(text[start:])
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// extractSingleVarName extracts a single variable name from "name = value" or "name: Type = value"
func extractSingleVarName(text string) string {
	text = strings.TrimSpace(text)
	// Skip destructuring
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return ""
	}
	for i, c := range text {
		if c == ' ' || c == ':' || c == '=' || c == ';' {
			name := strings.TrimSpace(text[:i])
			if isValidIdentifier(name) {
				return name
			}
			return ""
		}
	}
	// No delimiter found, check if entire text is valid identifier
	name := strings.TrimSuffix(strings.TrimSpace(text), ";")
	if isValidIdentifier(name) {
		return name
	}
	return ""
}

// extractFunctionExpressionName extracts the name from a named function expression
// Input: "function myFunc() { ... }" (the function expression part)
// Returns: "myFunc" or "" if anonymous
func extractFunctionExpressionName(text string, lang string) string {
	text = strings.TrimSpace(text)
	// Handle async function expressions
	text = strings.TrimPrefix(text, "async ")
	if !strings.HasPrefix(text, "function") {
		return ""
	}
	text = strings.TrimPrefix(text, "function")
	// Check for generator: function*
	text = strings.TrimPrefix(text, "*")
	text = strings.TrimSpace(text)

	// If starts with (, it's anonymous
	if strings.HasPrefix(text, "(") {
		return ""
	}

	// Extract name up to ( or <
	for i, c := range text {
		if c == '(' || c == '<' {
			name := strings.TrimSpace(text[:i])
			if isValidIdentifier(name) {
				return name
			}
			return ""
		}
	}
	return ""
}

func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
	}
	return true
}

// Legacy exports for compatibility
type AstGrepAnalyzer = AstGrepScanner

func NewAstGrepAnalyzer() *AstGrepAnalyzer {
	s, _ := NewAstGrepScanner()
	return s
}

func (s *AstGrepScanner) AnalyzeFile(filePath string) (*FileAnalysis, error) {
	results, err := s.ScanDirectory(filepath.Dir(filePath))
	if err != nil {
		return nil, err
	}
	base := filepath.Base(filePath)
	for _, r := range results {
		if filepath.Base(r.Path) == base {
			return &r, nil
		}
	}
	return nil, nil
}
