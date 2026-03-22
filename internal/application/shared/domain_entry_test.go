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

// 明确登记已审查的构造/重建入口。新增 NewX 不能仅凭前缀绕过方法约束。
func allowedDomainEntry(context string, fn *ast.FuncDecl) bool {
	entries := map[string]string{
		"ai":           "NewPromptVideoCandidate",
		"chat":         "NewMessage RestoreMessage RestoreConversation",
		"comment":      "NewComment RestoreComment",
		"contentops":   "NewBanner RestoreBanner",
		"dynamic":      "NewDynamic RestoreDynamic NewReadRecord RestoreReadRecord",
		"identity":     "NewBanRecord RestoreBanRecord RestoreCoinAccount NewCoinChange RestoreUser NewEmailUser NewOAuthUser NewSystemUser NewUserConf RestoreUserConf",
		"media":        "NewQueuedProgress NewMP4Target",
		"notification": "NewNotification RestoreNotification",
		"rbac":         "NewMenu RestoreMenu NewRole RestoreRole NewInheritedRoleMenus NewDirectRoleMenus NewPermissionEvaluator",
		"social":       "NewFocus RestoreFocus",
		"video":        "NewVideo RestoreVideo NewDanmu RestoreDanmu NewFavoriteFolder NewDefaultFavoriteFolder RestoreFavoriteFolder NewVideoReport RestoreVideoReport NewDanmuReport RestoreDanmuReport NewHotRankDecayPolicy NewRelatedSelection",
	}
	errors := map[string]string{
		"rbac":     "DuplicateRoleAssignedError AlreadyInheritedError IndirectUnlinkError",
		"identity": "IsCoinInsufficient", "video": "IsSourceFileNameError",
	}
	name := fn.Name.Name
	if strings.Contains(" "+errors[context]+" ", " "+name+" ") {
		return true
	}
	if !strings.Contains(" "+entries[context]+" ", " "+name+" ") {
		return false
	}
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false
	}
	var result ast.Expr = fn.Type.Results.List[0].Type
	for {
		switch typ := result.(type) {
		case *ast.StarExpr:
			result = typ.X
		case *ast.ArrayType:
			result = typ.Elt
		case *ast.Ident:
			return ast.IsExported(typ.Name)
		default:
			return false
		}
	}
}

func TestDomainEntryGuardRejectsDisguisedCalculations(t *testing.T) {
	for _, tc := range []struct {
		source  string
		allowed bool
	}{
		{"func NewScore(a int) int { return a*2 }", false},
		{"func RestoreScore(a int) int { return a*2 }", false},
		{"func NewVideo(a int) int { return a*2 }", false},
		{"func NewVideo() *Video { return &Video{} }", true},
		{"func RestoreVideo() *Video { return &Video{} }", true},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package video\n"+tc.source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := allowedDomainEntry("video", file.Decls[0].(*ast.FuncDecl)); got != tc.allowed {
			t.Errorf("%s: allowed=%v", tc.source, got)
		}
	}
}

func TestDomainCreateRepositoriesReturnPersistenceResults(t *testing.T) {
	err := filepath.WalkDir("../../domain", func(path string, entry fs.DirEntry, err error) error {
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
		ast.Inspect(file, func(node ast.Node) bool {
			body, ok := node.(*ast.InterfaceType)
			if !ok {
				return true
			}
			for _, method := range body.Methods.List {
				if len(method.Names) != 1 || method.Names[0].Name != "Create" {
					continue
				}
				fn, ok := method.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				entityInput := false
				for _, field := range fn.Params.List {
					if ptr, ok := field.Type.(*ast.StarExpr); ok {
						if model, ok := ptr.X.(*ast.Ident); ok {
							entityInput = ast.IsExported(model.Name)
						}
						if entityInput {
							break
						}
					}
				}
				if entityInput && fn.Results != nil && len(fn.Results.List) == 1 {
					if result, ok := fn.Results.List[0].Type.(*ast.Ident); ok && result.Name == "error" {
						t.Errorf("%s: Create must return persisted identity instead of mutating its input", path)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplicationDoesNotBuildRedisKeys(t *testing.T) {
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
		aliases := map[string]bool{}
		for _, imp := range file.Imports {
			value, _ := strconv.Unquote(imp.Path.Value)
			if value != modulePath+"/pkg/constants" {
				continue
			}
			alias := "constants"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if alias == "." {
				t.Errorf("%s: constants dot import hides storage keys", path)
			}
			aliases[alias] = true
		}
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && aliases[pkg.Name] && strings.HasSuffix(sel.Sel.Name, "KeyPrefix") {
				t.Errorf("%s: %s belongs in a cache adapter", path, sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplicationDoesNotNormalizeCoinAmounts(t *testing.T) {
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
		aliases := map[string]bool{}
		for _, imp := range file.Imports {
			value, _ := strconv.Unquote(imp.Path.Value)
			if value != modulePath+"/pkg/coinutil" {
				continue
			}
			alias := "coinutil"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if alias == "." {
				t.Errorf("%s: coinutil dot import hides domain amount rules", path)
			}
			aliases[alias] = true
		}
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && aliases[pkg.Name] && sel.Sel.Name == "Round" {
				t.Errorf("%s: coin amount rounding belongs in the owning domain", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
