package handler

import (
	"k8s-admin-informer/pkg/informer"
	"k8s-admin-informer/pkg/model"
	"k8s-admin-informer/pkg/util"
	"k8s.io/apimachinery/pkg/api/resource"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	PodInformer          *informer.PodInformer
	DeploymentInformer   *informer.DeploymentInformer
	StatefulSetInformer  *informer.StatefulSetInformer
	EventInformer        *informer.EventInformer
	ServiceInformer      *informer.ServiceInformer
	DeptResourceInformer *informer.DeptResourceQuotaInformer
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) ComputeDeptResourceQuotaLimit(c *gin.Context) {
	var req model.DeptResourceQuotaRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deptResourceQuota := h.DeptResourceInformer.GetDeptResourceQuotaByName(req.Dept)
	if deptResourceQuota == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DeptResourceQuota not found"})
		return
	}

	// 比较 Spec.Resources.Limits.Memory 与 Status.NonXcMemory
	if cmpResult := deptResourceQuota.Spec.Resources.NonXcResources.Limits.Memory().Cmp(deptResourceQuota.Status.UsedResources.UsedNonXcResource.Limits.Memory().DeepCopy()); cmpResult < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "Non-XC memory usage exceeds limit"})
		return
	}

	// 比较 Spec.XcResources.Limits.Memory 与 Status.XcMemory
	kylinMemory := deptResourceQuota.Status.UsedResources.UsedXcResource.KylinResource.Limits.Memory()
	if cmpResult := deptResourceQuota.Spec.Resources.XcResources.KylinResource.Limits.Memory().Cmp(kylinMemory.DeepCopy()); cmpResult < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "XC memory usage exceeds limit"})
		return
	}

	requestNonXcMem := resource.MustParse(req.RequestNonXcMemory)
	requestXcMem := resource.MustParse(req.RequestXcMemory)

	// 将请求中的 memory 和 xcMemory 分别加到 Status 中对应的字段
	newNonXcMemory := deptResourceQuota.Status.UsedResources.UsedNonXcResource.Limits.Memory().DeepCopy()
	newNonXcMemory.Add(requestNonXcMem)

	newXcMemory := deptResourceQuota.Status.UsedResources.UsedXcResource.KylinResource.Limits.Memory().DeepCopy()
	newXcMemory.Add(requestXcMem)

	// 比较 newNonXcMemory 与 Spec.Resources.Limits.Memory
	if cmpResult := deptResourceQuota.Spec.Resources.NonXcResources.Limits.Memory().Cmp(newNonXcMemory); cmpResult < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "After adding request, non-XC memory usage exceeds limit"})
		return
	}

	// 比较 newXcMemory 与 Spec.XcResources.Limits.Memory
	if cmpResult := deptResourceQuota.Spec.Resources.XcResources.KylinResource.Limits.Memory().Cmp(newXcMemory); cmpResult < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "After adding request, XC memory usage exceeds limit"})
		return
	}

	// 如果所有检查都通过
	c.JSON(http.StatusOK, gin.H{"success": true})
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
					asiaTime = event.CreationTimestamp.Time.Format(time.RFC3339)
				}
				instanceEvent := model.InstanceEvent{
					Message: event.Message,
					Reason:  event.Reason,
					Time:    asiaTime,
					Type:    event.Type,
				}
				instEvents = append(instEvents, instanceEvent)
			}

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
					Instances:   h.getPodAndEvents(app.Namespace, app.Name),
					Name:        app.Name,
					Namespace:   app.Namespace,
					Ready:       deployment.Status.ReadyReplicas,
					Total:       deployment.Status.Replicas,
					Services:    h.getServices(app.Namespace, app.Name),
					Labels:      deployment.Labels,
					Annotations: deployment.Annotations,
				}
				res = append(res, appInstance)
			}
		} else {
			statefulSets := h.StatefulSetInformer.GetStatefulSets(app.Namespace, app.Name)
			for _, statefulSet := range statefulSets {
				appInstance := model.AppInstance{
					Instances:   h.getPodAndEvents(app.Namespace, app.Name),
					Name:        app.Name,
					Namespace:   app.Namespace,
					Ready:       statefulSet.Status.ReadyReplicas,
					Total:       statefulSet.Status.Replicas,
					Services:    h.getServices(app.Namespace, app.Name),
					Labels:      statefulSet.Labels,
					Annotations: statefulSet.Annotations,
				}
				res = append(res, appInstance)
			}
		}
	}
	return res
}

func (h *Handler) getServices(ns string, name string) []model.Service {
	var res []model.Service

	services := h.ServiceInformer.GetServices(ns, name)

	for _, service := range services {
		if service != nil {
			modelSvc := model.Service{
				Namespace:   service.Namespace,
				Name:        service.Name,
				Annotations: service.Annotations,
			}
			res = append(res, modelSvc)
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
