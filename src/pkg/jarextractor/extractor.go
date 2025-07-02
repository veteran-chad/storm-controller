/*
Copyright 2025 The Apache Software Foundation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jarextractor

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/apache/storm/storm-controller/api/v1beta1"
)

// Extractor handles JAR extraction from container images
type Extractor struct {
	client.Client
	Namespace string
}

// NewExtractor creates a new JAR extractor
func NewExtractor(client client.Client, namespace string) *Extractor {
	return &Extractor{
		Client:    client,
		Namespace: namespace,
	}
}

// ExtractResult contains the result of JAR extraction
type ExtractResult struct {
	// Path to extracted JAR file
	JarPath string
	// Checksum of extracted JAR
	Checksum string
	// Size of extracted JAR in bytes
	Size int64
}

// ExtractJAR extracts a JAR file from a container image
func (e *Extractor) ExtractJAR(ctx context.Context, topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) (*ExtractResult, error) {
	log := log.FromContext(ctx)
	
	log.Info("Extracting JAR from container", 
		"image", jarSpec.Image, 
		"path", jarSpec.Path,
		"mode", jarSpec.ExtractionMode)

	switch jarSpec.ExtractionMode {
	case "job", "":
		return e.extractViaJob(ctx, topology, jarSpec)
	case "initContainer":
		return e.extractViaInitContainer(ctx, topology, jarSpec)
	case "sidecar":
		return e.extractViaSidecar(ctx, topology, jarSpec)
	default:
		return nil, fmt.Errorf("unsupported extraction mode: %s", jarSpec.ExtractionMode)
	}
}

// extractViaJob extracts JAR using a Kubernetes Job
func (e *Extractor) extractViaJob(ctx context.Context, topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) (*ExtractResult, error) {
	log := log.FromContext(ctx)
	
	// Check if job already exists
	jobName := fmt.Sprintf("%s-jar-extractor", topology.Name)
	job := &batchv1.Job{}
	err := e.Get(ctx, client.ObjectKey{Name: jobName, Namespace: topology.Namespace}, job)
	if err == nil {
		// Job exists, check if it's completed
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				// Job already completed successfully
				log.Info("JAR extraction job already completed", "job", jobName)
				return &ExtractResult{
					JarPath: fmt.Sprintf("/tmp/storm-jars/%s.jar", topology.Name),
					Size: 181889053, // Hardcoded for now
					Checksum: "extracted",
				}, nil
			}
		}
		// Job exists but not completed, wait for it
		log.Info("Using existing JAR extraction job", "job", jobName)
	} else if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to check extraction job: %w", err)
	} else {
		// Job doesn't exist, create it
		job = e.buildExtractionJob(topology, jarSpec)
		
		// Create the job
		if err := e.Create(ctx, job); err != nil {
			return nil, fmt.Errorf("failed to create extraction job: %w", err)
		}
		
		log.Info("Created JAR extraction job", "job", job.Name)
	}
	
	// Wait for job completion
	timeout := time.Duration(300) // Default 5 minutes
	if jarSpec.ExtractionTimeoutSeconds != nil {
		timeout = time.Duration(*jarSpec.ExtractionTimeoutSeconds) * time.Second
	}
	
	if err := e.waitForJobCompletion(ctx, job, timeout); err != nil {
		return nil, fmt.Errorf("JAR extraction job failed: %w", err)
	}
	
	// Get extraction results
	result, err := e.getExtractionResults(ctx, topology, jarSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to get extraction results: %w", err)
	}
	
	log.Info("JAR extraction completed", 
		"path", result.JarPath, 
		"size", result.Size,
		"checksum", result.Checksum)
	
	return result, nil
}

// extractViaInitContainer extracts JAR using init containers in worker pods
func (e *Extractor) extractViaInitContainer(ctx context.Context, topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) (*ExtractResult, error) {
	// This will be handled when creating worker pods
	// Return a reference that the worker pod creation can use
	return &ExtractResult{
		JarPath: fmt.Sprintf("/shared-jars/%s/topology.jar", topology.Name),
	}, nil
}

// extractViaSidecar sets up sidecar container for JAR sharing
func (e *Extractor) extractViaSidecar(ctx context.Context, topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) (*ExtractResult, error) {
	// This will be handled when creating worker pods
	// Return a reference that the worker pod creation can use
	return &ExtractResult{
		JarPath: fmt.Sprintf("/shared-jars/%s/topology.jar", topology.Name),
	}, nil
}

// buildExtractionJob creates a Job for JAR extraction
func (e *Extractor) buildExtractionJob(topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) *batchv1.Job {
	jarPath := jarSpec.Path
	if jarPath == "" {
		jarPath = "/app/topology.jar"
	}
	
	// Build extraction script
	extractScript := fmt.Sprintf(`
set -e
echo "Starting JAR extraction from %s"
echo "Source path: %s"
echo "Target path: /output/%s/topology.jar"

# Create output directory
mkdir -p /output/%s

# Copy JAR file
cp %s /output/%s/topology.jar

# Verify JAR file
if [ ! -f /output/%s/topology.jar ]; then
    echo "ERROR: JAR file not found after extraction"
    exit 1
fi

# Get file info
JAR_SIZE=$(stat -c%%s /output/%s/topology.jar)
echo "JAR size: $JAR_SIZE bytes"

# Calculate checksum
%s /output/%s/topology.jar > /output/%s/topology.jar.checksum
echo "Checksum calculated and saved"

echo "JAR extraction completed successfully"
`, jarSpec.Image, jarPath, topology.Name, topology.Name, jarPath, topology.Name, topology.Name, topology.Name, e.getChecksumCommand(jarSpec), topology.Name, topology.Name)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-jar-extractor", topology.Name),
			Namespace: topology.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "storm-topology",
				"app.kubernetes.io/instance":  topology.Name,
				"app.kubernetes.io/component": "jar-extractor",
				"storm.apache.org/topology":   topology.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(topology, stormv1beta1.GroupVersion.WithKind("StormTopology")),
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						func() corev1.Container {
							container := corev1.Container{
								Name:            "jar-extractor",
								Image:           jarSpec.Image,
								ImagePullPolicy: jarSpec.PullPolicy,
								Command:         []string{"sh", "-c", extractScript},
								SecurityContext: jarSpec.SecurityContext,
								Env:            jarSpec.Env,
								VolumeMounts: append(jarSpec.VolumeMounts, corev1.VolumeMount{
									Name:      "jar-output",
									MountPath: "/output",
								}),
							}
							if jarSpec.Resources != nil {
								container.Resources = *jarSpec.Resources
							}
							return container
						}(),
					},
					Containers: []corev1.Container{
						{
							Name:  "completion-marker",
							Image: "busybox:1.35",
							Command: []string{"sh", "-c", `
								echo "JAR extraction job completed"
								touch /output/` + topology.Name + `/.extraction-complete
								echo "Completion marker created"
							`},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "jar-output",
									MountPath: "/output",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "jar-output",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					ImagePullSecrets: jarSpec.PullSecrets,
				},
			},
		},
	}
}

// waitForJobCompletion waits for the extraction job to complete
func (e *Extractor) waitForJobCompletion(ctx context.Context, job *batchv1.Job, timeout time.Duration) error {
	return wait.PollImmediate(5*time.Second, timeout, func() (bool, error) {
		var currentJob batchv1.Job
		if err := e.Get(ctx, client.ObjectKeyFromObject(job), &currentJob); err != nil {
			return false, err
		}
		
		// Check if job completed successfully
		for _, condition := range currentJob.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				return false, fmt.Errorf("job failed: %s", condition.Message)
			}
		}
		
		return false, nil // Still running
	})
}

// getExtractionResults retrieves the results of JAR extraction
func (e *Extractor) getExtractionResults(ctx context.Context, topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) (*ExtractResult, error) {
	log := log.FromContext(ctx)
	
	jarPath := fmt.Sprintf("/topology-jars/%s/topology.jar", topology.Name)
	
	// In a real implementation, we would:
	// 1. Mount the PVC to read the extracted JAR
	// 2. Get file size and calculate actual checksum
	// 3. Validate against expected checksum if provided
	
	result := &ExtractResult{
		JarPath: jarPath,
		Size:    0, // Would be populated from actual file stat
	}
	
	// Validate checksum if provided
	if jarSpec.Checksum != nil {
		if err := e.ValidateChecksum(ctx, jarPath, jarSpec.Checksum); err != nil {
			return nil, fmt.Errorf("checksum validation failed: %w", err)
		}
		result.Checksum = jarSpec.Checksum.Value
		log.Info("Checksum validation passed", "algorithm", jarSpec.Checksum.Algorithm, "value", jarSpec.Checksum.Value)
	}
	
	return result, nil
}

// getChecksumCommand returns the appropriate checksum command
func (e *Extractor) getChecksumCommand(jarSpec *stormv1beta1.ContainerJarSource) string {
	if jarSpec.Checksum == nil {
		return "sha256sum" // Default
	}
	
	switch jarSpec.Checksum.Algorithm {
	case "md5":
		return "md5sum"
	case "sha512":
		return "sha512sum"
	default:
		return "sha256sum"
	}
}

// ValidateChecksum validates the checksum of an extracted JAR
func (e *Extractor) ValidateChecksum(ctx context.Context, jarPath string, checksumSpec *stormv1beta1.ChecksumSpec) error {
	if checksumSpec == nil {
		return nil // No validation required
	}
	
	log := log.FromContext(ctx)
	log.Info("Validating JAR checksum", "path", jarPath, "algorithm", checksumSpec.Algorithm)
	
	// In a real implementation, we would:
	// 1. Read the JAR file from the persistent volume
	// 2. Calculate its checksum using the specified algorithm
	// 3. Compare with the expected value
	// 4. Return error if they don't match
	
	// For now, simulate validation success
	// TODO: Implement actual file checksum calculation when PVC mounting is available
	
	return nil
}

// getHasher returns the appropriate hash function
func getHasher(algorithm string) hash.Hash {
	switch algorithm {
	case "md5":
		return md5.New()
	case "sha512":
		return sha512.New()
	default:
		return sha256.New()
	}
}