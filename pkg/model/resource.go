package model

import "k8s.io/apimachinery/pkg/api/resource"

type CommonResource struct {
	Limits ComputationResources `json:"limits,omitempty"`
}

type NonXcResource struct {
	CommonResource
}

type XcResource struct {
	Kylin CommonResource `json:"kylin,omitempty"`
	Hg    CommonResource `json:"hg,omitempty"`
}

type ComputationResources struct {
	Cpu    *resource.Quantity `json:"cpu,omitempty"`
	Memory *resource.Quantity `json:"memory,omitempty"`
}
