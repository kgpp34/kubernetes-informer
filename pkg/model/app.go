package model

import "time"

type App struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	WorkloadType string `json:"workloadType"`
}

type GetWorkloadInstanceRequest struct {
	Apps []App `json:"apps"`
}

type InstanceEvent struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Time    string `json:"time"`
	Type    string `json:"type"`
}

type Instance struct {
	Name   string          `json:"name"`
	Events []InstanceEvent `json:"events"`
}

type AppInstance struct {
	Namespace string     `json:"namespace"`
	Name      string     `json:"name"`
	Total     int32      `json:"total"`
	Ready     int32      `json:"ready"`
	Instances []Instance `json:"instances"`
}

type GetWorkloadInstanceResponse struct {
	Apps []AppInstance `json:"apps"`
}

type ByTime []InstanceEvent

func (s ByTime) Len() int      { return len(s) }
func (s ByTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByTime) Less(i, j int) bool {
	time1, _ := time.Parse(time.RFC3339, s[i].Time)
	time2, _ := time.Parse(time.RFC3339, s[j].Time)
	return time1.Before(time2)
}
