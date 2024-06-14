package model

type NodeList struct {
	Items []Node `json:"items,omitempty"`
}

type Node struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Allocatable map[string]string `json:"allocatable,omitempty"`
	Used        map[string]string `json:"used,omitempty"`
}
