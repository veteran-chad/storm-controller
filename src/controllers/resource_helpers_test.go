package controllers

import (
	"testing"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

func TestGetStormImage(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *stormv1beta1.StormCluster
		expected string
	}{
		{
			name: "should use docker.io as default registry when no registry specified",
			cluster: &stormv1beta1.StormCluster{
				Spec: stormv1beta1.StormClusterSpec{
					Image: &stormv1beta1.ImageSpec{
						Repository: "apache/storm",
						Tag:        "2.8.1",
					},
				},
			},
			expected: "docker.io/apache/storm:2.8.1",
		},
		{
			name: "should use specified registry when provided",
			cluster: &stormv1beta1.StormCluster{
				Spec: stormv1beta1.StormClusterSpec{
					Image: &stormv1beta1.ImageSpec{
						Registry:   "myregistry.com",
						Repository: "apache/storm",
						Tag:        "2.8.1",
					},
				},
			},
			expected: "myregistry.com/apache/storm:2.8.1",
		},
		{
			name: "should not prepend docker.io when repository contains registry",
			cluster: &stormv1beta1.StormCluster{
				Spec: stormv1beta1.StormClusterSpec{
					Image: &stormv1beta1.ImageSpec{
						Repository: "hdscmnrtspsdevscuscr.azurecr.io/gp-upstream/docker.io/library/storm",
						Tag:        "2.8.1",
					},
				},
			},
			expected: "hdscmnrtspsdevscuscr.azurecr.io/gp-upstream/docker.io/library/storm:2.8.1",
		},
		{
			name: "should handle ACR registry with path",
			cluster: &stormv1beta1.StormCluster{
				Spec: stormv1beta1.StormClusterSpec{
					Image: &stormv1beta1.ImageSpec{
						Repository: "hdscmnrtspsdevscuscr.azurecr.io/rts/storm",
						Tag:        "2.8.1",
					},
				},
			},
			expected: "hdscmnrtspsdevscuscr.azurecr.io/rts/storm:2.8.1",
		},
		{
			name: "should handle short registry names",
			cluster: &stormv1beta1.StormCluster{
				Spec: stormv1beta1.StormClusterSpec{
					Image: &stormv1beta1.ImageSpec{
						Repository: "storm",
						Tag:        "2.8.1",
					},
				},
			},
			expected: "docker.io/storm:2.8.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getStormImage(tt.cluster)
			if actual != tt.expected {
				t.Errorf("getStormImage() = %v, want %v", actual, tt.expected)
			}
		})
	}
}
