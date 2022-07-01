package machine

import (
	"reflect"
	"testing"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
)

func TestClusterTagFilter(t *testing.T) {
	var cases = []struct {
		clusterId   string
		machineName string
		expected    []ecs.DescribeInstancesTag
	}{
		{
			clusterId:   "testCluster",
			machineName: "test-machine",
			expected: []ecs.DescribeInstancesTag{
				{
					Key:   "kubernetes.io/cluster/testCluster",
					Value: "owned",
				},
				{
					Key:   "Name",
					Value: "test-machine",
				},
			},
		},
	}

	for i, c := range cases {
		t.Run(c.machineName, func(t *testing.T) {
			actual := clusterTagFilter(c.clusterId, c.machineName)
			if !reflect.DeepEqual(c.expected, actual) {
				t.Errorf("test #%d: expected %+v, got %+v", i, c.expected, actual)
			}
		})
	}
}
