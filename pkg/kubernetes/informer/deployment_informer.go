package informer

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"time"
)

type DeploymentInformer struct {
	informer cache.SharedIndexInformer
}

func NewDeploymentInformer(cs *kubernetes.Clientset) *DeploymentInformer {
	deploymentInformer := DeploymentInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
					list, err := cs.AppsV1().Deployments(metaV1.NamespaceAll).List(context.TODO(), options)
					if err != nil {
						log.Errorf("list deployment 异常:%v", err)
						return nil, err
					}
					return list, err
				},
				WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
					w, err := cs.AppsV1().Deployments(metaV1.NamespaceAll).Watch(context.TODO(), options)
					if err != nil {
						log.Errorf("watch deployment 异常:%v", err)
						return nil, err
					}
					return w, err
				},
			},
			&appsV1.Deployment{},
			10*time.Second,
			cache.Indexers{},
		),
	}
	indexFunc := genNamespaceDepIndexFunc()
	deploymentInformer.AddIndexer(indexFunc, "namespaceDepIdx")

	_, err := deploymentInformer.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: nil,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDep := oldObj.(*appsV1.Deployment)
			newDep := newObj.(*appsV1.Deployment)
			if oldDep.Namespace == "tdd-m6-biz-docker" && oldDep.Name == "aut" {
				fmt.Println(oldDep)
				fmt.Println(newDep)
			}
		},
		DeleteFunc: nil,
	})
	if err != nil {
		return nil
	}
	return &deploymentInformer
}

// AddIndexer 为Informer增加索引
func (depInformer *DeploymentInformer) AddIndexer(idxFunc cache.IndexFunc, idxName string) {
	err := depInformer.informer.AddIndexers(cache.Indexers{
		idxName: idxFunc,
	})
	if err != nil {
		log.Errorf("增加索引失败:%v", err)
	}
	//log.Infof("增加Deployment索引：%s", idxName)
}

func genNamespaceDepIndexFunc() cache.IndexFunc {
	return func(obj interface{}) ([]string, error) {
		dep := obj.(*appsV1.Deployment)
		return []string{dep.Namespace + "/" + dep.Name}, nil
	}
}

// GetDeployments 查询deployment
func (depInformer *DeploymentInformer) GetDeployments(ns string, name string) []*appsV1.Deployment {
	var res []*appsV1.Deployment

	if ns == "" || name == "" {
		log.Errorf("namespace和name不能为空")
		return res
	}

	deployments, err := depInformer.informer.GetIndexer().ByIndex("namespaceDepIdx", ns+"/"+name)
	if err != nil {
		log.Errorf("根据namespace和name查询deployment异常:%v", err)
		return res
	}

	for _, obj := range deployments {
		pod := obj.(*appsV1.Deployment)
		res = append(res, pod)
	}

	return res
}

func (depInformer *DeploymentInformer) Start(stopCh <-chan struct{}) {
	depInformer.informer.Run(stopCh)
}

func (depInformer *DeploymentInformer) HasSynced() bool {
	return depInformer.informer.HasSynced()
	//return true
}
