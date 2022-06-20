package client

import (
	"github.com/caihcloud/node-annotator/conf"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

func InitClientSet() {
	// init clientset for k8s operation
	// if kubeconf path is empty, Using the inClusterConfig.
	config, err := clientcmd.BuildConfigFromFlags("", conf.KubeconfigPath)
	if err != nil {
		klog.Fatal(err)
	}

	K8sClientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}
}
