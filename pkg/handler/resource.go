package handler

import (
    "context"
    "net/http"
    "strings"
    "time"
    "os"
    "sync"

    "github.com/gin-gonic/gin"
    "github.com/prometheus/client_golang/prometheus"
    log "github.com/sirupsen/logrus"
    v1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/labels"
    metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
    "k8s.io/client-go/tools/cache"

    "k8s-admin-informer/pkg/kubernetes/informer"
    "k8s-admin-informer/pkg/model"
)

type NodeType string

const (
	XcArmNodeType NodeType = "xcArm"
	XcX86NodeType NodeType = "xcX86"
	NonXcNodeType NodeType = "nonXc"
)

type NodePrefix string

const (
	RedHatX86NodePrefix NodePrefix = "b"
	KylinX86NodePrefix  NodePrefix = "hk"
	KylinArmNodePrefix  NodePrefix = "kk"
)

var (
	deptMemResourceQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dept_memory_resource_quota_bytes",
			Help: "Department current resource quota bytes.",
		},
		[]string{"department", "os", "arch"},
	)
	deptUsedMemResource = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dept_used_memory_quota_bytes",
			Help: "Department used memory resource quota bytes.",
		},
		[]string{"department", "os", "arch"},
	)
	deptPodCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dept_current_pods_num_total",
			Help: "Department current pod counts.",
		},
		[]string{"department"},
	)
)

func init() {
	// 注册自定义指标
	prometheus.MustRegister(deptMemResourceQuota)
	prometheus.MustRegister(deptUsedMemResource)
	prometheus.MustRegister(deptPodCount)
}

func (h *ResourceHandler) ProbeDeptResource() {
	deptResources := h.GetDeptResource()
	for _, deptResource := range deptResources {
		// set resource quota gauge
		rhelAmdMemQuota := resource.MustParse(deptResource.Resources.NonXc.Limits.Memory)
		kylinArmMemQuota := resource.MustParse(deptResource.Resources.XC.Arm.Limits.Memory)
		kylinX86MemQuota := resource.MustParse(deptResource.Resources.XC.X86.Limits.Memory)

		deptMemResourceQuota.With(prometheus.Labels{"department": deptResource.Name, "os": "rhel", "arch": "amd64"}).Set(float64(rhelAmdMemQuota.Value()))
		deptMemResourceQuota.With(prometheus.Labels{"department": deptResource.Name, "os": "kylin", "arch": "arm64v8"}).Set(float64(kylinArmMemQuota.Value()))
		deptMemResourceQuota.With(prometheus.Labels{"department": deptResource.Name, "os": "kylin", "arch": "amd64"}).Set(float64(kylinX86MemQuota.Value()))

		// set used mem gauge
		rhelAmdUsedMem := resource.MustParse(deptResource.Announced.NonXc.Limits.Memory)
		kylinArmUsedMem := resource.MustParse(deptResource.Announced.XC.Arm.Limits.Memory)
		kylinX86UsedMem := resource.MustParse(deptResource.Announced.XC.X86.Limits.Memory)

		deptUsedMemResource.With(prometheus.Labels{"department": deptResource.Name, "os": "rhel", "arch": "amd64"}).Set(float64(rhelAmdUsedMem.Value()))
		deptUsedMemResource.With(prometheus.Labels{"department": deptResource.Name, "os": "kylin", "arch": "arm64v8"}).Set(float64(kylinArmUsedMem.Value()))
		deptUsedMemResource.With(prometheus.Labels{"department": deptResource.Name, "os": "kylin", "arch": "amd64"}).Set(float64(kylinX86UsedMem.Value()))
		// set pod count gauge
		deptPodCount.With(prometheus.Labels{"department": deptResource.Name}).Set(float64(deptResource.Pods))
	}
}

// ResourceHandler 负责资源相关的处理：
// - 使用 PodInformer 本地缓存进行一次性遍历与部门聚合
// - 维护部门资源查询缓存与 TTL，降低重复计算与远端请求
type ResourceHandler struct {
    Handler               *Handler
    // deptResourceCache 缓存最近一次部门资源聚合结果
    deptResourceCache     []model.DeptResource
    // deptResourceCacheTime 记录缓存生成时间
    deptResourceCacheTime time.Time
    // cacheTTL 缓存有效期，默认 30s，可通过环境变量配置
    cacheTTL              time.Duration
    // nodeResourceCache 缓存最近一次节点资源聚合结果
    nodeResourceCache     model.NodeList
    // nodeResourceCacheTime 记录节点资源缓存生成时间
    nodeResourceCacheTime time.Time
    // deptRefreshInterval 部门指标后台刷新间隔
    deptRefreshInterval   time.Duration
    // nodeRefreshInterval 节点指标后台刷新间隔
    nodeRefreshInterval   time.Duration
    deptEvents            chan struct{}
    nodeEvents            chan struct{}
    recomputeMu           sync.Mutex
    deptAgg               map[string]*struct{ nonXc, arm, x86 resource.Quantity; pods int }
    podRecords            map[string]struct{ dept string; arch NodeType; mem resource.Quantity }
    nodeAgg               map[string]struct{ allocCPU, allocMem, usedCPU, usedMem string; nodeType NodeType }
}

// DeptRefreshInterval 返回部门资源后台刷新间隔
func (h *ResourceHandler) DeptRefreshInterval() time.Duration {
    return h.deptRefreshInterval
}

// NodeRefreshInterval 返回节点资源后台刷新间隔
func (h *ResourceHandler) NodeRefreshInterval() time.Duration {
    return h.nodeRefreshInterval
}

// NewResourceHandler 初始化资源处理器：
// - 从环境变量 DEPT_RESOURCE_CACHE_TTL 读取缓存 TTL（如 "30s"、"1m"）
// - 未设置或解析失败则使用默认 30s
func NewResourceHandler(handler *Handler) *ResourceHandler {
    defaultTTL := 30 * time.Second
    ttl := defaultTTL
    if v := os.Getenv("DEPT_RESOURCE_CACHE_TTL"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            ttl = d
        } else {
            log.Warnf("解析 DEPT_RESOURCE_CACHE_TTL 失败，使用默认值 %s，错误: %v", defaultTTL.String(), err)
        }
    }
    defaultDeptRefresh := 15 * time.Second
    deptRefresh := defaultDeptRefresh
    if v := os.Getenv("DEPT_RESOURCE_REFRESH_INTERVAL"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            deptRefresh = d
        } else {
            log.Warnf("解析 DEPT_RESOURCE_REFRESH_INTERVAL 失败，使用默认值 %s，错误: %v", defaultDeptRefresh.String(), err)
        }
    }
    defaultNodeRefresh := 10 * time.Second
    nodeRefresh := defaultNodeRefresh
    if v := os.Getenv("NODE_RESOURCE_REFRESH_INTERVAL"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            nodeRefresh = d
        } else {
            log.Warnf("解析 NODE_RESOURCE_REFRESH_INTERVAL 失败，使用默认值 %s，错误: %v", defaultNodeRefresh.String(), err)
        }
    }
    return &ResourceHandler{
        Handler:            handler,
        cacheTTL:           ttl,
        deptRefreshInterval: deptRefresh,
        nodeRefreshInterval: nodeRefresh,
        deptEvents:         make(chan struct{}, 1),
        nodeEvents:         make(chan struct{}, 1),
        deptAgg:            make(map[string]*struct{ nonXc, arm, x86 resource.Quantity; pods int }),
        podRecords:         make(map[string]struct{ dept string; arch NodeType; mem resource.Quantity }),
        nodeAgg:            make(map[string]struct{ allocCPU, allocMem, usedCPU, usedMem string; nodeType NodeType }),
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

	/*// 比较 Spec.Resources.Limits.Memory 与 Status.NonXcMemory
	if cmpResult := deptResourceQuota.Spec.Resources.NonXcResources.Limits.Memory().Cmp(deptResourceQuota.Status.UsedResources.UsedNonXcResource.Limits.Memory().DeepCopy()); cmpResult < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "Non-XC memory usage exceeds limit"})
		return
	}

	// 比较 Spec.XcResources.Limits.Memory 与 Status.XcMemory
	kylinArmMemory := deptResourceQuota.Status.UsedResources.UsedXcResource.ArmResource.Limits.Memory()
	if cmpResult := deptResourceQuota.Spec.Resources.XcResources.ArmResource.Limits.Memory().Cmp(kylinArmMemory.DeepCopy()); cmpResult < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "XC memory usage exceeds limit"})
		return
	}*/

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
		if !deptResourceQuota.Spec.Resources.XcResources.HgResource.Limits.Memory().IsZero() || !newKylinX86Memory.IsZero() {
			if cmpResult := deptResourceQuota.Spec.Resources.XcResources.HgResource.Limits.Memory().Cmp(newKylinX86Memory); cmpResult <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "After adding request, kylin hg x86 memory usage exceeds limit"})
				return
			}
		}
	}

	// 如果所有检查都通过
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeptResources 返回部门资源，支持通过查询参数控制缓存：
// - refresh=true 强制同步重算
// - maxAge=duration 覆盖默认 TTL
// 响应头包含 X-Cache 与 X-Generated-At
func (h *ResourceHandler) DeptResources(c *gin.Context) {
    refresh := c.Query("refresh") == "true"
    maxAgeStr := c.Query("maxAge")
    var maxAge *time.Duration
    if maxAgeStr != "" {
        if d, err := time.ParseDuration(maxAgeStr); err == nil {
            maxAge = &d
        }
    }

    var data []model.DeptResource
    var cacheState string
    if refresh {
        data = h.buildDeptResourceFromAgg()
        cacheState = "MISS"
    } else {
        if maxAge != nil && !h.deptResourceCacheTime.IsZero() && time.Since(h.deptResourceCacheTime) > *maxAge {
            data = h.buildDeptResourceFromAgg()
            cacheState = "MISS"
        } else if len(h.deptResourceCache) > 0 && !h.deptResourceCacheTime.IsZero() && time.Since(h.deptResourceCacheTime) < h.cacheTTL {
            data = h.deptResourceCache
            cacheState = "HIT"
        } else {
            data = h.buildDeptResourceFromAgg()
            cacheState = "MISS"
        }
    }
    c.Header("X-Cache", cacheState)
    if !h.deptResourceCacheTime.IsZero() {
        c.Header("X-Generated-At", h.deptResourceCacheTime.Format(time.RFC3339))
    }
    c.JSON(http.StatusOK, data)
}

// GetDeptResource 聚合并返回部门资源：
// 1) 若缓存未过期直接返回缓存
// 2) 拉取一次 PodMetrics，按 namespace/name 建立映射
// 3) 使用 PodInformer 本地缓存一次遍历所有 Pod，按 department 标签聚合内存用量与 Pod 数
// 4) 合并部门配额对象的限额与已宣布用量，生成响应并写入缓存
func (h *ResourceHandler) GetDeptResource() []model.DeptResource {
    if len(h.deptResourceCache) > 0 && !h.deptResourceCacheTime.IsZero() && time.Since(h.deptResourceCacheTime) < h.cacheTTL {
        return h.deptResourceCache
    }
    data := h.buildDeptResourceFromAgg()
    h.deptResourceCache = data
    h.deptResourceCacheTime = time.Now()
    return data
}

// RecomputeDeptResource 强制重算部门资源并更新缓存
func (h *ResourceHandler) RecomputeDeptResource() []model.DeptResource {
    var deptResource []model.DeptResource
    pms, err := h.Handler.metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metaV1.ListOptions{})
    if err != nil {
        log.Errorf("获取pod资源信息失败: %v", err)
    }
    // 指标映射，键为 namespace/name，避免跨命名空间同名覆盖
    metricsMap := make(map[string]metrics.PodMetrics)
	for _, pm := range pms.Items {
		key := pm.Namespace + "/" + pm.Name
		metricsMap[key] = pm
	}

	pods := h.Handler.Informers[PodInformer].(*informer.PodInformer).List()
    // 部门聚合项：记录非信创/信创(Arm/X86)内存用量与 Pod 数
    type aggItem struct {
        nonXc resource.Quantity
        arm   resource.Quantity
        x86   resource.Quantity
        pods  int
    }
	agg := make(map[string]*aggItem)

	for _, pod := range pods {
		dept := pod.Labels["department"]
		if dept == "" {
			continue
		}
		item, ok := agg[dept]
		if !ok {
			non := resource.MustParse("0Mi")
			arm := resource.MustParse("0Mi")
			x86 := resource.MustParse("0Mi")
			item = &aggItem{nonXc: non, arm: arm, x86: x86, pods: 0}
			agg[dept] = item
		}
		item.pods++

		key := pod.Namespace + "/" + pod.Name
		metric, mok := metricsMap[key]
		if !mok {
			continue
		}

		var isNonXc, isArm, isX86 bool
		node := pod.Spec.NodeName
		if strings.Contains(node, string(RedHatX86NodePrefix)) {
			isNonXc = true
		} else if strings.Contains(node, string(KylinArmNodePrefix)) {
			isArm = true
		} else if strings.Contains(node, string(KylinX86NodePrefix)) {
			isX86 = true
		}

		for _, c := range metric.Containers {
			mem := c.Usage.Memory()
			if mem == nil {
				continue
			}
			if isNonXc {
				item.nonXc.Add(*mem)
			} else if isArm {
				item.arm.Add(*mem)
			} else if isX86 {
				item.x86.Add(*mem)
			}
		}
	}

	for _, deptRscQuota := range h.Handler.Informers[DeptResourceQuotaInformer].(*informer.DeptResourceQuotaInformer).List() {
		a := agg[deptRscQuota.Spec.DeptName]
		var used model.UsedResource
		if a != nil {
			used.NonXc.Memory = a.nonXc.String()
			used.XC.Arm.Memory = a.arm.String()
			used.XC.X86.Memory = a.x86.String()
		} else {
			used.NonXc.Memory = "0Mi"
			used.XC.Arm.Memory = "0Mi"
			used.XC.X86.Memory = "0Mi"
		}

		deptResource = append(deptResource, model.DeptResource{
			Name: deptRscQuota.Spec.DeptName,
			Resources: model.Resources{
				NonXc: model.ResourceQuotas{
					Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.NonXcResources.Limits.Memory().String()},
				},
				XC: model.SubResource{
					X86: model.ResourceQuotas{
						Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.XcResources.HgResource.Limits.Memory().String()},
					},
					Arm: model.ResourceQuotas{
						Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.XcResources.ArmResource.Limits.Memory().String()},
					},
				},
			},
			Announced: model.Announced{
				NonXc: model.ResourceQuotas{
					Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedNonXcResource.Limits.Memory().String()},
				},
				XC: model.SubResource{
					X86: model.ResourceQuotas{
						Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedXcResource.HgResource.Limits.Memory().String()},
					},
					Arm: model.ResourceQuotas{
						Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedXcResource.ArmResource.Limits.Memory().String()},
					},
				},
			},
			Used: used,
			Pods: func() int {
				if a != nil {
					return a.pods
				}
				return 0
			}(),
		})
	}

	h.deptResourceCache = deptResource
	h.deptResourceCacheTime = time.Now()
	return deptResource
}

// NodeResources 返回节点资源，支持通过查询参数控制缓存：
// - refresh=true 强制同步重算
// - maxAge=duration 覆盖默认 TTL
// 响应头包含 X-Cache 与 X-Generated-At
func (h *ResourceHandler) NodeResources(c *gin.Context) {
    refresh := c.Query("refresh") == "true"
    maxAgeStr := c.Query("maxAge")
    var maxAge *time.Duration
    if maxAgeStr != "" {
        if d, err := time.ParseDuration(maxAgeStr); err == nil {
            maxAge = &d
        }
    }

    if !refresh && maxAge == nil && !h.nodeResourceCacheTime.IsZero() && time.Since(h.nodeResourceCacheTime) < h.cacheTTL && len(h.nodeResourceCache.Items) > 0 {
        c.Header("X-Cache", "HIT")
        c.Header("X-Generated-At", h.nodeResourceCacheTime.Format(time.RFC3339))
        c.JSON(http.StatusOK, h.nodeResourceCache)
        return
    }

    h.recomputeMu.Lock()
    aggEmpty := len(h.nodeAgg) == 0
    h.recomputeMu.Unlock()

    var data model.NodeList
    if aggEmpty {
        data = h.RecomputeNodeResources()
    } else {
        data = h.buildNodeListFromAgg()
        h.nodeResourceCache = data
        h.nodeResourceCacheTime = time.Now()
    }
    c.Header("X-Cache", "MISS")
    c.Header("X-Generated-At", h.nodeResourceCacheTime.Format(time.RFC3339))
    c.JSON(http.StatusOK, data)
}

func (h *ResourceHandler) SeedNodeAggFromInformer() {
    nodes := h.Handler.Informers[NodeInformer].(*informer.NodeInformer).List()
    for _, node := range nodes {
        h.onNodeAdd(node)
    }
    h.recomputeMu.Lock()
    h.nodeResourceCache = h.buildNodeListFromAgg()
    h.nodeResourceCacheTime = time.Now()
    h.recomputeMu.Unlock()
}

// RecomputeNodeResources 强制重算节点资源并更新缓存
func (h *ResourceHandler) RecomputeNodeResources() model.NodeList {
    var nodeList model.NodeList
    nodes := h.Handler.Informers[NodeInformer].(*informer.NodeInformer).List()
    m := make(map[string]v1.ResourceList)

    nodeMetricsList, err := h.Handler.metricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), metaV1.ListOptions{})
    if err != nil {
        log.Errorf("获取节点资源失败，错误原因:%v", err)
    }
    for _, node := range nodeMetricsList.Items {
        m[node.Name] = node.Usage
    }

    for _, node := range nodes {
        var nodeType NodeType
        name := node.Name
        if strings.Contains(name, string(RedHatX86NodePrefix)) {
            nodeType = NonXcNodeType
        } else if strings.Contains(name, string(KylinArmNodePrefix)) {
            nodeType = XcArmNodeType
        } else if strings.Contains(name, string(KylinX86NodePrefix)) {
            nodeType = XcX86NodeType
        }

        nodeMetrics, _ := m[name]
        var usedCPU, usedMem string
        if nodeMetrics != nil {
            if cq := nodeMetrics.Cpu(); cq != nil {
                usedCPU = cq.String()
            } else {
                usedCPU = "0"
            }
            if mq := nodeMetrics.Memory(); mq != nil {
                usedMem = mq.String()
            } else {
                usedMem = "0"
            }
        } else {
            usedCPU = "0"
            usedMem = "0"
        }

        nodeList.Items = append(nodeList.Items, model.Node{
            Name: name,
            Type: string(nodeType),
            Allocatable: map[string]string{
                "cpu":    node.Status.Allocatable.Cpu().String(),
                "memory": node.Status.Allocatable.Memory().String(),
            },
            Used: map[string]string{
                "cpu":    usedCPU,
                "memory": usedMem,
            },
        })
    }
    h.nodeResourceCache = nodeList
    h.nodeResourceCacheTime = time.Now()
    return nodeList
}

func (h *ResourceHandler) EnableEventDrivenInvalidation() {
    podInf := h.Handler.Informers[PodInformer].(*informer.PodInformer)
    nodeInf := h.Handler.Informers[NodeInformer].(*informer.NodeInformer)

    _ = podInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            pod := obj.(*v1.Pod)
            h.onPodAdd(pod)
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            oldPod := oldObj.(*v1.Pod)
            newPod := newObj.(*v1.Pod)
            h.onPodUpdate(oldPod, newPod)
        },
        DeleteFunc: func(obj interface{}) {
            pod := obj.(*v1.Pod)
            h.onPodDelete(pod)
        },
    })
    _ = nodeInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            node := obj.(*v1.Node)
            h.onNodeAdd(node)
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            node := newObj.(*v1.Node)
            h.onNodeUpdate(node)
        },
        DeleteFunc: func(obj interface{}) {
            node := obj.(*v1.Node)
            h.onNodeDelete(node)
        },
    })

    go h.deptWorker()
    go h.nodeWorker()
}

func (h *ResourceHandler) triggerDeptEvent() {
    select {
    case h.deptEvents <- struct{}{}:
    default:
    }
}

func (h *ResourceHandler) triggerNodeEvent() {
    select {
    case h.nodeEvents <- struct{}{}:
    default:
    }
}

func (h *ResourceHandler) deptWorker() {
    for {
        <-h.deptEvents
        timer := time.NewTimer(500 * time.Millisecond)
        drain := true
        for drain {
            select {
            case <-h.deptEvents:
                continue
            case <-timer.C:
                drain = false
            }
        }
        // 合并事件后根据增量聚合构建缓存（读取需加锁以避免与事件写入并发）
        h.recomputeMu.Lock()
        h.deptResourceCache = h.buildDeptResourceFromAgg()
        h.deptResourceCacheTime = time.Now()
        h.recomputeMu.Unlock()
    }
}

func (h *ResourceHandler) nodeWorker() {
    for {
        <-h.nodeEvents
        timer := time.NewTimer(500 * time.Millisecond)
        drain := true
        for drain {
            select {
            case <-h.nodeEvents:
                continue
            case <-timer.C:
                drain = false
            }
        }
        // 合并事件后根据增量聚合构建缓存（读取需加锁以避免与事件写入并发）
        h.recomputeMu.Lock()
        h.nodeResourceCache = h.buildNodeListFromAgg()
        h.nodeResourceCacheTime = time.Now()
        h.recomputeMu.Unlock()
    }
}

func (h *ResourceHandler) buildDeptResourceFromAgg() []model.DeptResource {
    var deptResource []model.DeptResource
    for _, deptRscQuota := range h.Handler.Informers[DeptResourceQuotaInformer].(*informer.DeptResourceQuotaInformer).List() {
        a := h.deptAgg[deptRscQuota.Spec.DeptName]
        var used model.UsedResource
        if a != nil {
            used.NonXc.Memory = a.nonXc.String()
            used.XC.Arm.Memory = a.arm.String()
            used.XC.X86.Memory = a.x86.String()
        } else {
            used.NonXc.Memory = "0Mi"
            used.XC.Arm.Memory = "0Mi"
            used.XC.X86.Memory = "0Mi"
        }

        deptResource = append(deptResource, model.DeptResource{
            Name: deptRscQuota.Spec.DeptName,
            Resources: model.Resources{
                NonXc: model.ResourceQuotas{
                    Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.NonXcResources.Limits.Memory().String()},
                },
                XC: model.SubResource{
                    X86: model.ResourceQuotas{
                        Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.XcResources.HgResource.Limits.Memory().String()},
                    },
                    Arm: model.ResourceQuotas{
                        Limits: model.ResourceLimits{Memory: deptRscQuota.Spec.Resources.XcResources.ArmResource.Limits.Memory().String()},
                    },
                },
            },
            Announced: model.Announced{
                NonXc: model.ResourceQuotas{
                    Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedNonXcResource.Limits.Memory().String()},
                },
                XC: model.SubResource{
                    X86: model.ResourceQuotas{
                        Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedXcResource.HgResource.Limits.Memory().String()},
                    },
                    Arm: model.ResourceQuotas{
                        Limits: model.ResourceLimits{Memory: deptRscQuota.Status.UsedResources.UsedXcResource.ArmResource.Limits.Memory().String()},
                    },
                },
            },
            Used: used,
            Pods: func() int { if a != nil { return a.pods } ; return 0 }(),
        })
    }
    return deptResource
}

func (h *ResourceHandler) buildNodeListFromAgg() model.NodeList {
    var nodeList model.NodeList
    for name, rec := range h.nodeAgg {
        nodeList.Items = append(nodeList.Items, model.Node{
            Name: name,
            Type: string(rec.nodeType),
            Allocatable: map[string]string{
                "cpu":    rec.allocCPU,
                "memory": rec.allocMem,
            },
            Used: map[string]string{
                "cpu":    rec.usedCPU,
                "memory": rec.usedMem,
            },
        })
    }
    return nodeList
}

func (h *ResourceHandler) archOf(nodeName string) NodeType {
    if strings.Contains(nodeName, string(RedHatX86NodePrefix)) {
        return NonXcNodeType
    }
    if strings.Contains(nodeName, string(KylinArmNodePrefix)) {
        return XcArmNodeType
    }
    if strings.Contains(nodeName, string(KylinX86NodePrefix)) {
        return XcX86NodeType
    }
    return NonXcNodeType
}

func (h *ResourceHandler) getPodMem(ns, name string) (resource.Quantity, bool) {
    pm, err := h.Handler.metricsClient.MetricsV1beta1().PodMetricses(ns).Get(context.TODO(), name, metaV1.GetOptions{})
    if err != nil {
        return resource.MustParse("0Mi"), false
    }
    total := resource.MustParse("0Mi")
    for _, c := range pm.Containers {
        mem := c.Usage.Memory()
        if mem != nil {
            total.Add(*mem)
        }
    }
    return total, true
}

func (h *ResourceHandler) onPodAdd(pod *v1.Pod) {
    // 无部门标签直接跳过
    dept := pod.Labels["department"]
    if dept == "" { return }
    // 指标不可用时按 0 记录，但仍计入 Pod 数
    mem, ok := h.getPodMem(pod.Namespace, pod.Name)
    if !ok { mem = resource.MustParse("0Mi") }
    arch := h.archOf(pod.Spec.NodeName)
    key := pod.Namespace + "/" + pod.Name
    h.recomputeMu.Lock()
    a := h.deptAgg[dept]
    if a == nil {
        non := resource.MustParse("0Mi")
        arm := resource.MustParse("0Mi")
        x86 := resource.MustParse("0Mi")
        a = &struct{ nonXc, arm, x86 resource.Quantity; pods int }{non, arm, x86, 0}
        h.deptAgg[dept] = a
    }
    switch arch {
    case NonXcNodeType:
        a.nonXc.Add(mem)
    case XcArmNodeType:
        a.arm.Add(mem)
    case XcX86NodeType:
        a.x86.Add(mem)
    }
    a.pods++
    h.podRecords[key] = struct{ dept string; arch NodeType; mem resource.Quantity }{dept: dept, arch: arch, mem: mem}
    h.recomputeMu.Unlock()
    h.triggerDeptEvent()
}

func (h *ResourceHandler) onPodUpdate(oldPod, newPod *v1.Pod) {
    // 读取新旧键与新属性
    oldKey := oldPod.Namespace + "/" + oldPod.Name
    newKey := newPod.Namespace + "/" + newPod.Name
    newDept := newPod.Labels["department"]
    newArch := h.archOf(newPod.Spec.NodeName)
    newMem, ok := h.getPodMem(newPod.Namespace, newPod.Name)
    h.recomputeMu.Lock()
    defer h.recomputeMu.Unlock()
    rec, exists := h.podRecords[oldKey]
    if exists {
        // 若新对象删除部门标签：扣减旧值并减少 Pod 数后退出
        if newDept == "" {
            if a := h.deptAgg[rec.dept]; a != nil {
                switch rec.arch {
                case NonXcNodeType:
                    if a.nonXc.Cmp(rec.mem) >= 0 { a.nonXc.Sub(rec.mem) } else { a.nonXc = resource.MustParse("0Mi") }
                case XcArmNodeType:
                    if a.arm.Cmp(rec.mem) >= 0 { a.arm.Sub(rec.mem) } else { a.arm = resource.MustParse("0Mi") }
                case XcX86NodeType:
                    if a.x86.Cmp(rec.mem) >= 0 { a.x86.Sub(rec.mem) } else { a.x86 = resource.MustParse("0Mi") }
                }
                if a.pods > 0 { a.pods-- }
            }
            delete(h.podRecords, oldKey)
            h.triggerDeptEvent()
            return
        }
        oldMem := rec.mem
        oldDept := rec.dept
        oldArch := rec.arch
        // 指标可用：扣旧加新，处理部门迁移的 Pod 数
        if ok {
            if a := h.deptAgg[oldDept]; a != nil {
                switch oldArch {
                case NonXcNodeType:
                    if a.nonXc.Cmp(oldMem) >= 0 { a.nonXc.Sub(oldMem) } else { a.nonXc = resource.MustParse("0Mi") }
                case XcArmNodeType:
                    if a.arm.Cmp(oldMem) >= 0 { a.arm.Sub(oldMem) } else { a.arm = resource.MustParse("0Mi") }
                case XcX86NodeType:
                    if a.x86.Cmp(oldMem) >= 0 { a.x86.Sub(oldMem) } else { a.x86 = resource.MustParse("0Mi") }
                }
                if oldDept != newDept { if a.pods > 0 { a.pods-- } }
            }
            a2 := h.deptAgg[newDept]
            if a2 == nil {
                non := resource.MustParse("0Mi")
                arm := resource.MustParse("0Mi")
                x86 := resource.MustParse("0Mi")
                a2 = &struct{ nonXc, arm, x86 resource.Quantity; pods int }{non, arm, x86, 0}
                h.deptAgg[newDept] = a2
            }
            switch newArch {
            case NonXcNodeType:
                a2.nonXc.Add(newMem)
            case XcArmNodeType:
                a2.arm.Add(newMem)
            case XcX86NodeType:
                a2.x86.Add(newMem)
            }
            if oldDept != newDept { a2.pods++ }
            // 更新记录为新值
            delete(h.podRecords, oldKey)
            h.podRecords[newKey] = struct{ dept string; arch NodeType; mem resource.Quantity }{dept: newDept, arch: newArch, mem: newMem}
        } else {
            // 指标不可用：保持旧内存，若部门/架构变化则按旧值搬迁
            if a := h.deptAgg[oldDept]; a != nil {
                switch oldArch {
                case NonXcNodeType:
                    if a.nonXc.Cmp(oldMem) >= 0 { a.nonXc.Sub(oldMem) } else { a.nonXc = resource.MustParse("0Mi") }
                case XcArmNodeType:
                    if a.arm.Cmp(oldMem) >= 0 { a.arm.Sub(oldMem) } else { a.arm = resource.MustParse("0Mi") }
                case XcX86NodeType:
                    if a.x86.Cmp(oldMem) >= 0 { a.x86.Sub(oldMem) } else { a.x86 = resource.MustParse("0Mi") }
                }
                if oldDept != newDept { if a.pods > 0 { a.pods-- } }
            }
            a2 := h.deptAgg[newDept]
            if a2 == nil {
                non := resource.MustParse("0Mi")
                arm := resource.MustParse("0Mi")
                x86 := resource.MustParse("0Mi")
                a2 = &struct{ nonXc, arm, x86 resource.Quantity; pods int }{non, arm, x86, 0}
                h.deptAgg[newDept] = a2
            }
            switch newArch {
            case NonXcNodeType:
                a2.nonXc.Add(oldMem)
            case XcArmNodeType:
                a2.arm.Add(oldMem)
            case XcX86NodeType:
                a2.x86.Add(oldMem)
            }
            if oldDept != newDept { a2.pods++ }
            // 记录仍保留旧内存值
            delete(h.podRecords, oldKey)
            h.podRecords[newKey] = struct{ dept string; arch NodeType; mem resource.Quantity }{dept: newDept, arch: newArch, mem: oldMem}
        }
    } else {
        // 无旧记录：按新增逻辑处理
        if newDept == "" { h.triggerDeptEvent(); return }
        if !ok { newMem = resource.MustParse("0Mi") }
        a2 := h.deptAgg[newDept]
        if a2 == nil {
            non := resource.MustParse("0Mi")
            arm := resource.MustParse("0Mi")
            x86 := resource.MustParse("0Mi")
            a2 = &struct{ nonXc, arm, x86 resource.Quantity; pods int }{non, arm, x86, 0}
            h.deptAgg[newDept] = a2
        }
        switch newArch {
        case NonXcNodeType:
            a2.nonXc.Add(newMem)
        case XcArmNodeType:
            a2.arm.Add(newMem)
        case XcX86NodeType:
            a2.x86.Add(newMem)
        }
        a2.pods++
        h.podRecords[newKey] = struct{ dept string; arch NodeType; mem resource.Quantity }{dept: newDept, arch: newArch, mem: newMem}
    }
    h.triggerDeptEvent()
}

func (h *ResourceHandler) onPodDelete(pod *v1.Pod) {
    key := pod.Namespace + "/" + pod.Name
    h.recomputeMu.Lock()
    defer h.recomputeMu.Unlock()
    rec, exists := h.podRecords[key]
    if !exists { return }
    a := h.deptAgg[rec.dept]
    if a != nil {
        switch rec.arch {
        case NonXcNodeType:
            if a.nonXc.Cmp(rec.mem) >= 0 { a.nonXc.Sub(rec.mem) } else { a.nonXc = resource.MustParse("0Mi") }
        case XcArmNodeType:
            if a.arm.Cmp(rec.mem) >= 0 { a.arm.Sub(rec.mem) } else { a.arm = resource.MustParse("0Mi") }
        case XcX86NodeType:
            if a.x86.Cmp(rec.mem) >= 0 { a.x86.Sub(rec.mem) } else { a.x86 = resource.MustParse("0Mi") }
        }
        if a.pods > 0 { a.pods-- }
    }
    delete(h.podRecords, key)
    h.triggerDeptEvent()
}

func (h *ResourceHandler) onNodeAdd(node *v1.Node) {
    name := node.Name
    nodeType := h.archOf(name)
    allocCPU := node.Status.Allocatable.Cpu().String()
    allocMem := node.Status.Allocatable.Memory().String()
    usedCPU := "0"
    usedMem := "0"
    nm, err := h.Handler.metricsClient.MetricsV1beta1().NodeMetricses().Get(context.TODO(), name, metaV1.GetOptions{})
    if err == nil {
        if cq := nm.Usage.Cpu(); cq != nil { usedCPU = cq.String() }
        if mq := nm.Usage.Memory(); mq != nil { usedMem = mq.String() }
    }
    h.recomputeMu.Lock()
    h.nodeAgg[name] = struct{ allocCPU, allocMem, usedCPU, usedMem string; nodeType NodeType }{allocCPU: allocCPU, allocMem: allocMem, usedCPU: usedCPU, usedMem: usedMem, nodeType: nodeType}
    h.recomputeMu.Unlock()
    h.triggerNodeEvent()
}

func (h *ResourceHandler) onNodeUpdate(node *v1.Node) {
    h.onNodeAdd(node)
}

func (h *ResourceHandler) onNodeDelete(node *v1.Node) {
    name := node.Name
    h.recomputeMu.Lock()
    delete(h.nodeAgg, name)
    h.recomputeMu.Unlock()
    h.triggerNodeEvent()
}

// ClusterResources return the cluster resources(so far, only limits memory)
func (h *ResourceHandler) ClusterResources(c *gin.Context) {
	pods := h.Handler.Informers[PodInformer].(*informer.PodInformer).List()

	nonXcQuantity := resource.MustParse("0Mi")
	kylinArmQuantity := resource.MustParse("0Mi")
	kylinX86Quantity := resource.MustParse("0Mi")

	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			if strings.Contains(pod.Spec.NodeName, string(RedHatX86NodePrefix)) {
				nonXcQuantity.Add(c.Resources.Limits.Memory().DeepCopy())
			} else if strings.Contains(pod.Spec.NodeName, string(KylinArmNodePrefix)) {
				kylinArmQuantity.Add(c.Resources.Limits.Memory().DeepCopy())
			} else if strings.Contains(pod.Spec.NodeName, string(KylinX86NodePrefix)) {
				kylinX86Quantity.Add(c.Resources.Limits.Memory().DeepCopy())
			}
		}
	}

	clusterResource := model.ClusterResource{
		NonXcLimitsResources: map[string]string{
			"memory": nonXcQuantity.String(),
		},
		XcLimitsResources: model.XcLimitsResources{
			X86: map[string]string{
				"memory": kylinX86Quantity.String(),
			},
			Arm: map[string]string{
				"memory": kylinArmQuantity.String(),
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
				kylinArmMemory := resource.MustParse("0Mi")
				kylinX86Memory := resource.MustParse("0Mi")

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
						Arm: model.CommonResource{
							Limits: model.ComputationResources{
								Memory: &kylinArmMemory,
							},
						},
						X86: model.CommonResource{
							Limits: model.ComputationResources{
								Memory: &kylinX86Memory,
							},
						},
					},
				}
			}
			for _, c := range pod.Spec.Containers {
				if strings.Contains(pod.Spec.NodeName, string(RedHatX86NodePrefix)) {
					envPods[namespaceGroup].NonXcResource.Limits.Memory.Add(c.Resources.Limits.Memory().DeepCopy())
				} else if strings.Contains(pod.Spec.NodeName, string(KylinArmNodePrefix)) {
					envPods[namespaceGroup].XcResource.Arm.Limits.Memory.Add(c.Resources.Limits.Memory().DeepCopy())
				} else if strings.Contains(pod.Spec.NodeName, string(KylinX86NodePrefix)) {
					envPods[namespaceGroup].XcResource.X86.Limits.Memory.Add(c.Resources.Limits.Memory().DeepCopy())
				}
			}
		}
	}

	c.JSON(http.StatusOK, envPods)
}
