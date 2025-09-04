/*
Copyright 2016 The Kubernetes Authors.

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

package garbagecollector

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

// cluster scoped resources don't have namespaces.  Default to the item's namespace, but clear it for cluster scoped resources
func resourceDefaultNamespace(namespaced bool, defaultNamespace string) string {
	if namespaced {
		return defaultNamespace
	}
	return ""
}

func (gc *GarbageCollector) chooseBuilderForUID(uid types.UID) *GraphBuilder {
	for _, gb := range gc.dependencyGraphBuilders {
		if _, ok := gb.uidToNode.Read(uid); ok {
			return gb
		}
	}
	return nil
}

func apiResourceUsing(mapper meta.RESTMapper, apiVersion, kind string) (schema.GroupVersionResource, bool, error) {
	fqKind := schema.FromAPIVersionAndKind(apiVersion, kind)
	mapping, err := mapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
	if err != nil {
		return schema.GroupVersionResource{}, false, newRESTMappingError(kind, apiVersion)
	}
	return mapping.Resource, mapping.Scope == meta.RESTScopeNamespace, nil
}

// apiResource consults the REST mapper to translate an <apiVersion, kind,
// namespace> tuple to a unversioned.APIResource struct.
func (gc *GarbageCollector) apiResource(apiVersion, kind string) (schema.GroupVersionResource, bool, error) {
	fqKind := schema.FromAPIVersionAndKind(apiVersion, kind)
	mapping, err := gc.restMapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
	if err != nil {
		return schema.GroupVersionResource{}, false, newRESTMappingError(kind, apiVersion)
	}
	return mapping.Resource, mapping.Scope == meta.RESTScopeNamespace, nil
}

func (gc *GarbageCollector) deleteObject(
	item objectReference,
	resourceVersion string,
	ownersAtResourceVersion []metav1.OwnerReference,
	policy *metav1.DeletionPropagation,
) error {
	gb := gc.chooseBuilderForUID(item.UID)
	if gb == nil {
		return fmt.Errorf("deleteObject: no graphBuilder for uid %s", item.UID)
	}

	resource, namespaced, err := apiResourceUsing(gb.restMapper, item.APIVersion, item.Kind)
	if err != nil {
		return err
	}

	uid := item.UID
	preconditions := metav1.Preconditions{UID: &uid}
	if len(resourceVersion) > 0 {
		preconditions.ResourceVersion = &resourceVersion
	}
	deleteOptions := metav1.DeleteOptions{Preconditions: &preconditions, PropagationPolicy: policy}

	ns := resourceDefaultNamespace(namespaced, item.Namespace)
	rc := gb.metadataClient.Resource(resource).Namespace(ns)

	err = rc.Delete(context.TODO(), item.Name, deleteOptions)
	if errors.IsConflict(err) && len(resourceVersion) > 0 {
		// Check if only RV changed (owners are same); if so, retry w/o RV precondition
		liveObject, liveErr := rc.Get(context.TODO(), item.Name, metav1.GetOptions{})
		if errors.IsNotFound(liveErr) {
			return nil
		}
		if liveErr == nil &&
			liveObject.UID == item.UID &&
			liveObject.ResourceVersion != resourceVersion &&
			reflect.DeepEqual(liveObject.OwnerReferences, ownersAtResourceVersion) {
			return gc.deleteObject(item, "", nil, policy)
		}
	}
	return err
}

func (gc *GarbageCollector) getObject(item objectReference) (*metav1.PartialObjectMetadata, error) {
	gb := gc.chooseBuilderForUID(item.UID)
	if gb == nil {
		return nil, fmt.Errorf("getObject: no graphBuilder for uid %s", item.UID)
	}

	resource, namespaced, err := apiResourceUsing(gb.restMapper, item.APIVersion, item.Kind)
	if err != nil {
		return nil, err
	}
	ns := resourceDefaultNamespace(namespaced, item.Namespace)
	if namespaced && len(ns) == 0 {
		// cluster-scoped child referencing a namespaced owner, invalid
		return nil, errNamespacedOwnerOfClusterScopedObject
	}

	return gb.metadataClient.Resource(resource).Namespace(ns).
		Get(context.TODO(), item.Name, metav1.GetOptions{})
}

func (gc *GarbageCollector) patchObject(item objectReference, patch []byte, pt types.PatchType) (*metav1.PartialObjectMetadata, error) {
	gb := gc.chooseBuilderForUID(item.UID)
	if gb == nil {
		return nil, fmt.Errorf("patchObject: no graphBuilder for uid %s", item.UID)
	}

	resource, namespaced, err := apiResourceUsing(gb.restMapper, item.APIVersion, item.Kind)
	if err != nil {
		return nil, err
	}
	ns := resourceDefaultNamespace(namespaced, item.Namespace)

	return gb.metadataClient.Resource(resource).Namespace(ns).
		Patch(context.TODO(), item.Name, pt, patch, metav1.PatchOptions{})
}

func (gc *GarbageCollector) removeFinalizer(logger klog.Logger, owner *node, targetFinalizer string) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ownerObject, err := gc.getObject(owner.identity)
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot finalize owner %s, because cannot get it: %v. The garbage collector will retry later", owner.identity, err)
		}
		accessor, err := meta.Accessor(ownerObject)
		if err != nil {
			return fmt.Errorf("cannot access the owner object %v: %v. The garbage collector will retry later", ownerObject, err)
		}
		finalizers := accessor.GetFinalizers()
		var newFinalizers []string
		found := false
		for _, f := range finalizers {
			if f == targetFinalizer {
				found = true
				continue
			}
			newFinalizers = append(newFinalizers, f)
		}
		if !found {
			logger.V(5).Info("finalizer already removed from object", "finalizer", targetFinalizer, "object", owner.identity)
			return nil
		}

		// remove the owner from dependent's OwnerReferences
		patch, err := json.Marshal(&objectForFinalizersPatch{
			ObjectMetaForFinalizersPatch: ObjectMetaForFinalizersPatch{
				ResourceVersion: accessor.GetResourceVersion(),
				Finalizers:      newFinalizers,
			},
		})
		if err != nil {
			return fmt.Errorf("unable to finalize %s due to an error serializing patch: %v", owner.identity, err)
		}
		_, err = gc.patchObject(owner.identity, patch, types.MergePatchType)
		return err
	})
	if errors.IsConflict(err) {
		return fmt.Errorf("updateMaxRetries(%d) has reached. The garbage collector will retry later for owner %v", retry.DefaultBackoff.Steps, owner.identity)
	}
	return err
}
