package clerk

import (
	"testing"

	rpc "6.5840-final-project/rsm/rpc"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	integration "go.etcd.io/etcd/tests/v3/framework/integration"
)

// Required by etcd's integration framework — installs goroutine-leak detection.
func TestMain(m *testing.M) {
	testutil.MustTestMainWithLeakDetection(m)
}

func MakeTestClerk(t *testing.T, clusterSize int) (*Clerk, func()) {
	t.Helper()
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: clusterSize})
	ck := &Clerk{clnt: clus.RandClient()}
	return ck, func() { clus.Terminate(t) }
}

// Get on a missing key returns ErrNoKey with zero value/version.
func TestGet_MissingKey(t *testing.T) {
	ck, cleanup := MakeTestClerk(t, 3)
	defer cleanup()

	v, ver, err := ck.Get("nope")
	if err != rpc.ErrNoKey {
		t.Fatalf("err = %q, want ErrNoKey", err)
	}
	if v != "" || ver != 0 {
		t.Fatalf("got (%q, %d), want (\"\", 0)", v, ver)
	}
}

// First Put on a fresh key (version 0) creates it at version 1.
func TestPut_CreateThenGet(t *testing.T) {
	ck, cleanup := MakeTestClerk(t, 3)
	defer cleanup()

	if err := ck.Put("k", "v1", 0); err != rpc.OK {
		t.Fatalf("create put: %q", err)
	}
	v, ver, err := ck.Get("k")
	if err != rpc.OK {
		t.Fatalf("get: %q", err)
	}
	if v != "v1" || ver != 1 {
		t.Fatalf("got (%q, %d), want (\"v1\", 1)", v, ver)
	}
}

// Put with the matching version succeeds and bumps version by 1.
func TestPut_VersionMatchUpdates(t *testing.T) {
	ck, cleanup := MakeTestClerk(t, 3)
	defer cleanup()

	if err := ck.Put("k", "v1", 0); err != rpc.OK {
		t.Fatalf("create: %q", err)
	}
	if err := ck.Put("k", "v2", 1); err != rpc.OK {
		t.Fatalf("update: %q", err)
	}
	v, ver, _ := ck.Get("k")
	if v != "v2" || ver != 2 {
		t.Fatalf("got (%q, %d), want (\"v2\", 2)", v, ver)
	}
}

// Stale version on an existing key returns ErrVersion and leaves the value alone.
func TestPut_VersionMismatchOnExisting(t *testing.T) {
	ck, cleanup := MakeTestClerk(t, 3)
	defer cleanup()

	if err := ck.Put("k", "v1", 0); err != rpc.OK {
		t.Fatalf("create: %q", err)
	}
	if err := ck.Put("k", "v2", 99); err != rpc.ErrVersion {
		t.Fatalf("err = %q, want ErrVersion", err)
	}
	v, ver, _ := ck.Get("k")
	if v != "v1" || ver != 1 {
		t.Fatalf("got (%q, %d), want (\"v1\", 1) — value must be unchanged", v, ver)
	}
}

// Non-zero version on a missing key returns ErrNoKey (distinguishes
// "create" from "blind update" per MIT 6.5840 semantics).
func TestPut_NonZeroVersionOnMissingKey(t *testing.T) {
	ck, cleanup := MakeTestClerk(t, 3)
	defer cleanup()

	if err := ck.Put("ghost", "v", 1); err != rpc.ErrNoKey {
		t.Fatalf("err = %q, want ErrNoKey", err)
	}
	if _, _, err := ck.Get("ghost"); err != rpc.ErrNoKey {
		t.Fatalf("ghost should not exist, got err = %q", err)
	}
}

// Repeated successful Puts advance version by exactly 1 each.
func TestPut_VersionMonotonic(t *testing.T) {
	ck, cleanup := MakeTestClerk(t, 3)
	defer cleanup()

	if err := ck.Put("k", "0", 0); err != rpc.OK {
		t.Fatalf("seed: %q", err)
	}
	for i := rpc.Tversion(1); i < 5; i++ {
		if err := ck.Put("k", "x", i); err != rpc.OK {
			t.Fatalf("put at version %d: %q", i, err)
		}
		_, ver, _ := ck.Get("k")
		if ver != i+1 {
			t.Fatalf("after put@%d, got version %d, want %d", i, ver, i+1)
		}
	}
}
