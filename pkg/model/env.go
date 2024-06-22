package model

type EnvResource struct {
	Dept          string        `json:"dept,omitempty"`
	EnvName       string        `json:"envName,omitempty"`
	NonXcResource NonXcResource `json:"nonXcResource,omitempty"`
	XcResource    XcResource    `json:"xcResource,omitempty"`
}
