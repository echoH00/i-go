package main

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"time"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", "kubeconfig")
	if err != nil {
		panic(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	lw := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "pods", "test", fields.Everything())

	// eventhandler在NewInformer()是传入 只能1个
	s, informer := cache.NewInformer(lw, &corev1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			fmt.Printf("Add pod: %v\n", pod.Name)
		},
	})

	stopCh := make(chan struct{})
	go informer.Run(stopCh)

	// 阻塞等待cacheSync
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		fmt.Println("wait for cache sync failed")
		return
	}
        // 从cache获取数据
	for _, v := range s.List() {
		v := v.(*corev1.Pod)
		fmt.Println(v.Name, v.Namespace)
	}

	<-stopCh
}
