package main

import (
	"context"
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func enqueueIfNeeded(obj interface{}, queue workqueue.RateLimitingInterface) {
	pod := obj.(*corev1.Pod)
	if pod.Labels["who"] != "echoH00" {
		key, _ := cache.MetaNamespaceKeyFunc(obj)
		queue.Add(key)
	}
}

// 从workerQueue中Get()对象key，根据key从Indexer中获取对象，给对象增加label
func processNextItem(queue workqueue.RateLimitingInterface, indexer cache.Indexer, clientset *kubernetes.Clientset) bool {
	key, shutdown := queue.Get()
	defer queue.Done(key)
	if shutdown {
		return false
	}
	item, exits, err := indexer.GetByKey(key.(string))
	if !exits || err != nil {
		return false
	}
	pod := item.(*corev1.Pod)
	if pod.Labels["who"] == "echoH00" {
		queue.Forget(key)
		return true
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"who": "echoH00",
			},
		},
	}
	bpatch, _ := json.Marshal(patch)

	_, err = clientset.CoreV1().Pods(pod.Namespace).Patch(context.TODO(), pod.Name, types.StrategicMergePatchType, bpatch, metav1.PatchOptions{})
	// 失败 重新入队列
	if err != nil {
		klog.Fatalf("Patch lables failed: %v", err)
		queue.AddRateLimited(key)
	} else {
		queue.Forget(key)
	}
	return true

}

func main() {
	restcfg, _ := clientcmd.BuildConfigFromFlags("", "./config")
	clientset, _ := kubernetes.NewForConfig(restcfg)
	factory := informers.NewSharedInformerFactory(clientset, time.Minute*10)
	podInfomer := factory.Core().V1().Pods().Informer() // sharedIndexInformer

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter())

	podInfomer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			enqueueIfNeeded(obj, queue)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			enqueueIfNeeded(newObj, queue)
		},
	})

	stopChan := make(chan struct{})
	defer close(stopChan)

	// 启动informer
	go factory.Start(stopChan)
	if !cache.WaitForCacheSync(stopChan, podInfomer.HasSynced) {
		klog.Fatal("等待list之后建立好indexer缓存失败")
	} // 传入的podInfomer.HasSynced是一个回调函数，在cache.WaitForCacheSync被调用

	go wait.Until(func() {
		for processNextItem(queue, podInfomer.GetIndexer(), clientset) {
		}
	}, time.Second, stopChan)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	queue.ShuttingDown()
	klog.Info("Shutdown complate")
}
