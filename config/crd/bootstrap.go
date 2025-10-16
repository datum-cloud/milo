package crd

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

//go:embed bases/**/*.yaml
var crdFS embed.FS

// installOrder defines dependency relationships between API groups.
// resourcemanager must be first because other groups reference Organizations/Projects.
var installOrder = []string{
	"resourcemanager.miloapis.com",
	"iam.miloapis.com",
	"quota.miloapis.com",
	"infrastructure.miloapis.com",
	"notification.miloapis.com",
}

// Bootstrap installs embedded CRDs into the Milo API server.
// Infrastructure CRDs are filtered out as they belong in the infrastructure cluster.
func Bootstrap(ctx context.Context, client apiextensionsclient.Interface) error {
	logger := klog.FromContext(ctx).WithName("crd-bootstrap")

	allCRDFiles, err := discoverCRDFiles()
	if err != nil {
		return fmt.Errorf("failed to discover embedded CRDs: %w", err)
	}

	crdFiles := filterInfrastructureCRDs(allCRDFiles)
	if len(crdFiles) == 0 {
		logger.Info("No CRDs found to bootstrap")
		return nil
	}

	logger.Info("Starting CRD bootstrap", "count", len(crdFiles), "total", len(allCRDFiles), "filtered", len(allCRDFiles)-len(crdFiles))

	sortedFiles := sortByInstallOrder(crdFiles)

	// Parallel installation improves startup time significantly.
	wg := sync.WaitGroup{}
	bootstrapErrChan := make(chan error, len(sortedFiles))

	for _, crdFile := range sortedFiles {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			err := retryRetryableErrors(func() error {
				return installCRD(ctx, client, filename)
			})
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			bootstrapErrChan <- err
		}(crdFile)
	}

	wg.Wait()
	close(bootstrapErrChan)

	// Collect all errors
	var bootstrapErrors []error
	for err := range bootstrapErrChan {
		if err != nil {
			bootstrapErrors = append(bootstrapErrors, err)
		}
	}

	if err := utilerrors.NewAggregate(bootstrapErrors); err != nil {
		return fmt.Errorf("failed to bootstrap CRDs: %w", err)
	}

	logger.Info("Successfully bootstrapped all CRDs", "count", len(sortedFiles))
	return nil
}

// retryRetryableErrors retries on connection refused, too many requests, and conflicts.
// This matches KCP's retry behavior for transient errors during API server initialization.
func retryRetryableErrors(f func() error) error {
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return utilnet.IsConnectionRefused(err) || apierrors.IsTooManyRequests(err) || apierrors.IsConflict(err)
	}, f)
}

func discoverCRDFiles() ([]string, error) {
	var files []string

	err := fs.WalkDir(crdFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".yaml" && !strings.Contains(path, "kustomization.yaml") {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// filterInfrastructureCRDs removes infrastructure API group CRDs from the list.
// Infrastructure CRDs (infrastructure.miloapis.com) should remain in the infrastructure cluster
// and not be installed into the Milo API server.
func filterInfrastructureCRDs(files []string) []string {
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if !strings.Contains(file, "/infrastructure/") {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func sortByInstallOrder(files []string) []string {
	priority := make(map[string]int)
	for i, group := range installOrder {
		priority[group] = i
	}

	sort.SliceStable(files, func(i, j int) bool {
		groupI := extractAPIGroup(files[i])
		groupJ := extractAPIGroup(files[j])

		priI, okI := priority[groupI]
		priJ, okJ := priority[groupJ]

		if okI && okJ {
			return priI < priJ
		}

		if okI {
			return true
		}
		if okJ {
			return false
		}

		return groupI < groupJ
	})

	return files
}

func extractAPIGroup(filename string) string {
	base := filepath.Base(filename)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// installCRD loads and installs a single CRD.
// This follows KCP's CreateSingle pattern with proper error handling and race condition support.
func installCRD(ctx context.Context, client apiextensionsclient.Interface, filename string) error {
	logger := klog.FromContext(ctx)

	data, err := crdFS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read embedded file %s: %w", filename, err)
	}

	rawCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(data, rawCRD); err != nil {
		return fmt.Errorf("failed to unmarshal CRD from %s: %w", filename, err)
	}

	start := time.Now()
	logger.V(2).Info("bootstrapping CRD", "name", rawCRD.Name)

	updateNeeded := false
	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, rawCRD.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			crd, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, rawCRD, metav1.CreateOptions{})
			if err != nil {
				// Multiple post-start hooks could race to create the same CRD
				if apierrors.IsAlreadyExists(err) {
					crd, err = client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, rawCRD.Name, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("error getting CRD %s: %w", rawCRD.Name, err)
					}
					updateNeeded = true
				} else {
					return fmt.Errorf("error creating CRD %s: %w", rawCRD.Name, err)
				}
			} else {
				logger.Info("bootstrapped CRD", "name", crd.Name, "duration", time.Since(start).String())
			}
		} else {
			return fmt.Errorf("error fetching CRD %s: %w", rawCRD.Name, err)
		}
	} else {
		updateNeeded = true
	}

	if updateNeeded {
		rawCRD.ResourceVersion = crd.ResourceVersion
		_, err := client.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, rawCRD, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating CRD %s: %w", rawCRD.Name, err)
		}
		logger.Info("updated CRD", "name", rawCRD.Name, "duration", time.Since(start).String())
	}

	// CRDs will become established asynchronously after the API server is fully ready.
	// We don't wait here because this runs in a post-start hook which blocks readiness.
	logger.V(2).Info("CRD created/updated, will become established after API server is ready", "name", rawCRD.Name)
	return nil
}

// ListEmbeddedCRDs returns all embedded CRD filenames (useful for debugging).
func ListEmbeddedCRDs() ([]string, error) {
	return discoverCRDFiles()
}
