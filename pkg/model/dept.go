package model

type DeptResourceQuotaRequest struct {
	Dept               string `json:"dept"`
	RequestNonXcMemory string `json:"requestNonXcMemory"`
	RequestXcMemory    string `json:"requestXcMemory"`
}

type ResourceLimits struct {
	Memory string `json:"memory"`
}

type ResourceQuotas struct {
	Limits ResourceLimits `json:"limits"`
}

type SubResource struct {
	HG    struct{}       `json:"hg,omitempty"`
	Kylin ResourceQuotas `json:"kylin,omitempty"`
}

type Resources struct {
	NonXc ResourceQuotas `json:"nonXc"`
	XC    SubResource    `json:"xc,omitempty"`
}

type Announced struct {
	NonXc ResourceQuotas `json:"nonXc"`
	XC    SubResource    `json:"xc,omitempty"`
}

type UsedResource struct {
	NonXc struct {
		Memory string `json:"memory,omitempty"`
	} `json:"nonXc,omitempty"`
	XC struct {
		Kylin struct {
			Memory string `json:"memory,omitempty"`
		} `json:"kylin,omitempty"`
	} `json:"xc,omitempty"`
}

type DeptResource struct {
	Name      string       `json:"name"`
	Resources Resources    `json:"resources"`
	Announced Announced    `json:"announced,omitempty"`
	Used      UsedResource `json:"used,omitempty"`
}
