package model

type ClusterResource struct {
	XcLimitsResources    XcLimitsResources `json:"xcLimitsResources,omitempty"`
	NonXcLimitsResources map[string]string `json:"nonXcLimitsResources,omitempty"`
}

type XcLimitsResources struct {
	X86 map[string]string `json:"x86,omitempty"`
	Arm map[string]string `json:"arm,omitempty"`
}
