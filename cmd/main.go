package main

import (
	log "github.com/sirupsen/logrus"
	"k8s-admin-informer/pkg/app"
)

func main() {
	// 设置日志级别
	log.SetLevel(log.InfoLevel)
	//gin.SetMode(gin.ReleaseMode)

	// 创建路由
	server := app.NewK8sAdminInformerApp()

	// 运行server服务
	if err := server.Run(); err != nil {
		//panic(err)
	}

}
