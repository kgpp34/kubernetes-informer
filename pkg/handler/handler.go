package handler

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	k8s "k8s-admin-informer/pkg/kubernetes"
	"k8s-admin-informer/pkg/kubernetes/informer"
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
	D             cache.SharedIndexInformer
	stopCh        chan struct{}
}

func NewHandler() (*Handler, error) {
	// 创建k8s client
	// cs, dc, mc, err := k8s.NewKubernetesClientFromConfig("C:\\Users\\cffex\\.kube\\config")
	cs, dc, mc, err := k8s.NewKubernetesClientInCluster()
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
		D: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
					list, err := cs.AppsV1().Deployments(metaV1.NamespaceAll).List(context.TODO(), options)
					if err != nil {
						log.Errorf("list deployment异常:%v", err)
						return nil, err
					}
					return list, nil
				},
				WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
					return cs.AppsV1().Deployments(metaV1.NamespaceAll).Watch(context.TODO(), options)
				},
			},
			&appsV1.Deployment{},
			30*time.Second,
			cache.Indexers{
				"namespaceDepIdx": func(obj interface{}) ([]string, error) {
					dep := obj.(*appsV1.Deployment)
					return []string{dep.Namespace + "/" + dep.Name}, nil
				},
			},
		),
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
	h.stopCh = stopCh

	for name, inf := range h.Informers {
		go func(name string, inf informer.Informer) {
			log.Infof("启动informer: %s", name)
			inf.Start(stopCh)
			log.Infof("informer：%s 已停止", name)
		}(name, inf)
	}

	synced := make([]cache.InformerSynced, 0, len(h.Informers))
	for name, inf := range h.Informers {
		log.Infof("等待:%s同步", name)
		synced = append(synced, inf.HasSynced)
	}
	if !cache.WaitForCacheSync(stopCh, synced...) {
		log.Errorf("等待缓存同步失败")
		return fmt.Errorf("等待缓存同步失败")
	}

	return nil
}
