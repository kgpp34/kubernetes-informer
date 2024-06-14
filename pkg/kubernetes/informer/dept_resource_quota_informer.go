package informer

import (
	"context"
	log "github.com/sirupsen/logrus"
	"k8s-admin-informer/api/v1alpha1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type DeptResourceQuotaInformer struct {
	dynamicClient dynamic.Interface
	informer      cache.SharedInformer
}

func NewDeptResourceQuotaInformer(cs dynamic.Interface) *DeptResourceQuotaInformer {
	informer := cache.NewSharedInformer(&cache.ListWatch{
		ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
			return cs.Resource(schema.GroupVersionResource{
				Group:    "resource.wukong.io",
				Version:  "v1alpha1",
				Resource: "deptresourcequotas",
			}).List(context.TODO(), options)
		},
		WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
			return cs.Resource(schema.GroupVersionResource{
				Group:    "resource.wukong.io",
				Version:  "v1alpha1",
				Resource: "deptresourcequotas",
			}).Watch(context.TODO(), options)
		},
	},
		&unstructured.Unstructured{},
		60*time.Second,
	)

	//informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: nil,
	//	UpdateFunc: func(oldObj, newObj interface{}) {
	//		fmt.Println(oldObj, newObj)
	//	},
	//	DeleteFunc: nil,
	//})

	return &DeptResourceQuotaInformer{
		dynamicClient: cs,
		informer:      informer,
	}
}

func (d *DeptResourceQuotaInformer) GetDeptResourceQuotaByName(dept string) *v1alpha1.DeptResourceQuota {
	unstructuredList, err := d.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "resource.wukong.io",
		Version:  "v1alpha1",
		Resource: "deptresourcequotas",
	}).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		log.Errorf("获取deptresourcequota list失败：%v", err)
	}

	for _, obj := range unstructuredList.Items {
		deptResourceQuota := &v1alpha1.DeptResourceQuota{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deptResourceQuota)
		if err != nil {
			log.Errorf("failed to convert unstructed object: %v", err)
			continue
		}

		if dept == deptResourceQuota.Spec.DeptName {
			return deptResourceQuota
		}
	}
	return nil
}

func (d *DeptResourceQuotaInformer) List() []*v1alpha1.DeptResourceQuota {
	var quotas []*v1alpha1.DeptResourceQuota
	unstructuredList, err := d.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "resource.wukong.io",
		Version:  "v1alpha1",
		Resource: "deptresourcequotas",
	}).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		log.Errorf("获取deptresourcequota list失败：%v", err)
	}
	for _, unstructedObj := range unstructuredList.Items {
		deptResourceQuota := &v1alpha1.DeptResourceQuota{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructedObj.Object, deptResourceQuota)
		if err != nil {
			log.Errorf("failed to convert unstructed object: %v", err)
			continue
		}

		quotas = append(quotas, deptResourceQuota)
	}

	return quotas
}

func (d *DeptResourceQuotaInformer) Start(stopCh <-chan struct{}) {
	d.informer.Run(stopCh)
}

func (d *DeptResourceQuotaInformer) HasSynced() bool {
	return d.informer.HasSynced()
}
