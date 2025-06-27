package main

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	cfg, err := clientcmd.BuildConfigFromFlags("", "kubeconfig")
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	lw := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", "test", fields.Everything())
	sharedInformer := cache.NewSharedInformer(lw, &corev1.Pod{}, 0)

	/*
			sharedInformer shared同一个Reflector
		    informer在NewInformer()是传入eventHandler,只能1个
		    sharedInformer.AddEventHandler()添加多个eventHandler
	*/

	sharedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			fmt.Println("Add pod: ", pod.Name)
		},
	})
	sharedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			fmt.Printf("给Pod: %v, 添加一些自定义属性\n", pod.Name)
		},
	})

	stop := make(chan struct{})
	go sharedInformer.Run(stop)
	// 阻塞等待cacheSync
	if !cache.WaitForCacheSync(stop, sharedInformer.HasSynced) {
		fmt.Println("Wait for cache sync Failed")
		return
	}

	s := sharedInformer.GetStore()
	// 从cache获取数据
	for _, v := range s.List() {
		pod := v.(*corev1.Pod)
		fmt.Printf("List from cache,Pod: %v\n", pod.Name)
	}

	<-stop

}
