package app

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"k8s-admin-informer/pkg/handler"
)

type App struct {
	// router is the app router engine
	engine          *gin.Engine
	baseHandler     *handler.Handler
	workloadHandler *handler.WorkloadHandler
	rscHandler      *handler.ResourceHandler
}

func NewK8sAdminInformerApp() *App {
	baseHandler, err := handler.NewHandler()
	if err != nil {
		panic(err)
	}
	return &App{
		engine:          gin.Default(),
		baseHandler:     baseHandler,
		workloadHandler: handler.NewWorkloadHandler(baseHandler),
		rscHandler:      handler.NewResourceHandler(baseHandler),
	}
}

func (a *App) registerRoute() {
	// 查询工作负载后面的pod和event
	a.engine.POST("/informer/v1/getWorkloadInstance", a.workloadHandler.GetWorkloadInstance)
	// 检查当前请求资源是否超过部门配额
	a.engine.POST("/informer/v1/resource/dept/checkLimit", a.rscHandler.ComputeDeptResourceQuotaLimit)
	// 获取节点资源
	a.engine.GET("/informer/v1/resource/node", a.rscHandler.NodeResources)
	// 获取部门资源
	a.engine.GET("/informer/v1/resource/dept", a.rscHandler.DeptResources)
	// 获取集群资源
	a.engine.GET("/informer/v1/resource/cluster", a.rscHandler.ClusterResources)
	// 获取部门资源
	a.engine.GET("/informer/v1/resource/env", a.rscHandler.EnvResources)
	// prometheus metrics
	a.engine.GET("/metrics", a.prometheusHandler())
}

func (a *App) Run() error {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ticker.C:
				a.rscHandler.ProbeDeptResource()
			}
		}
	}()

    // 同步启动 informer，完成后注册事件并进行聚合预热
    if err := a.baseHandler.Start(); err != nil {
        log.Errorf("启动informer出现异常：%v", err)
        return err
    }
    a.rscHandler.EnableEventDrivenInvalidation()
    a.rscHandler.SeedNodeAggFromInformer()

    // 注册路由
    a.registerRoute()

    err := a.engine.Run(":8080")
	if err != nil {
		return err
	}
	return nil
}

func (a *App) prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()

	return func(context *gin.Context) {
		a.rscHandler.ProbeDeptResource()
		h.ServeHTTP(context.Writer, context.Request)
	}
}
