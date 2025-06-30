package collector

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildSnapshot(t *testing.T) {
	tcs := []struct {
		name string
		objs []runtime.Object
		want *v1.ClusterSnapshot
	}{
		{
			name: "empty",
			objs: []runtime.Object{},
			want: &v1.ClusterSnapshot{},
		},
		{
			name: "gpu node",
			objs: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node",
						Labels: map[string]string{
							"nvidia.com/gpu.memory":  "143771",
							"nvidia.com/gpu.product": "NVIDIA-H200",
						},
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							nvidiaGPU: *resource.NewQuantity(4, resource.DecimalSI),
						},
					},
				},
			},
			want: &v1.ClusterSnapshot{
				Nodes: []*v1.ClusterSnapshot_Node{
					{
						Name:           "gpu-node",
						GpuCapacity:    4,
						MemoryCapacity: 150754820096,
						NvidiaAttributes: &v1.ClusterSnapshot_Node_NvidiaAttributes{
							Product: "NVIDIA-H200",
						},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := newClusterSnapshotCollector(
				fake.NewFakeClient(tc.objs...),
				testr.New(t),
			)

			got, err := c.buildSnapshot(context.Background())
			assert.NoError(t, err)

			assert.Truef(t, proto.Equal(got, tc.want), cmp.Diff(got, tc.want, protocmp.Transform()))
		})
	}

}
