package auth

import (
	"os"
	"path/filepath"
	"testing"

	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNewEnforcerLoadsPersistedPolicy(t *testing.T) {
	model, err := os.ReadFile(filepath.Join("..", "..", "..", casbinModelPath))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("conf", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casbinModelPath, model, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "policies.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := gormadapter.NewAdapterByDB(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]gormadapter.CasbinRule{
		{Ptype: "p", V0: "viewer", V1: "/v1/admin/example", V2: "GET"},
		{Ptype: "g", V0: "user:1", V1: "viewer"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	enforcer, err := NewEnforcer(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		user string
		want bool
	}{{user: "user:1", want: true}, {user: "user:2", want: false}} {
		allowed, err := enforcer.Enforce(test.user, "/v1/admin/example", "GET")
		if err != nil || allowed != test.want {
			t.Fatalf("Enforce(%s) = (%v, %v), want %v", test.user, allowed, err, test.want)
		}
	}
}
