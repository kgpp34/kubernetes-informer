package client

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewKubernetesClientFromConfig 从config中新建clientSet对象
func NewKubernetesClientFromConfig(cfgPath string) (*kubernetes.Clientset, error) {
	// 创建kubernetes 客户端配置
	config, err := clientcmd.BuildConfigFromFlags("", cfgPath)
	if err != nil {
		log.Errorf("获取本地kubeconfig失败: %v\n", err)
		return nil, err
	}

	// 创建kubernetes client
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Errorf("创建client set失败: %v\n", err)
		return nil, err
	}

	return cs, nil
}
