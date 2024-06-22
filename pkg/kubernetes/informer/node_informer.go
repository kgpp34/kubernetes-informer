package informer

import (
	"context"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"time"
)

type NodeInformer struct {
	informer cache.SharedIndexInformer
}

// NewNodeInformer create a node informer from deploy
func NewNodeInformer(cs *kubernetes.Clientset) *NodeInformer {
	return &NodeInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
					return cs.CoreV1().Nodes().List(context.TODO(), options)
				},
				WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
					return cs.CoreV1().Nodes().Watch(context.TODO(), options)
				},
			},
			&coreV1.Node{},
			1*time.Second,
			cache.Indexers{}),
	}
}

func (nodeInformer *NodeInformer) List() []*coreV1.Node {
	var nodeList []*coreV1.Node
	nodes := nodeInformer.informer.GetStore().List()
	for _, obj := range nodes {
		node := obj.(*coreV1.Node)
		nodeList = append(nodeList, node)
	}
	return nodeList
}

func (nodeInformer *NodeInformer) Start(stopCh <-chan struct{}) {
	nodeInformer.informer.Run(stopCh)
}

func (nodeInformer *NodeInformer) HasSynced() bool {
	return nodeInformer.informer.HasSynced()
}
