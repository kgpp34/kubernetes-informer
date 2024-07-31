package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"k8s-admin-informer/pkg/kubernetes/informer"
	"k8s-admin-informer/pkg/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"net/http"
	"strings"
)

type NodeType string

const (
	XcNode    NodeType = "xc"
	NonXcNode NodeType = "nonXc"
)

type ResourceHandler struct {
	Handler *Handler
}

func NewResourceHandler(handler *Handler) *ResourceHandler {
	return &ResourceHandler{
		Handler: handler,
	}
}

func (h *ResourceHandler) ComputeDeptResourceQuotaLimit(c *gin.Context) {
	var req model.DeptResourceQuotaRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deptResourceQuota := h.Handler.Informers[DeptResourceQuotaInformer].(*informer.DeptResourceQuotaInformer).GetDeptResourceQuotaByName(req.Dept)
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
	kylinArmMemory := deptResourceQuota.Status.UsedResources.UsedXcResource.ArmResource.Limits.Memory()
	if cmpResult := deptResourceQuota.Spec.Resources.XcResources.ArmResource.Limits.Memory().Cmp(kylinArmMemory.DeepCopy()); cmpResult < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "XC memory usage exceeds limit"})
		return
	}

	requestNonXcMem := resource.MustParse(req.RequestNonXcMemory)
	requestKylinArmMem := resource.MustParse(req.RequestKylinArmMemory)
	requestKylinX86Mem := resource.MustParse(req.RequestKylinHgMemory)

	// 将请求中的 memory 和 xcMemory 分别加到 Status 中对应的字段
	newNonXcMemory := deptResourceQuota.Status.UsedResources.UsedNonXcResource.Limits.Memory().DeepCopy()
	newNonXcMemory.Add(requestNonXcMem)

	newKylinArmMemory := deptResourceQuota.Status.UsedResources.UsedXcResource.ArmResource.Limits.Memory().DeepCopy()
	newKylinArmMemory.Add(requestKylinArmMem)

	newKylinX86Memory := deptResourceQuota.Status.UsedResources.UsedXcResource.HgResource.Limits.Memory().DeepCopy()
	newKylinX86Memory.Add(requestKylinX86Mem)

	// 比较 newNonXcMemory 与 Spec.Resources.Limits.Memory
	if !requestNonXcMem.IsZero() {
		if !deptResourceQuota.Spec.Resources.NonXcResources.Limits.Memory().IsZero() || !newNonXcMemory.IsZero() {
			if cmpResult := deptResourceQuota.Spec.Resources.NonXcResources.Limits.Memory().Cmp(newNonXcMemory); cmpResult <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "After adding request, non-XC memory usage exceeds limit"})
				return
			}
		}
	}

	if !requestKylinArmMem.IsZero() {
		// 比较 newXcMemory 与 Spec.XcResources.Limits.Memory
		if !deptResourceQuota.Spec.Resources.XcResources.ArmResource.Limits.Memory().IsZero() || !newKylinArmMemory.IsZero() {
			if cmpResult := deptResourceQuota.Spec.Resources.XcResources.ArmResource.Limits.Memory().Cmp(newKylinArmMemory); cmpResult <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "After adding request, kylin arm memory usage exceeds limit"})
				return
			}
		}
	}

	if !requestKylinX86Mem.IsZero() {
		if !deptResourceQuota.Spec.Resources.XcResources.HgResource.Limits.Memory().IsZero() || !newKylinArmMemory.IsZero() {
			if cmpResult := deptResourceQuota.Spec.Resources.XcResources.HgResource.Limits.Memory().Cmp(newKylinX86Memory); cmpResult <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "After adding request, kylin hg x86 memory usage exceeds limit"})
				return
			}
		}
	}
	
	// 如果所有检查都通过
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ResourceHandler) DeptResources(c *gin.Context) {
	var deptResource []model.DeptResource
	// 获取所有的pod的资源
	pms, err := h.Handler.metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		log.Errorf("获取pod资源信息失败: %v", err)
	}
	// 将pod资源slice转换成map
	m := make(map[string]*metrics.PodMetrics)
	for _, pm := range pms.Items {
		m[pm.Name] = pm.DeepCopy()
	}

	for _, deptRscQuota := range h.Handler.Informers[DeptResourceQuotaInformer].(*informer.DeptResourceQuotaInformer).List() {
		// 根据部门标签获取该部门所有的pods
		ls := labels.Set{"department": deptRscQuota.Spec.DeptName}
		podList, err := h.Handler.client.CoreV1().Pods("").List(context.TODO(), metaV1.ListOptions{LabelSelector: ls.String()})
		if err != nil {
			log.Errorf("获取部门:%s pod失败:%v", deptRscQuota.Spec.DeptName, err)
		}

		nonXcQuantity := resource.MustParse("0Mi")
		xcQuantity := resource.MustParse("0Mi")
		for _, pod := range podList.Items {
			metric, ok := m[pod.Name]
			if !ok {
				continue
			}

			for _, c := range metric.Containers {
				if strings.Contains(pod.Spec.NodeName, "b") {
					nonXcQuantity.Add(c.Usage.Memory().DeepCopy())
				} else if strings.Contains(pod.Spec.NodeName, "kk") {
					xcQuantity.Add(c.Usage.Memory().DeepCopy())
				}
			}

		}

		// 计算该部门的信创和非信创资源
		var used model.UsedResource
		used.NonXc.Memory = nonXcQuantity.String()
		used.XC.Kylin.Memory = xcQuantity.String()

		deptResource = append(deptResource, model.DeptResource{
			Name: deptRscQuota.Spec.DeptName,
			Resources: model.Resources{
				NonXc: model.ResourceQuotas{
					Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.NonXcResources.Limits.Memory().String()},
				},
				XC: model.SubResource{
					HG: struct{}{},
					Kylin: model.ResourceQuotas{
						Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.XcResources.ArmResource.Limits.Memory().String()},
					},
				},
			},
			Announced: model.Announced{
				NonXc: model.ResourceQuotas{
					Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedNonXcResource.Limits.Memory().String()},
				},
				XC: model.SubResource{
					HG: struct{}{},
					Kylin: model.ResourceQuotas{
						Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedXcResource.ArmResource.Limits.Memory().String()},
					},
				},
			},
			Used: used,
		})

	}
	c.JSON(http.StatusOK, deptResource)
}

// NodeResources return the node resources in cluster
func (h *ResourceHandler) NodeResources(c *gin.Context) {
	var nodeList model.NodeList
	// 从informer中获取node信息
	nodes := h.Handler.Informers[NodeInformer].(*informer.NodeInformer).List()
	m := make(map[string]v1.ResourceList)

	// 查询metric server，获得所有节点的资源瞬时数据
	nodeMetricsList, err := h.Handler.metricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		log.Errorf("获取节点资源失败，错误原因:%v", err)
	}
	for _, node := range nodeMetricsList.Items {
		m[node.Name] = node.Usage
	}

	for _, node := range nodes {
		var nodeType NodeType
		if strings.Contains(node.Name, "b") {
			nodeType = NonXcNode
		} else if strings.Contains(node.Name, "kk") {
			nodeType = XcNode
		} else {
			continue
		}

		nodeMetrics, _ := m[node.Name]

		nodeList.Items = append(nodeList.Items, model.Node{
			Name: node.Name,
			Type: string(nodeType),
			Allocatable: map[string]string{
				"cpu":    node.Status.Allocatable.Cpu().String(),
				"memory": node.Status.Allocatable.Memory().String(),
			},
			Used: map[string]string{
				"cpu":    nodeMetrics.Cpu().String(),
				"memory": nodeMetrics.Memory().String(),
			},
		})
	}
	c.JSON(http.StatusOK, nodeList)
}

// ClusterResources return the cluster resources(so far, only limits memory)
func (h *ResourceHandler) ClusterResources(c *gin.Context) {
	pods := h.Handler.Informers[PodInformer].(*informer.PodInformer).List()

	nonXcQuantity := resource.MustParse("0Mi")
	xcQuantity := resource.MustParse("0Mi")
	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			if strings.Contains(pod.Spec.NodeName, "b") {
				nonXcQuantity.Add(c.Resources.Limits.Memory().DeepCopy())
			} else if strings.Contains(pod.Spec.NodeName, "kk") {
				xcQuantity.Add(c.Resources.Limits.Memory().DeepCopy())
			}
		}
	}

	clusterResource := model.ClusterResource{
		NonXcLimitsResources: map[string]string{
			"memory": nonXcQuantity.String(),
		},
		XcLimitsResources: model.XcLimitsResources{
			Hg: struct{}{},
			Kylin: map[string]string{
				"memory": xcQuantity.String(),
			},
		},
	}

	c.JSON(http.StatusOK, clusterResource)
}

func (h *ResourceHandler) EnvResources(c *gin.Context) {
	dept := c.Query("dept")
	if dept == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dept query parameter is required"})
		return
	}

	labelSelector := labels.Set{"department": dept}
	pods := h.Handler.Informers[PodInformer].(*informer.PodInformer).ListBySelector(labelSelector)
	if len(pods) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable query dept pods"})
		return
	}

	envPods := make(map[string]model.EnvResource)
	for _, pod := range pods {
		namespaceGroup, exists := pod.Labels["namespaceGroup"]
		if exists {
			if _, groupExists := envPods[namespaceGroup]; !groupExists {
				nonXcMemory := resource.MustParse("0Mi")
				kylinMemory := resource.MustParse("0Mi")
				envPods[namespaceGroup] = model.EnvResource{
					Dept:    dept,
					EnvName: namespaceGroup,
					NonXcResource: model.NonXcResource{
						CommonResource: model.CommonResource{
							Limits: model.ComputationResources{
								Memory: &nonXcMemory,
							},
						},
					},
					XcResource: model.XcResource{
						Kylin: model.CommonResource{
							Limits: model.ComputationResources{
								Memory: &kylinMemory,
							},
						},
						Hg: model.CommonResource{},
					},
				}
			}
			for _, c := range pod.Spec.Containers {
				if strings.Contains(pod.Spec.NodeName, "b") {
					envPods[namespaceGroup].NonXcResource.Limits.Memory.Add(c.Resources.Limits.Memory().DeepCopy())
				} else if strings.Contains(pod.Spec.NodeName, "kk") {
					envPods[namespaceGroup].XcResource.Kylin.Limits.Memory.Add(c.Resources.Limits.Memory().DeepCopy())
				}
			}
		}
	}

	c.JSON(http.StatusOK, envPods)
}
