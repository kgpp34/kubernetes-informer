package main

import (
	log "github.com/sirupsen/logrus"
	"k8s-admin-informer/pkg/router"
)

func main() {
	// 设置日志级别
	log.SetLevel(log.InfoLevel)

	// 创建路由
	app := router.NewApp()

	// 运行server服务
	app.Run()
}
