package model

import "k8s.io/apimachinery/pkg/api/resource"

type CommonResource struct {
	Limits ComputationResources `json:"limits,omitempty"`
}

type NonXcResource struct {
	CommonResource
}

type XcResource struct {
	Arm CommonResource `json:"arm,omitempty"`
	X86 CommonResource `json:"x86,omitempty"`
}

type ComputationResources struct {
	Cpu    *resource.Quantity `json:"cpu,omitempty"`
	Memory *resource.Quantity `json:"memory,omitempty"`
}
