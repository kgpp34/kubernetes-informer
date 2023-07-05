package handler

import (
	"k8s-admin-informer/pkg/informer"
	"k8s-admin-informer/pkg/model"
	"k8s-admin-informer/pkg/util"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	PodInformer         *informer.PodInformer
	DeploymentInformer  *informer.DeploymentInformer
	StatefulSetInformer *informer.StatefulSetInformer
	EventInformer       *informer.EventInformer
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) GetWorkloadInstance(c *gin.Context) {
	var req model.GetWorkloadInstanceRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deployments := filterAppsByWorkloadType(req.Apps, "deployment")
	statefulSets := filterAppsByWorkloadType(req.Apps, "statefulset")

	var apps []model.AppInstance
	apps = append(apps, h.getAppInstance(deployments)...)
	apps = append(apps, h.getAppInstance(statefulSets)...)

	var response model.GetWorkloadInstanceResponse
	response.Apps = apps

	c.JSON(http.StatusOK, response)
}

func (h *Handler) getPodAndEvents(ns string, parentName string) []model.Instance {
	var instances []model.Instance

	pods, err := h.PodInformer.GetPodsByNsAndParent(ns, parentName)
	if err != nil {
		log.Errorf("查询pod异常: %v", err)
		return instances
	}
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		instance := model.Instance{Name: pod.Name}
		events := h.EventInformer.GetPodEvent(pod.Namespace, pod.Name)
		var instEvents []model.InstanceEvent

		if len(events) > 0 {
			for _, event := range events {
				asiaTime, err := util.ConvertUTCToAsiaShanghai(event.CreationTimestamp.Time)
				if err != nil {
					log.Errorf("解析时间出现错误:%v", err)
					asiaTime = event.CreationTimestamp.Time.String()
				}
				instanceEvent := model.InstanceEvent{
					Message: event.Message,
					Reason:  event.Reason,
					Time:    asiaTime,
				}
				instEvents = append(instEvents, instanceEvent)
			}
			sort.Sort(model.ByTime(instEvents))
		}
		instance.Events = instEvents
		instances = append(instances, instance)
	}
	return instances
}

func (h *Handler) getAppInstance(apps []model.App) []model.AppInstance {
	var res []model.AppInstance

	for _, app := range apps {
		if app.WorkloadType == "deployment" {
			deployments := h.DeploymentInformer.GetDeployments(app.Namespace, app.Name)
			for _, deployment := range deployments {
				appInstance := model.AppInstance{
					Instances: h.getPodAndEvents(app.Namespace, app.Name),
					Name:      app.Name,
					Namespace: app.Namespace,
					Ready:     deployment.Status.ReadyReplicas,
					Total:     deployment.Status.Replicas,
				}
				res = append(res, appInstance)
			}
		} else {
			statefulSets := h.StatefulSetInformer.GetStatefulSets(app.Namespace, app.Name)
			for _, statefulSet := range statefulSets {
				appInstance := model.AppInstance{
					Instances: h.getPodAndEvents(app.Namespace, app.Name),
					Name:      app.Name,
					Namespace: app.Namespace,
					Ready:     statefulSet.Status.ReadyReplicas,
					Total:     statefulSet.Status.Replicas,
				}
				res = append(res, appInstance)
			}
		}
	}
	return res
}

// filterAppsByWorkloadType
func filterAppsByWorkloadType(apps []model.App, workloadType string) []model.App {
	var filtered []model.App

	for _, app := range apps {
		if app.WorkloadType == workloadType {
			filtered = append(filtered, app)
		}
	}

	return filtered
}
