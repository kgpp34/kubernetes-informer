package handler

import (
	log "github.com/sirupsen/logrus"
	k8s "k8s-admin-informer/pkg/kubernetes"
	"k8s-admin-informer/pkg/kubernetes/informer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	DeploymentInformer        string = "deployment"
	StatefulSetInformer       string = "statefulSet"
	NodeInformer              string = "node"
	PodInformer               string = "pod"
	DeptResourceQuotaInformer string = "deptResourceQuota"
	EventInformer             string = "event"
	ServiceInformer           string = "service"
)

type Handler struct {
	client        *kubernetes.Clientset
	dynamicClient dynamic.Interface
	metricsClient *metricsv.Clientset
	Informers     map[string]informer.Informer
}

func NewHandler() (*Handler, error) {
	// 创建k8s client
	cs, dc, mc, err := k8s.NewKubernetesClientFromConfig("C:\\Users\\cffex\\.kube\\config")
	//cs, dc, err := deploy.NewKubernetesClientInCluster()
	if err != nil {
		log.Errorf("创建clientSet失败，错误原因:%v", err)
		return nil, err
	}
	return &Handler{
		client:        cs,
		dynamicClient: dc,
		metricsClient: mc,
		Informers: map[string]informer.Informer{
			DeploymentInformer:        &informer.DeploymentInformer{},
			StatefulSetInformer:       &informer.StatefulSetInformer{},
			PodInformer:               &informer.PodInformer{},
			EventInformer:             &informer.EventInformer{},
			DeptResourceQuotaInformer: &informer.DeptResourceQuotaInformer{},
			NodeInformer:              &informer.NodeInformer{},
			ServiceInformer:           &informer.ServiceInformer{},
		},
	}, nil
}

func (h *Handler) Start() error {
	// new各类informer
	h.Informers[DeploymentInformer] = informer.NewDeploymentInformer(h.client)
	h.Informers[StatefulSetInformer] = informer.NewStatefulSetInformer(h.client)
	h.Informers[PodInformer] = informer.NewPodInformer(h.client)
	h.Informers[ServiceInformer] = informer.NewServiceInformer(h.client)
	h.Informers[EventInformer] = informer.NewEventInformer(h.client)
	h.Informers[NodeInformer] = informer.NewNodeInformer(h.client)
	h.Informers[DeptResourceQuotaInformer] = informer.NewDeptResourceQuotaInformer(h.dynamicClient)

	// 启动informer
	stopCh := make(chan struct{})
	defer close(stopCh)
	for _, inf := range h.Informers {
		go inf.Start(stopCh)
	}

	// 检测同步是否完成
	synced := make([]cache.InformerSynced, 0, len(h.Informers))
	for _, inf := range h.Informers {
		synced = append(synced, inf.HasSynced)
	}

	if !cache.WaitForCacheSync(stopCh, synced...) {
		log.Errorf("等待缓存同步失败")
	}

	return nil
}
