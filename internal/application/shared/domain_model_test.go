package shared_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDomainRulesAreMethodsAndStateIsPrivate(t *testing.T) {
	models := map[string]string{
		"video":    "Video FavoriteFolder Danmu VideoReport DanmuReport",
		"identity": "User UserConf CoinAccount BanRecord",
		"comment":  "Comment", "dynamic": "Dynamic ReadRecord", "social": "Focus",
		"chat": "Message Conversation", "notification": "Notification", "contentops": "Banner", "rbac": "Role Menu",
	}
	root := filepath.Join("..", "..", "domain")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(filepath.ToSlash(rel), "shared/") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			value, _ := strconv.Unquote(imp.Path.Value)
			if file.Name.Name == "rbac" && value == "encoding/json" {
				t.Errorf("%s: keep resource storage codecs in infrastructure", rel)
			}
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
				switch fn.Name.Name {
				case "WithID", "SetID", "SetUserID", "SetStatus", "SetBalance", "AssignCurrentRole":
					t.Errorf("%s: %s bypasses business identity/state actions", rel, fn.Name.Name)
				}
			}
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				name := fn.Name.Name
				if !allowedDomainEntry(file.Name.Name, fn) {
					t.Errorf("%s: domain rule %s must have a receiver", rel, name)
				}
			}
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				typ, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				body, ok := typ.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if !strings.Contains(" "+models[file.Name.Name]+" ", " "+typ.Name.Name+" ") {
					continue
				}
				for _, field := range body.Fields.List {
					if len(field.Names) == 0 {
						t.Errorf("%s: %s must not expose state through embedded fields", rel, typ.Name.Name)
					}
					for _, name := range field.Names {
						if ast.IsExported(name.Name) {
							t.Errorf("%s: %s.%s exposes mutable domain state", rel, typ.Name.Name, name.Name)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplicationDoesNotRebuildPersistedDomainModels(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		domain := map[string]bool{}
		for _, imp := range file.Imports {
			value, _ := strconv.Unquote(imp.Path.Value)
			if !strings.HasPrefix(value, modulePath+"/internal/domain/") {
				continue
			}
			alias := filepath.Base(value)
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			domain[alias] = true
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && domain[pkg.Name] && strings.HasPrefix(sel.Sel.Name, "Restore") {
				t.Errorf("%s: use a domain action instead of %s.%s", path, pkg.Name, sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
