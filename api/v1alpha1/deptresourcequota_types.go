/*
Copyright 2024.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeptResourceQuotaStatusReason string

const (
	ErrorComputingQuotaResource DeptResourceQuotaStatusReason = "ErrorComputingQuotaResource"
	ComputingQuotaResource      DeptResourceQuotaStatusReason = "Updated"
)

// WkResources defines the company department resource limits
type WkResources struct {
	XcResources    XcResources          `json:"xc,omitempty"`
	NonXcResources ComputationResources `json:"nonXc,omitempty"`
}

// XcResources defines the xc resources
type XcResources struct {
	HgResource    ComputationResources `json:"hg,omitempty"`
	KylinResource ComputationResources `json:"kylin,omitempty"`
}

// DeptResourceQuotaSpec defines the desired state of DeptResourceQuota
type DeptResourceQuotaSpec struct {
	// DeptName is the name of department
	DeptName string `json:"deptName"`
	// Resources demonstrate the resource limit for department
	Resources WkResources `json:"resources,omitempty"`
}

// ComputationResources record the limit and request resources
type ComputationResources struct {
	// Limits demonstrate limitation of different resource type
	Limits corev1.ResourceList `json:"limits,omitempty"`
	// Requests demonstrate requirement of different resource type
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

type UsedComputationResource struct {
	Limits   corev1.ResourceList `json:"limits,omitempty"`
	Requests corev1.ResourceList `json:"requests,omitempty"`
}
type UsedXcResource struct {
	HgResource    UsedComputationResource `json:"hg,omitempty"`
	KylinResource UsedComputationResource `json:"kylin,omitempty"`
}

type UsedResources struct {
	UsedXcResource    UsedXcResource          `json:"usedXc,omitempty"`
	UsedNonXcResource UsedComputationResource `json:"usedNonXc,omitempty"`
}

// DeptResourceQuotaStatus defines the observed state of DeptResourceQuota
type DeptResourceQuotaStatus struct {
	UsedResources UsedResources `json:"usedResources,omitempty"`
	// AvailableResource record the allocatable resource of current dept
	QuotaStatus corev1.ConditionStatus `json:"quotaStatus,omitempty"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DeptResourceQuota is the Schema for the deptresourcequotas API
type DeptResourceQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeptResourceQuotaSpec   `json:"spec,omitempty"`
	Status DeptResourceQuotaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeptResourceQuotaList contains a list of DeptResourceQuota
type DeptResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeptResourceQuota `json:"items"`
}
