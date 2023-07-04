package router

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"k8s-admin-informer/pkg/client"
	"k8s-admin-informer/pkg/handler"
	"k8s-admin-informer/pkg/informer"
)

type App struct {
	router              *gin.Engine
	Handler             *handler.Handler
	PodInformer         *informer.PodInformer
	DeploymentInformer  *informer.DeploymentInformer
	StatefulSetInformer *informer.StatefulSetInformer
	EventInformer       *informer.EventInformer
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
	// 查询工作负载后面的pod和event
	a.router.POST("/informer/v1/getWorkloadInstance", a.Handler.GetWorkloadInstance)
}

func (a *App) Run() {
	// 启动gin http server
	cs, err := client.NewKubernetesClientFromConfig("/deployment/config/kubeconfig")
	if err != nil {
		log.Errorf("创建clientSet失败，错误原因:%v", err)
		panic(err)
	}

	a.PodInformer = informer.NewPodInformer(cs)
	a.EventInformer = informer.NewEventInformer(cs)
	a.DeploymentInformer = informer.NewDeploymentInformer(cs)
	a.StatefulSetInformer = informer.NewStatefulSetInformer(cs)

	a.Register()
	// 启动informer
	stopCh := make(chan struct{})
	defer close(stopCh)
	go a.PodInformer.Run(stopCh)
	go a.DeploymentInformer.Run(stopCh)
	go a.EventInformer.Run(stopCh)
	go a.StatefulSetInformer.Run(stopCh)
	//gin.SetMode(gin.)
	err = a.router.Run(":8080")
	if err != nil {
		panic(err)
	}
}
