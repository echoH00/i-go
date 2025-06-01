package main

/*
手动模拟informer
1. ListWatch Pod对象
2. Reflector启动时将Pod对象构建成DeltaFIFO
3. goroutine1 pop(DeltaFIFO)并且做了两件事   1. 更新indexer  2. 将没有team="devops"的pod对象的Key加入到workqueue
4. goroutine2 Get(workQueue) 给Pod 加上team="devops"的label
*/

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

func main() {
	restcfg, _ := clientcmd.BuildConfigFromFlags("", "C:\\Users\\echo\\Desktop\\goproject\\informer\\client\\config")
	clientset, _ := kubernetes.NewForConfig(restcfg)
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Pods("").Watch(context.TODO(), metav1.ListOptions{})
		},
	}
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter())
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	fifo := cache.NewDeltaFIFOWithOptions(cache.DeltaFIFOOptions{
		KeyFunction:  cache.MetaNamespaceKeyFunc,
		KnownObjects: indexer,
	})

	reflector := cache.NewReflector(lw, &corev1.Pod{}, fifo, 0)
	stopChan := make(chan struct{})
	go reflector.Run(stopChan)

	go func() {
		for {
			_, err := fifo.Pop(func(obj interface{}, isInInitialList bool) error {
				deltas := obj.(cache.Deltas)
				for _, d := range deltas {
					pod := d.Object.(*corev1.Pod)
					indexer.Update(pod)

					if pod.Labels["team"] != "devops" {
						fmt.Println(pod.Name, pod.Namespace)
						key, _ := cache.MetaNamespaceKeyFunc(pod)
						queue.Add(key)
					}
				}
				return nil
			})
			if err != nil {
				klog.Errorf("Error processing: %v", err)
			}
		}
	}()

	go func() {
		for {
			key, shutdown := queue.Get()
			fmt.Printf("Get from workqueue: %v\n", key.(string))
			if shutdown {
				break
			}
			namespace, name, _ := cache.SplitMetaNamespaceKey(key.(string))
			obj, exists, err := indexer.GetByKey(key.(string))
			if err != nil || !exists {
				fmt.Printf("indexer中不存在%v\n", name)
				queue.Done(key)
				continue
			}
			pod := obj.(*corev1.Pod)
			if pod.Labels["team"] == "devops" {
				queue.Done(key)
				continue
			}
			patch := []byte(`{"metadata":{"labels": {"team":"devops"}}}`)
			_, err = clientset.CoreV1().Pods(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				queue.AddRateLimited(key)
			} else {
				queue.Forget(key)
			}
			queue.Done(key)
		}
	}()

	select {}
}
