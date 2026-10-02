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
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	v1alpha1 "github.com/opendatahub-io/mcp-lifecycle-module-operator/api/v1alpha1"
)

var (
	mcpGatewayExtensionGVR = schema.GroupVersionResource{
		Group:    "mcp.kuadrant.io",
		Version:  "v1",
		Resource: "mcpgatewayextensions",
	}

	gatewayGVR = schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}
)

type mcpGatewayExtension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              struct {
		TargetRef struct {
			Name        string `json:"name"`
			Namespace   string `json:"namespace,omitempty"`
			SectionName string `json:"sectionName"`
		} `json:"targetRef"`
	} `json:"spec"`
	Status struct {
		Conditions []metav1.Condition `json:"conditions,omitempty"`
	} `json:"status"`
}

func (r *MCPLifecycleOperatorReconciler) mcpGatewayExtensionCRDAvailable() bool {
	list, err := r.DiscoveryClient.ServerResourcesForGroupVersion(
		mcpGatewayExtensionGVR.GroupVersion().String(),
	)
	if err != nil {
		return false
	}

	for _, res := range list.APIResources {
		if res.Name == mcpGatewayExtensionGVR.Resource {
			return true
		}
	}

	return false
}

func (r *MCPLifecycleOperatorReconciler) discoverMCPGateways(ctx context.Context) []v1alpha1.MCPGatewayInfo {
	log := logf.FromContext(ctx)

	if !r.mcpGatewayExtensionCRDAvailable() {
		return nil
	}

	list, err := r.DynamicClient.Resource(mcpGatewayExtensionGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Failed to list MCPGatewayExtensions")
		return nil
	}

	if len(list.Items) == 0 {
		return nil
	}

	var result []v1alpha1.MCPGatewayInfo

	for i := range list.Items {
		raw, err := list.Items[i].MarshalJSON()
		if err != nil {
			log.Error(err, "Failed to marshal MCPGatewayExtension", "name", list.Items[i].GetName())
			continue
		}

		var ext mcpGatewayExtension
		if err := json.Unmarshal(raw, &ext); err != nil {
			log.Error(err, "Failed to unmarshal MCPGatewayExtension", "name", list.Items[i].GetName())
			continue
		}

		gwNamespace := ext.Spec.TargetRef.Namespace
		if gwNamespace == "" {
			gwNamespace = ext.Namespace
		}

		info := v1alpha1.MCPGatewayInfo{
			Name:      ext.Name,
			Namespace: ext.Namespace,
			Ready:     isConditionTrue(ext.Status.Conditions, "Ready"),
			Gateway: v1alpha1.GatewayRef{
				Name:      ext.Spec.TargetRef.Name,
				Namespace: gwNamespace,
			},
		}

		info.Gateway.Listeners = r.resolveGatewayListeners(ctx, gwNamespace, ext.Spec.TargetRef.Name)

		result = append(result, info)
	}

	return result
}

func (r *MCPLifecycleOperatorReconciler) resolveGatewayListeners(ctx context.Context, namespace, name string) []v1alpha1.ListenerRef {
	log := logf.FromContext(ctx)

	obj, err := r.DynamicClient.Resource(gatewayGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.V(1).Info("Failed to get Gateway for MCPGatewayExtension", "gateway", name, "namespace", namespace, "error", err)
		return nil
	}

	raw, err := obj.MarshalJSON()
	if err != nil {
		log.Error(err, "Failed to marshal Gateway", "name", name)
		return nil
	}

	var gw gatewayv1.Gateway
	if err := json.Unmarshal(raw, &gw); err != nil {
		log.Error(err, "Failed to unmarshal Gateway", "name", name)
		return nil
	}

	listeners := make([]v1alpha1.ListenerRef, len(gw.Spec.Listeners))
	for i, l := range gw.Spec.Listeners {
		listeners[i] = v1alpha1.ListenerRef{Name: string(l.Name)}
	}

	return listeners
}

func isConditionTrue(conditions []metav1.Condition, condType string) bool {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Status == metav1.ConditionTrue
		}
	}

	return false
}

func (r *MCPLifecycleOperatorReconciler) tryRegisterGatewayWatches() {
	if r.controller == nil || r.DynamicInformerFactory == nil {
		return
	}

	if r.mcpGatewayExtensionCRDAvailable() {
		r.watchMCPGEOnce.Do(func() {
			informer := r.DynamicInformerFactory.ForResource(mcpGatewayExtensionGVR).Informer()
			r.DynamicInformerFactory.Start(make(chan struct{}))

			if err := r.controller.Watch(&source.Informer{
				Informer: informer,
				Handler:  r.enqueueComponentCR,
			}); err != nil {
				logf.Log.Error(err, "Failed to watch MCPGatewayExtension resources")
			}
		})

		r.watchGatewayOnce.Do(func() {
			informer := r.DynamicInformerFactory.ForResource(gatewayGVR).Informer()
			r.DynamicInformerFactory.Start(make(chan struct{}))

			if err := r.controller.Watch(&source.Informer{
				Informer: informer,
				Handler:  r.enqueueComponentCR,
			}); err != nil {
				logf.Log.Error(err, "Failed to watch Gateway resources")
			}
		})
	}
}
