package informer

import (
	"k8s-admin-informer/api/v1alpha1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type DeptResourceQuotaInformer struct {
	informer cache.SharedIndexInformer
}

func NewDeptResourceQuotaInformer(cs dynamic.Interface) *DeptResourceQuotaInformer {
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(cs, 0, metaV1.NamespaceAll, nil)
	informer := factory.ForResource(schema.GroupVersionResource{
		Group:    "resource.wukong.io",
		Version:  "v1alpha1",
		Resource: "deptresourcequotas",
	}).Informer()

	return &DeptResourceQuotaInformer{
		informer: informer,
	}
}

func (d *DeptResourceQuotaInformer) GetDeptResourceQuotaByName(dept string) *v1alpha1.DeptResourceQuota {
	for _, obj := range d.informer.GetStore().List() {
		deptResourceQuota, ok := obj.(*v1alpha1.DeptResourceQuota)
		if !ok {
			continue
		}

		if dept == deptResourceQuota.Spec.DeptName {
			return deptResourceQuota
		}
	}
	return nil
}

func (d *DeptResourceQuotaInformer) Run(stopCh chan struct{}) {
	d.informer.Run(stopCh)
}
