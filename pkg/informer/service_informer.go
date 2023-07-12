package informer

import (
	"context"

	log "github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type ServiceInformer struct {
	informer cache.SharedIndexInformer
}

func NewServiceInformer(cs *kubernetes.Clientset) *ServiceInformer {
	serviceInformer := ServiceInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
					return cs.CoreV1().Services(metaV1.NamespaceAll).List(context.TODO(), options)
				},
				WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
					return cs.CoreV1().Services(metaV1.NamespaceAll).Watch(context.TODO(), options)
				},
			},
			&coreV1.Service{},
			3000,
			cache.Indexers{},
		),
	}
	indexFunc := genNamespaceServiceIndexFunc()
	serviceInformer.AddIndexer(indexFunc, "namespaceSvcIdx")
	return &serviceInformer
}

// AddIndexer 为Informer增加索引
func (serviceInformer *ServiceInformer) AddIndexer(idxFunc cache.IndexFunc, idxName string) {
	err := serviceInformer.informer.AddIndexers(cache.Indexers{
		idxName: idxFunc,
	})
	if err != nil {
		log.Errorf("增加索引失败:%v", err)
	}
	log.Infof("增加Service索引：%s", idxName)
}

func genNamespaceServiceIndexFunc() cache.IndexFunc {
	return func(obj interface{}) ([]string, error) {
		service := obj.(*coreV1.Service)
		return []string{service.Namespace + "/" + service.Name}, nil
	}
}

// GetDeployments 查询deployment
func (serviceInformer *ServiceInformer) GetServices(ns string, name string) []*coreV1.Service {
	var res []*coreV1.Service

	if ns == "" || name == "" {
		log.Errorf("namespace和name不能为空")
		return res
	}

	services, err := serviceInformer.informer.GetIndexer().ByIndex("namespaceSvcIdx", ns+"/"+name)
	if err != nil {
		log.Errorf("根据namespace和name查询deployment异常:%v", err)
		return res
	}

	for _, obj := range services {
		svc := obj.(*coreV1.Service)
		res = append(res, svc)
	}

	return res
}

func (serviceInformer *ServiceInformer) Run(stopCh chan struct{}) {
	serviceInformer.informer.Run(stopCh)
}
