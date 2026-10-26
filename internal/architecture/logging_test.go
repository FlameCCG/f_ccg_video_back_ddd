package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggingArchitecture(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate repository")

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "runtime", "uploads", "vendor":
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "wire_gen.go") {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		return inspectLoggingSource(t, path, relative)
	})
	require.NoError(t, err)
}

func inspectLoggingSource(t *testing.T, path, relative string) error {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return err
	}

	aliases := map[string]string{}

	for _, im := range parsed.Imports {
		pkg, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			return err
		}

		alias := filepath.Base(pkg)
		if im.Name != nil {
			alias = im.Name.Name
		}

		aliases[alias] = pkg
		checkLoggingImport(t, relative, pkg)
	}

	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		alias, ok := selector.X.(*ast.Ident)
		if ok {
			checkLoggingCall(t, relative, aliases[alias.Name], selector.Sel.Name)
		}

		return true
	})

	return nil
}

func checkLoggingImport(t *testing.T, relative, pkg string) {
	t.Helper()

	switch pkg {
	case "log", "log/slog", "github.com/sirupsen/logrus":
		t.Errorf("%s imports implicit logger %s", relative, pkg)
	}

	assert.False(t, pkg == "github.com/siddontang/go-log/log" && relative != "internal/infrastructure/legacylog/adapter.go", "legacy logger outside its adapter: %s", relative)

	if strings.HasPrefix(relative, "internal/domain/") {
		assert.False(t, forbiddenDomainImport(pkg), "domain depends on logging/transport: %s -> %s", relative, pkg)
	}
}

func forbiddenDomainImport(pkg string) bool {
	return pkg == "go.uber.org/zap" || pkg == "github.com/gin-gonic/gin" || strings.Contains(pkg, "/internal/infrastructure/") || strings.Contains(pkg, "/internal/interfaces/")
}

func checkLoggingCall(t *testing.T, relative, pkg, function string) {
	t.Helper()

	switch pkg {
	case "go.uber.org/zap":
		assert.False(t, function == "L" || function == "S" || function == "ReplaceGlobals", "global Zap in %s", relative)
	case "github.com/gin-gonic/gin":
		assert.NotEqual(t, "Default", function, "Gin default logging bypasses request policy in %s", relative)
	case "net/http/httputil":
		assert.False(t, function == "DumpRequest" || function == "DumpRequestOut", "request dump in %s", relative)
	}
}
