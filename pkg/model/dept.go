package model

type DeptResourceQuotaRequest struct {
	Dept                  string `json:"dept"`
	RequestNonXcMemory    string `json:"requestNonXcMemory"`
	RequestKylinArmMemory string `json:"requestKylinArmMemory"`
	RequestKylinHgMemory  string `json:"requestKylinHgMemory"`
}

type ResourceLimits struct {
	Memory string `json:"memory"`
}

type ResourceQuotas struct {
	Limits ResourceLimits `json:"limits"`
}

type SubResource struct {
	X86 ResourceQuotas `json:"x86,omitempty"`
	Arm ResourceQuotas `json:"arm,omitempty"`
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
		Arm struct {
			Memory string `json:"memory,omitempty"`
		} `json:"arm,omitempty"`
		X86 struct {
			Memory string `json:"memory,omitempty"`
		} `json:"x86,omitempty"`
	} `json:"xc,omitempty"`
}

type DeptResource struct {
	Name      string       `json:"name"`
	Resources Resources    `json:"resources"`
	Announced Announced    `json:"announced,omitempty"`
	Used      UsedResource `json:"used,omitempty"`
	Pods      int          `json:"pods,omitempty"`
}
