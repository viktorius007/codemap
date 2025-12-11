package scanner

import (
	"bytes"
	"embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// ScanDirectoryV2 analyzes all files using the enhanced Symbol struct with metadata
func (s *AstGrepScanner) ScanDirectoryV2(root string, includeRefs bool) ([]FileAnalysisV2, error) {
	if !s.Available() {
		return nil, nil
	}

	// Create file cache for scope resolution
	cache := newFileCache()

	// Extract scope containers first (two-pass approach)
	containers, err := s.extractScopeContainers(root, cache)
	if err != nil {
		// Continue without scope resolution if container extraction fails
		containers = make(map[string][]ScopeContainer)
	}

	// Combine all rules into one string with --- separators
	var rules []string
	entries, _ := os.ReadDir(s.rulesDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yml") && e.Name() != "sgconfig.yml" {
			// Skip reference rules unless requested
			if !includeRefs && strings.Contains(e.Name(), "-refs") {
				continue
			}
			// Skip container rules (they're only used for scope extraction)
			if strings.Contains(e.Name(), "-containers") {
				continue
			}
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
		if len(out) == 0 || !strings.HasPrefix(string(out), "[") {
			return nil, nil
		}
	}

	var matches []ScanMatch
	if err := json.Unmarshal(out, &matches); err != nil {
		return nil, err
	}

	// Group matches by file
	fileMap := make(map[string]*FileAnalysisV2)

	for _, m := range matches {
		relPath, _ := filepath.Rel(root, m.File)
		if relPath == "" {
			relPath = m.File
		}

		if fileMap[relPath] == nil {
			lang := detectLangFromRuleID(m.RuleID)
			fileMap[relPath] = &FileAnalysisV2{
				Path:     relPath,
				Language: lang,
				Symbols:  []Symbol{},
			}
		}

		// Extract symbol with full metadata
		sym := extractSymbolV2(m, fileMap[relPath].Language)
		if sym.Name != "" {
			// Resolve scope using container ranges (for TS/JS)
			if fileContainers, ok := containers[relPath]; ok && len(fileContainers) > 0 {
				sym.Scope = findContainingScope(sym.Line, fileContainers)
			}
			// For Go methods, extract receiver type as scope
			if fileMap[relPath].Language == "go" && strings.HasSuffix(m.RuleID, "-methods") {
				if receiver := extractGoReceiverType(m.Text); receiver != "" {
					sym.Scope = "struct:" + receiver
				}
			}
			fileMap[relPath].Symbols = append(fileMap[relPath].Symbols, sym)
		}
	}

	// Convert map to slice and dedupe
	var results []FileAnalysisV2
	for _, a := range fileMap {
		a.Symbols = dedupeSymbols(a.Symbols)
		results = append(results, *a)
	}

	return results, nil
}

// extractSymbolV2 creates a Symbol struct with full metadata from a match
func extractSymbolV2(m ScanMatch, lang string) Symbol {
	sym := Symbol{
		Line:   m.Range.Start.Line,
		Column: m.Range.Start.Column,
		Role:   determineRole(m.RuleID),
		Kind:   determineKind(m.RuleID),
		Scope:  "global", // Default scope
	}

	// Extract name based on rule type
	switch {
	case strings.HasSuffix(m.RuleID, "-imports"):
		if pathVar, ok := m.MetaVariables.Single["PATH"]; ok && pathVar.Text != "" {
			sym.Name = pathVar.Text
		} else {
			sym.Name = extractImportPath(m.Text)
		}
	case strings.HasSuffix(m.RuleID, "-arrow-functions"):
		sym.Name = extractArrowFunctionName(m.Lines, lang)
	case strings.HasSuffix(m.RuleID, "-function-signatures"):
		sym.Name = extractFunctionSignatureName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-generator-functions"):
		sym.Name = extractGeneratorFunctionName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-functions"):
		sym.Name = extractFunctionName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-structs"), strings.HasSuffix(m.RuleID, "-classes"):
		sym.Name = extractStructName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-interfaces"):
		sym.Name = extractInterfaceName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-methods"):
		sym.Name = extractMethodName(m.Text, lang)
		sym.Scope = extractScopeFromContext(m.Lines, lang)
	case strings.HasSuffix(m.RuleID, "-method-signatures"):
		sym.Name = extractMethodSignatureName(m.Text, lang)
		sym.Scope = extractScopeFromContext(m.Lines, lang)
	case strings.HasSuffix(m.RuleID, "-constants"):
		names := extractConstantNames(m.Text, lang)
		if len(names) > 0 {
			sym.Name = names[0] // First constant
		}
	case strings.HasSuffix(m.RuleID, "-types"):
		sym.Name = extractTypeName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-vars"):
		names := extractVarNames(m.Text, lang)
		if len(names) > 0 {
			sym.Name = names[0]
		}
	case strings.HasSuffix(m.RuleID, "-enums"):
		sym.Name = extractEnumName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-lexical"):
		text := strings.TrimSpace(m.Text)
		if strings.HasPrefix(text, "const ") {
			names := extractConstantNames(m.Text, lang)
			if len(names) > 0 {
				sym.Name = names[0]
				sym.Kind = KindConstant
			}
		} else {
			names := extractVarNames(m.Text, lang)
			if len(names) > 0 {
				sym.Name = names[0]
				sym.Kind = KindVariable
			}
		}
	case strings.HasSuffix(m.RuleID, "-namespaces"):
		sym.Name = extractNamespaceName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-property-signatures"):
		sym.Name = extractPropertySignatureName(m.Text, lang)
		sym.Scope = extractScopeFromContext(m.Lines, lang)
	case strings.HasSuffix(m.RuleID, "-field-definitions"):
		sym.Name = extractFieldDefinitionName(m.Text, lang)
		sym.Scope = extractScopeFromContext(m.Lines, lang)
	case strings.HasSuffix(m.RuleID, "-decorators"):
		sym.Name = extractDecoratorName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-import-aliases"):
		sym.Name = extractImportAliasName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-call-signatures"):
		sym.Name = extractCallSignature(m.Text)
	case strings.HasSuffix(m.RuleID, "-construct-signatures"):
		sym.Name = extractConstructSignature(m.Text)
	case strings.HasSuffix(m.RuleID, "-index-signatures"):
		sym.Name = extractIndexSignature(m.Text)
	case strings.HasSuffix(m.RuleID, "-variable-declarations"):
		names := extractVarDeclarationNames(m.Text, lang)
		if len(names) > 0 {
			sym.Name = names[0]
		}
	case strings.HasSuffix(m.RuleID, "-function-expressions"):
		sym.Name = extractFunctionExpressionName(m.Text, lang)
	case strings.HasSuffix(m.RuleID, "-static-blocks"):
		sym.Name = "(static block)"
	// Reference rules
	case strings.HasSuffix(m.RuleID, "-ref-function-calls"):
		sym.Name = extractCallExpressionName(m.Text)
	case strings.HasSuffix(m.RuleID, "-ref-new-expressions"):
		sym.Name = extractNewExpressionName(m.Text)
	case strings.HasSuffix(m.RuleID, "-ref-type-references"):
		sym.Name = extractTypeReferenceName(m.Text)
	}

	// Extract modifiers if present
	sym.Modifiers = extractModifiers(m.Text, lang)

	return sym
}

// determineRole returns whether a rule captures definitions or references
func determineRole(ruleID string) SymbolRole {
	if strings.Contains(ruleID, "-ref-") {
		return RoleReference
	}
	return RoleDefinition
}

// determineKind maps rule IDs to symbol kinds
func determineKind(ruleID string) SymbolKind {
	switch {
	case strings.HasSuffix(ruleID, "-imports"), strings.HasSuffix(ruleID, "-import-aliases"):
		return KindImport
	case strings.HasSuffix(ruleID, "-functions"), strings.HasSuffix(ruleID, "-arrow-functions"),
		strings.HasSuffix(ruleID, "-function-signatures"), strings.HasSuffix(ruleID, "-generator-functions"),
		strings.HasSuffix(ruleID, "-function-expressions"), strings.HasSuffix(ruleID, "-ref-function-calls"):
		return KindFunction
	case strings.HasSuffix(ruleID, "-methods"), strings.HasSuffix(ruleID, "-method-signatures"),
		strings.HasSuffix(ruleID, "-call-signatures"), strings.HasSuffix(ruleID, "-construct-signatures"),
		strings.HasSuffix(ruleID, "-static-blocks"):
		return KindMethod
	case strings.HasSuffix(ruleID, "-structs"), strings.HasSuffix(ruleID, "-classes"),
		strings.HasSuffix(ruleID, "-ref-new-expressions"):
		return KindClass
	case strings.HasSuffix(ruleID, "-interfaces"):
		return KindInterface
	case strings.HasSuffix(ruleID, "-types"), strings.HasSuffix(ruleID, "-ref-type-references"):
		return KindType
	case strings.HasSuffix(ruleID, "-enums"):
		return KindEnum
	case strings.HasSuffix(ruleID, "-namespaces"):
		return KindNamespace
	case strings.HasSuffix(ruleID, "-constants"):
		return KindConstant
	case strings.HasSuffix(ruleID, "-vars"), strings.HasSuffix(ruleID, "-variable-declarations"):
		return KindVariable
	case strings.HasSuffix(ruleID, "-lexical"):
		return KindVariable // Will be refined in extractSymbolV2
	case strings.HasSuffix(ruleID, "-field-definitions"):
		return KindField
	case strings.HasSuffix(ruleID, "-property-signatures"), strings.HasSuffix(ruleID, "-index-signatures"):
		return KindProperty
	case strings.HasSuffix(ruleID, "-decorators"):
		return KindDecorator
	}
	return KindVariable // Default
}

// extractScopeFromContext attempts to determine scope from surrounding code context
func extractScopeFromContext(lines string, lang string) string {
	if lang != "typescript" && lang != "javascript" {
		return "global"
	}

	// Look for class context in the lines
	if strings.Contains(lines, "class ") {
		// Try to extract class name
		idx := strings.Index(lines, "class ")
		if idx >= 0 {
			rest := lines[idx+6:]
			for i, c := range rest {
				if c == ' ' || c == '<' || c == '{' || c == '\n' {
					className := strings.TrimSpace(rest[:i])
					if isValidIdentifier(className) {
						return "class:" + className
					}
					break
				}
			}
		}
	}

	// Look for interface context
	if strings.Contains(lines, "interface ") {
		idx := strings.Index(lines, "interface ")
		if idx >= 0 {
			rest := lines[idx+10:]
			for i, c := range rest {
				if c == ' ' || c == '<' || c == '{' || c == '\n' {
					ifaceName := strings.TrimSpace(rest[:i])
					if isValidIdentifier(ifaceName) {
						return "interface:" + ifaceName
					}
					break
				}
			}
		}
	}

	return "global"
}

// extractModifiers extracts modifiers from code text
// Only looks at the first line to avoid picking up modifiers from nested content
func extractModifiers(text string, lang string) []string {
	var mods []string
	if lang != "typescript" && lang != "javascript" {
		return mods
	}

	// Only look at the first line to avoid matching modifiers in method bodies
	firstLine := text
	if newlineIdx := strings.Index(text, "\n"); newlineIdx > 0 {
		firstLine = text[:newlineIdx]
	}
	// Also limit to content before opening brace
	if braceIdx := strings.Index(firstLine, "{"); braceIdx > 0 {
		firstLine = firstLine[:braceIdx]
	}

	modifiers := []string{"public", "private", "protected", "static", "readonly", "async", "abstract", "override", "export", "default"}
	for _, mod := range modifiers {
		// Check if modifier appears as a word (followed by space)
		if strings.Contains(firstLine, mod+" ") || strings.HasPrefix(firstLine, mod+" ") {
			mods = append(mods, mod)
		}
	}
	return mods
}

// extractCallExpressionName extracts function name from a call expression
func extractCallExpressionName(text string) string {
	text = strings.TrimSpace(text)
	// Find the opening paren
	parenIdx := strings.Index(text, "(")
	if parenIdx < 0 {
		return ""
	}
	name := strings.TrimSpace(text[:parenIdx])
	// Handle member expressions: obj.method() -> method
	if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
		name = name[dotIdx+1:]
	}
	// Handle optional chaining: obj?.method() -> method
	if qIdx := strings.LastIndex(name, "?"); qIdx >= 0 {
		name = name[qIdx+1:]
	}
	if isValidIdentifier(name) {
		return name
	}
	return ""
}

// extractNewExpressionName extracts class name from a new expression
func extractNewExpressionName(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "new ") {
		return ""
	}
	text = strings.TrimPrefix(text, "new ")
	// Find end of class name (up to < or ()
	for i, c := range text {
		if c == '(' || c == '<' || c == ' ' {
			name := strings.TrimSpace(text[:i])
			if isValidIdentifier(name) {
				return name
			}
			return ""
		}
	}
	return ""
}

// extractTypeReferenceName extracts type name from a type reference
func extractTypeReferenceName(text string) string {
	text = strings.TrimSpace(text)
	// Handle generic types: Array<T> -> Array
	if bracketIdx := strings.Index(text, "<"); bracketIdx > 0 {
		text = text[:bracketIdx]
	}
	// Handle qualified names: Namespace.Type -> Type
	if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
		text = text[dotIdx+1:]
	}
	if isValidIdentifier(text) {
		return text
	}
	return ""
}

// extractGoReceiverType extracts the receiver type from a Go method declaration
// e.g., "func (m *MyStruct) Method()" -> "MyStruct"
func extractGoReceiverType(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "func ") {
		return ""
	}
	text = strings.TrimPrefix(text, "func ")
	// Find receiver: (receiver Type)
	if !strings.HasPrefix(text, "(") {
		return "" // Not a method, just a function
	}
	closeIdx := strings.Index(text, ")")
	if closeIdx < 0 {
		return ""
	}
	receiver := text[1:closeIdx]
	// receiver is like "m *MyStruct" or "m MyStruct"
	parts := strings.Fields(receiver)
	if len(parts) < 2 {
		return ""
	}
	typeName := parts[len(parts)-1]
	// Remove pointer prefix
	typeName = strings.TrimPrefix(typeName, "*")
	if isValidIdentifier(typeName) {
		return typeName
	}
	return ""
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
		// Strip decorators at the start (lines starting with @)
		for strings.HasPrefix(text, "@") {
			if newline := strings.Index(text, "\n"); newline > 0 {
				text = strings.TrimSpace(text[newline+1:])
			} else {
				break
			}
		}
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
		// Handle computed property names: [expr]() or ["literal"]()
		if strings.HasPrefix(text, "[") {
			if closeIdx := strings.Index(text, "]"); closeIdx > 0 {
				inner := text[1:closeIdx]
				// Remove quotes if present
				inner = strings.Trim(inner, "\"'`")
				if inner != "" {
					return "[" + inner + "]"
				}
			}
		}
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
			// Allow # prefix for JavaScript/TypeScript private fields
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '#') {
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

// fileCache provides thread-safe caching of file contents
type fileCache struct {
	content map[string][]byte
	mu      sync.RWMutex
}

func newFileCache() *fileCache {
	return &fileCache{content: make(map[string][]byte)}
}

func (c *fileCache) get(path string) ([]byte, error) {
	c.mu.RLock()
	if data, ok := c.content[path]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock
	if data, ok := c.content[path]; ok {
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c.content[path] = data
	return data, nil
}

// extractScopeContainers runs ast-grep to find all scope-creating containers
// and returns them grouped by file path
func (s *AstGrepScanner) extractScopeContainers(root string, cache *fileCache) (map[string][]ScopeContainer, error) {
	if !s.Available() {
		return nil, nil
	}

	// Build inline rules for containers only
	var containerRules []string
	entries, _ := os.ReadDir(s.rulesDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), "-containers") && strings.HasSuffix(e.Name(), ".yml") {
			content, err := os.ReadFile(filepath.Join(s.rulesDir, e.Name()))
			if err == nil {
				containerRules = append(containerRules, string(content))
			}
		}
	}

	if len(containerRules) == 0 {
		return make(map[string][]ScopeContainer), nil
	}

	inlineRules := strings.Join(containerRules, "\n---\n")
	cmd := exec.Command(s.binary, "scan", "--inline-rules", inlineRules, "--json", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) == 0 || !strings.HasPrefix(string(out), "[") {
			return make(map[string][]ScopeContainer), nil
		}
	}

	var matches []ScanMatch
	if err := json.Unmarshal(out, &matches); err != nil {
		return nil, err
	}

	// Group containers by file
	result := make(map[string][]ScopeContainer)
	for _, m := range matches {
		relPath, _ := filepath.Rel(root, m.File)
		if relPath == "" {
			relPath = m.File
		}

		container := parseContainerMatch(m, cache)
		if container.Name != "" {
			result[relPath] = append(result[relPath], container)
		}
	}

	return result, nil
}

// parseContainerMatch extracts a ScopeContainer from an ast-grep match
func parseContainerMatch(m ScanMatch, cache *fileCache) ScopeContainer {
	container := ScopeContainer{
		StartLine: m.Range.Start.Line, // 0-indexed, matching symbol line numbers
	}

	// Determine kind from rule ID
	switch {
	case strings.Contains(m.RuleID, "-container-class"):
		container.Kind = "class"
	case strings.Contains(m.RuleID, "-container-interface"):
		container.Kind = "interface"
	case strings.Contains(m.RuleID, "-container-namespace"):
		container.Kind = "namespace"
	case strings.Contains(m.RuleID, "-container-enum"):
		container.Kind = "enum"
	case strings.Contains(m.RuleID, "-container-struct"):
		container.Kind = "struct"
	default:
		return container
	}

	// Extract name from the match text
	container.Name = extractContainerName(m.Text, container.Kind)
	if container.Name == "" {
		return container
	}

	// Find end line by counting braces
	content, err := cache.get(m.File)
	if err != nil {
		// Fallback: estimate from match text
		container.EndLine = container.StartLine + strings.Count(m.Text, "\n")
		return container
	}

	container.EndLine = findEndLine(content, container.StartLine)
	return container
}

// extractContainerName extracts the name from container declaration text
func extractContainerName(text string, kind string) string {
	text = strings.TrimSpace(text)

	// Skip decorators at the start
	for strings.HasPrefix(text, "@") {
		if newline := strings.Index(text, "\n"); newline > 0 {
			text = strings.TrimSpace(text[newline+1:])
		} else {
			break
		}
	}

	// Strip common prefixes
	text = strings.TrimPrefix(text, "export ")
	text = strings.TrimPrefix(text, "default ")
	text = strings.TrimPrefix(text, "declare ")
	text = strings.TrimPrefix(text, "abstract ")
	text = strings.TrimPrefix(text, "const ") // for const enum

	// Handle Go struct/interface: type Name struct { or type Name interface {
	if kind == "struct" || (kind == "interface" && strings.HasPrefix(text, "type ")) {
		if strings.HasPrefix(text, "type ") {
			text = strings.TrimPrefix(text, "type ")
			for i, c := range text {
				if c == ' ' || c == '\t' {
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

	// Get the keyword and extract name (TypeScript/JavaScript)
	var keyword string
	switch kind {
	case "class":
		keyword = "class "
	case "interface":
		keyword = "interface "
	case "namespace":
		if strings.HasPrefix(text, "namespace ") {
			keyword = "namespace "
		} else if strings.HasPrefix(text, "module ") {
			keyword = "module "
		}
	case "enum":
		keyword = "enum "
	}

	if keyword == "" || !strings.HasPrefix(text, keyword) {
		return ""
	}

	text = strings.TrimPrefix(text, keyword)

	// Extract name (up to space, <, {, newline, or extends/implements)
	for i, c := range text {
		if c == ' ' || c == '<' || c == '{' || c == '\n' {
			name := strings.TrimSpace(text[:i])
			if isValidIdentifier(name) {
				return name
			}
			break
		}
	}

	// No delimiter found, return whole thing if valid
	for i, c := range text {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			name := text[:i]
			if isValidIdentifier(name) {
				return name
			}
			return ""
		}
	}
	if isValidIdentifier(text) {
		return text
	}
	return ""
}

// findEndLine finds the line number where a brace-delimited block ends
// startLine is 0-indexed, returns 0-indexed line number
func findEndLine(content []byte, startLine int) int {
	lines := bytes.Split(content, []byte("\n"))
	braceCount := 0
	started := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			if ch == '{' {
				braceCount++
				started = true
			} else if ch == '}' {
				braceCount--
				if started && braceCount == 0 {
					return i // Return 0-indexed line number
				}
			}
		}
	}
	return len(lines) - 1 // Fallback to last line (0-indexed)
}

// findContainingScope finds the innermost scope container that contains the given line
// Symbols on the container's start line (class/interface declaration) get "global" scope
func findContainingScope(symbolLine int, containers []ScopeContainer) string {
	var best *ScopeContainer
	for i := range containers {
		c := &containers[i]
		// Symbol must be strictly inside the container (not on start line)
		// This ensures class/interface declarations themselves get "global" scope
		if symbolLine > c.StartLine && symbolLine <= c.EndLine {
			if best == nil || c.StartLine > best.StartLine {
				best = c // Prefer innermost (most recent start line)
			}
		}
	}
	if best != nil {
		return best.Kind + ":" + best.Name
	}
	return "global"
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
