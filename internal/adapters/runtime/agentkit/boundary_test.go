package agentkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKImportsStayInsideAdapter(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	adapterRoots := []string{
		filepath.ToSlash(filepath.Join("internal", "adapters", "runtime", "agentkit")) + "/",
	}
	einoPrefix := "github.com/cloudwego/" + "eino"
	localAdapterPrefix := "fkteams/internal/adapters/runtime/agentkit"
	localAdapterConsumers := []string{
		filepath.ToSlash(filepath.Join("internal", "bootstrap", "runtimes")) + "/",
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "release", "node_modules", "web":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		if !strings.HasSuffix(rel, "_test.go") {
			assertAgentKitOwnsExecution(t, rel, file)
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			if (strings.HasPrefix(importPath, einoPrefix) || strings.HasPrefix(importPath, "github.com/wsshow/agentkit")) && !isPathUnderAny(rel, adapterRoots) {
				t.Errorf("%s imports %s outside adapter packages", rel, importPath)
			}
			if strings.HasPrefix(importPath, localAdapterPrefix) &&
				!isPathUnderAny(rel, adapterRoots) &&
				!isPathUnderAny(rel, localAdapterConsumers) {
				t.Errorf("%s imports %s outside adapter packages", rel, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertAgentKitOwnsExecution(t *testing.T, path string, file *ast.File) {
	t.Helper()
	imports := make(map[string]string)
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		name := filepath.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = importPath
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		importPath := imports[pkg.Name]
		if importPath == "github.com/cloudwego/eino/adk" {
			switch selector.Sel.Name {
			case "NewRunner", "NewChatModelAgent", "NewLoopAgent", "NewAgentTool":
				t.Errorf("%s directly constructs %s; use AgentKit execution", path, selector.Sel.Name)
			}
		}
		if importPath == "github.com/cloudwego/eino/adk/prebuilt/deep" && selector.Sel.Name == "New" {
			t.Errorf("%s constructs a second Deep execution path", path)
		}
		return true
	})
}

func isPathUnderAny(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
