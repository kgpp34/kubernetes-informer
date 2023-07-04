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

type PodInformer struct {
	informer cache.SharedIndexInformer
}

// NewPodInformer 新建podInformer
func NewPodInformer(cs *kubernetes.Clientset) *PodInformer {
	podInformer := PodInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
					return cs.CoreV1().Pods(coreV1.NamespaceAll).List(context.TODO(), options)
				},
				WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
					return cs.CoreV1().Pods(coreV1.NamespaceAll).Watch(context.TODO(), options)
				},
			},
			&coreV1.Pod{},
			3000,
			cache.Indexers{},
		),
	}
	// 设置索引
	namespaceSvcIndexFunc := genNamespaceSvcIndexFunc()
	podInformer.AddIndexer(namespaceSvcIndexFunc, "NamespaceReleaseIdx")
	return &podInformer
}

func (podInformer *PodInformer) Run(stopCh chan struct{}) {
	podInformer.informer.Run(stopCh)
}

func genNamespaceSvcIndexFunc() cache.IndexFunc {
	return func(obj interface{}) ([]string, error) {
		pod := obj.(*coreV1.Pod)
		label := pod.GetLabels()["release"]
		if label == "" {
			label = "unknown"
		}
		return []string{pod.Namespace + "/" + label}, nil
	}
}

// AddIndexer 为Informer增加索引
func (podInformer *PodInformer) AddIndexer(idxFunc cache.IndexFunc, idxName string) {
	err := podInformer.informer.AddIndexers(cache.Indexers{
		idxName: idxFunc,
	})
	if err != nil {
		log.Errorf("增加索引失败:%v", err)
	}
	log.Infof("增加Pod索引：%s", idxName)
}

// GetPodsByNsAndParent 根据namespace和parent名查询pod
func (podInformer *PodInformer) GetPodsByNsAndParent(ns string, parentName string) ([]*coreV1.Pod, error) {
	if ns == "" || parentName == "" {
		return nil, nil
	}

	var res []*coreV1.Pod
	// 根据NamespaceReleaseIdx索引进行查询
	pods, err := podInformer.informer.GetIndexer().ByIndex("NamespaceReleaseIdx", ns+"/"+parentName)
	if err != nil {
		return res, nil
	}

	for _, obj := range pods {
		pod := obj.(*coreV1.Pod)
		res = append(res, pod)
	}

	return res, nil
}
