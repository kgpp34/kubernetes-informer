package router

import (
	"k8s-admin-informer/pkg/client"
	"k8s-admin-informer/pkg/handler"
	"k8s-admin-informer/pkg/informer"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type App struct {
	router                    *gin.Engine
	Handler                   *handler.Handler
	PodInformer               *informer.PodInformer
	DeploymentInformer        *informer.DeploymentInformer
	StatefulSetInformer       *informer.StatefulSetInformer
	EventInformer             *informer.EventInformer
	ServiceInformer           *informer.ServiceInformer
	DeptResourceQuotaInformer *informer.DeptResourceQuotaInformer
}

func NewApp() *App {
	return &App{
		router:  gin.Default(),
		Handler: handler.NewHandler(),
	}
}

func (a *App) Register() {
	a.Handler.PodInformer = a.PodInformer
	a.Handler.DeploymentInformer = a.DeploymentInformer
	a.Handler.StatefulSetInformer = a.StatefulSetInformer
	a.Handler.EventInformer = a.EventInformer
	a.Handler.ServiceInformer = a.ServiceInformer
	a.Handler.DeptResourceInformer = a.DeptResourceQuotaInformer
	// 查询工作负载后面的pod和event
	a.router.POST("/informer/v1/getWorkloadInstance", a.Handler.GetWorkloadInstance)
	a.router.POST("/informer/v1/computeDeptResource", a.Handler.ComputeDeptResourceQuotaLimit)
}

func (a *App) Run() {
	// 启动gin http server
	//cs, dc, err := client.NewKubernetesClientFromConfig("/Users/yanglinhan/.kube/config")
	cs, dc, err := client.NewKubernetesClientInCluster()
	if err != nil {
		log.Errorf("创建clientSet失败，错误原因:%v", err)
		panic(err)
	}

	a.PodInformer = informer.NewPodInformer(cs)
	a.EventInformer = informer.NewEventInformer(cs)
	a.DeploymentInformer = informer.NewDeploymentInformer(cs)
	a.StatefulSetInformer = informer.NewStatefulSetInformer(cs)
	a.ServiceInformer = informer.NewServiceInformer(cs)
	a.DeptResourceQuotaInformer = informer.NewDeptResourceQuotaInformer(dc)

	a.Register()
	// 启动informer
	stopCh := make(chan struct{})
	defer close(stopCh)

	go a.PodInformer.Run(stopCh)
	go a.DeploymentInformer.Run(stopCh)
	go a.EventInformer.Run(stopCh)
	go a.StatefulSetInformer.Run(stopCh)
	go a.ServiceInformer.Run(stopCh)
	go a.DeptResourceQuotaInformer.Run(stopCh)

	err = a.router.Run(":8991")
	if err != nil {
		panic(err)
	}

}
