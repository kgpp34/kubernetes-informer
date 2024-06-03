package client

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewKubernetesClientFromConfig 从config中新建clientSet对象
func NewKubernetesClientFromConfig(cfgPath string) (*kubernetes.Clientset, dynamic.Interface, error) {
	// 创建kubernetes 客户端配置
	config, err := clientcmd.BuildConfigFromFlags("", cfgPath)
	if err != nil {
		log.Errorf("获取本地kubeconfig失败: %v\n", err)
		return nil, nil, err
	}

	// 创建kubernetes client
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Errorf("创建client set失败: %v\n", err)
		return nil, nil, err
	}

	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Errorf("创建dynamic client失败: %v\n", err)
		return nil, nil, err
	}

	return cs, dc, nil
}

func NewKubernetesClientInCluster() (*kubernetes.Clientset, dynamic.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("获取集群内config失败: %v", err)
		return nil, nil, err
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Errorf("创建client set失败: %v\n", err)
		return nil, nil, err
	}

	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Errorf("创建dynamic client失败: %v\n", err)
		return nil, nil, err
	}

	return cs, dc, nil
}
