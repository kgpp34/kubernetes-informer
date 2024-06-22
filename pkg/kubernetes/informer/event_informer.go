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

type EventInformer struct {
	informer cache.SharedIndexInformer
}

func NewEventInformer(cs *kubernetes.Clientset) *EventInformer {
	eventInformer := EventInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
					return cs.CoreV1().Events(coreV1.NamespaceAll).List(context.TODO(), options)
				},
				WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
					return cs.CoreV1().Events(coreV1.NamespaceAll).Watch(context.TODO(), options)
				},
			},
			&coreV1.Event{},
			0,
			cache.Indexers{},
		),
	}
	idx := genNamespaceIdx()
	eventInformer.AddIndexer(idx, "NamespaceIdx")
	return &eventInformer
}

func (eventInformer *EventInformer) Start(stopCh <-chan struct{}) {
	eventInformer.informer.Run(stopCh)
}

func genNamespaceIdx() cache.IndexFunc {
	return func(obj interface{}) ([]string, error) {
		ev := obj.(*coreV1.Event)
		return []string{ev.Namespace}, nil
	}
}

// AddIndexer 为Informer增加索引
func (eventInformer *EventInformer) AddIndexer(idxFunc cache.IndexFunc, idxName string) {
	err := eventInformer.informer.AddIndexers(cache.Indexers{
		idxName: idxFunc,
	})
	if err != nil {
		log.Errorf("增加索引失败:%v", err)
	}
	//log.Infof("增加Event索引：%s", idxName)
}

// GetPodEvent 获取pod事件
func (eventInformer *EventInformer) GetPodEvent(ns string, pod string) []*coreV1.Event {
	var res []*coreV1.Event
	if ns == "" || pod == "" {
		return res
	}

	events, err := eventInformer.informer.GetIndexer().ByIndex("NamespaceIdx", ns)
	if err != nil {
		log.Errorf("查询event出现错误：%v", err)
		return res
	}

	for _, obj := range events {
		eve := obj.(*coreV1.Event)
		if eve.InvolvedObject.Kind == "Pod" && eve.InvolvedObject.Name == pod {
			res = append(res, eve)
		}
	}

	return res
}

func (eventInformer *EventInformer) HasSynced() bool {
	return eventInformer.informer.HasSynced()
}
