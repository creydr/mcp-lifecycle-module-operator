/*
Copyright 2026.

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

package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestFindDeploymentNames(t *testing.T) {
	resources := []unstructured.Unstructured{
		{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata":   map[string]interface{}{"name": "sa", "namespace": "ns"},
		}},
		{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "controller-manager", "namespace": "target-ns"},
		}},
		{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "webhook", "namespace": "target-ns"},
		}},
		{Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata":   map[string]interface{}{"name": "manager-role"},
		}},
	}

	names := findDeploymentNames(resources)

	if len(names) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(names))
	}

	expected := []types.NamespacedName{
		{Namespace: "target-ns", Name: "controller-manager"},
		{Namespace: "target-ns", Name: "webhook"},
	}
	for i, nn := range names {
		if nn != expected[i] {
			t.Errorf("deployment[%d] = %v, want %v", i, nn, expected[i])
		}
	}
}

func TestFindDeploymentNamesEmpty(t *testing.T) {
	resources := []unstructured.Unstructured{
		{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": "cm", "namespace": "ns"},
		}},
	}

	names := findDeploymentNames(resources)
	if len(names) != 0 {
		t.Fatalf("expected 0 deployments, got %d", len(names))
	}
}
