package informer

import (
	"context"
	log "github.com/sirupsen/logrus"
	"k8s-admin-informer/api/v1alpha1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type DeptResourceQuotaInformer struct {
	informer cache.SharedIndexInformer
	client   dynamic.Interface
}

func NewDeptResourceQuotaInformer(cs dynamic.Interface) *DeptResourceQuotaInformer {
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(cs, 30, metaV1.NamespaceAll, nil)
	informer := factory.ForResource(schema.GroupVersionResource{
		Group:    "resource.wukong.io",
		Version:  "v1alpha1",
		Resource: "deptresourcequotas",
	}).Informer()

	return &DeptResourceQuotaInformer{
		client:   cs,
		informer: informer,
	}
}

func (d *DeptResourceQuotaInformer) GetDeptResourceQuotaByName(dept string) *v1alpha1.DeptResourceQuota {
	list, err := d.client.Resource(schema.GroupVersionResource{
		Group:    "resource.wukong.io",
		Version:  "v1alpha1",
		Resource: "deptresourcequotas",
	}).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		return nil
	}

	for _, obj := range list.Items {
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
	list, err := d.client.Resource(schema.GroupVersionResource{
		Group:    "resource.wukong.io",
		Version:  "v1alpha1",
		Resource: "deptresourcequotas",
	}).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		return nil
	}
	for _, obj := range list.Items {
		deptResourceQuota := &v1alpha1.DeptResourceQuota{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deptResourceQuota)
		if err != nil {
			log.Errorf("failed to convert unstructed object: %v", err)
			continue
		}

		quotas = append(quotas, deptResourceQuota)
	}

	return quotas
}

func (d *DeptResourceQuotaInformer) Start(stopCh <-chan struct{}) {
	//d.informer.Run(stopCh)
}

func (d *DeptResourceQuotaInformer) HasSynced() bool {
	return d.informer.HasSynced()
}
