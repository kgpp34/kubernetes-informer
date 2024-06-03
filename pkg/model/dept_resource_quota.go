package model

type DeptResourceQuotaRequest struct {
	Dept               string `json:"dept"`
	RequestNonXcMemory string `json:"requestNonXcMemory"`
	RequestXcMemory    string `json:"requestXcMemory"`
}
