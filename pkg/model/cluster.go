package model

type ClusterResource struct {
	XcLimitsResources    XcLimitsResources `json:"xcLimitsResources,omitempty"`
	NonXcLimitsResources map[string]string `json:"nonXcLimitsResources,omitempty"`
}

type XcLimitsResources struct {
	Hg    map[string]string `json:"hg,omitempty"`
	Kylin map[string]string `json:"kylin,omitempty"`
}
