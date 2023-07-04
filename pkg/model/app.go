package model

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
