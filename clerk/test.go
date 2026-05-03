package clerk

import (
	"testing"

	integration "go.etcd.io/etcd/tests/v3/framework/integration"
)

func MakeTestClerk(t *testing.T, clusterSize int, opts ...integration.TestOption) (*Clerk, func()) {
	t.Helper()
	integration.BeforeTest(t, opts...)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: clusterSize})
	return &Clerk{clnt: clus.RandClient()}, func() { clus.Terminate(t) }
}
