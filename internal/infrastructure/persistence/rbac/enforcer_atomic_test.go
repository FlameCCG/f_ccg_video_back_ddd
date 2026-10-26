package rbac

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/stretchr/testify/require"

	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
)

const (
	testV1AdminBanner = "/v1/admin/banner"
	testV1Common      = "/v1/common/**"
	testGET           = "GET"
)

func TestPolicyMutationHidesIntermediateState(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "policy.csv")
	{
		err := os.WriteFile(path, nil, 0o600)
		require.NoError(t, err)
	}

	enforcer, err := casbin.NewSyncedEnforcer(m, fileadapter.NewAdapter(path))
	require.NoError(t, err)

	adapter := NewCasbinEnforcer(enforcer)

	scoped, end := adapter.BeginPolicyMutation()
	if _, err := scoped.AddPolicy("subject", "/private", testGET); err != nil {
		end()
		t.Fatal(err)
	}

	if data, err := os.ReadFile(path); err != nil || len(data) != 0 { // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
		end()
		t.Fatalf("partial change reached storage: %q %v", data, err)
	}

	result := make(chan bool, 1)

	started := make(chan struct{})
	go func() { close(started); allow, _ := enforcer.Enforce("subject", "/private", testGET); result <- allow }()

	<-started

	select {
	case <-result:
		end()
		t.Fatal("authorization observed uncommitted mutation")
	case <-time.After(30 * time.Millisecond):
	}
	// A failed operation restores the old model before releasing authorization.
	scopedAdapter, ok := scoped.(*CasbinEnforcer)
	if !ok {
		end()
		t.Fatal("unexpected scoped enforcer type")
	}

	if err := scopedAdapter.RestorePolicy(nil, nil); err != nil {
		end()
		t.Fatal(err)
	}

	end()

	select {
	case allow := <-result:
		require.False(t, allow, "rolled-back grant remained effective")
	case <-time.After(time.Second):
		t.Fatal("authorization did not resume")
	}
}
