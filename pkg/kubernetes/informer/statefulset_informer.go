package informer

import (
	"context"
	log "github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type StatefulSetInformer struct {
	informer cache.SharedIndexInformer
}

func NewStatefulSetInformer(cs *kubernetes.Clientset) *StatefulSetInformer {
	statefulSetInformer := StatefulSetInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
					return cs.AppsV1().StatefulSets(metaV1.NamespaceAll).List(context.TODO(), options)
				},
				WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
					return cs.AppsV1().StatefulSets(metaV1.NamespaceAll).Watch(context.TODO(), options)
				},
			},
			&appsV1.StatefulSet{},
			3000,
			cache.Indexers{},
		),
	}
	indexFunc := genNamespaceStatIndexFunc()
	statefulSetInformer.AddIndexer(indexFunc, "namespaceStatIdx")
	return &statefulSetInformer
}

// AddIndexer 为Informer增加索引
func (statefulSetInformer *StatefulSetInformer) AddIndexer(idxFunc cache.IndexFunc, idxName string) {
	err := statefulSetInformer.informer.AddIndexers(cache.Indexers{
		idxName: idxFunc,
	})
	if err != nil {
		log.Errorf("增加索引失败:%v", err)
	}
	log.Infof("增加StatefulSet索引：%s", idxName)
}

func genNamespaceStatIndexFunc() cache.IndexFunc {
	return func(obj interface{}) ([]string, error) {
		dep := obj.(*appsV1.StatefulSet)
		return []string{dep.Namespace + "/" + dep.Name}, nil
	}
}

// GetStatefulSets 查询statefulSets
func (statefulSetInformer *StatefulSetInformer) GetStatefulSets(ns string, name string) []*appsV1.StatefulSet {
	var res []*appsV1.StatefulSet

	if ns == "" || name == "" {
		log.Errorf("namespace和name不能为空")
		return res
	}

	statefulSets, err := statefulSetInformer.informer.GetIndexer().ByIndex("namespaceStatIdx", ns+"/"+name)
	if err != nil {
		log.Errorf("根据namespace和name查询statefulSet异常:%v", err)
		return res
	}

	for _, obj := range statefulSets {
		pod := obj.(*appsV1.StatefulSet)
		res = append(res, pod)
	}

	return res
}

func (statefulSetInformer *StatefulSetInformer) Start(stopCh <-chan struct{}) {
	statefulSetInformer.informer.Run(stopCh)
}

func (statefulSetInformer *StatefulSetInformer) HasSynced() bool {
	return statefulSetInformer.informer.HasSynced()
}
