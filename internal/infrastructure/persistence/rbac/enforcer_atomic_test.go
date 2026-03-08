package rbac

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist/file-adapter"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPolicyMutationHidesIntermediateState(t *testing.T) {
	m, err := model.NewModelFromString(`[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub,p.sub) && r.obj == p.obj && r.act == p.act`)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "policy.csv")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	enforcer, err := casbin.NewSyncedEnforcer(m, fileadapter.NewAdapter(path))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewCasbinEnforcer(enforcer)
	scoped, end := adapter.BeginPolicyMutation()
	if _, err := scoped.AddPolicy("subject", "/private", "GET"); err != nil {
		end()
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || len(data) != 0 {
		end()
		t.Fatalf("partial change reached storage: %q %v", data, err)
	}
	result := make(chan bool, 1)
	started := make(chan struct{})
	go func() { close(started); allow, _ := enforcer.Enforce("subject", "/private", "GET"); result <- allow }()
	<-started
	select {
	case <-result:
		end()
		t.Fatal("authorization observed uncommitted mutation")
	case <-time.After(30 * time.Millisecond):
	}
	// A failed operation restores the old model before releasing authorization.
	if err := scoped.(*CasbinEnforcer).RestorePolicy(nil, nil); err != nil {
		end()
		t.Fatal(err)
	}
	end()
	select {
	case allow := <-result:
		if allow {
			t.Fatal("rolled-back grant remained effective")
		}
	case <-time.After(time.Second):
		t.Fatal("authorization did not resume")
	}
}
