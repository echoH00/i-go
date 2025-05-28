package main

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"
)

const (
	MinWorkers    = 3
	MaxWorkers    = 10
	WorkerIDLE    = time.Millisecond * 50
	TaskTimeout   = time.Millisecond * 100
	CheckInterval = time.Millisecond * 500
)

type Task struct {
	id         int
	retryCount int
	result     bool
}

var (
	currentWorker int
	TaskQueue     = make(chan Task, 100)
	DeadLetterQ   = make(chan Task, 100)
	mu            sync.Mutex
)

// 初始3个worker
func init() {
	for i := 0; i < MinWorkers; i++ {
		mu.Lock()
		currentWorker++
		mu.Unlock()
		addWorker(currentWorker, TaskQueue)
		log.Printf("add [Worker%v]", currentWorker)
	}
}

// 模拟任务处理
func processTask(ctx context.Context, workerID int, task Task) bool {
	result := make(chan bool)
	go func() {
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
		randNum := rand.Intn(3)
		if randNum%2 == 0 {
			result <- true
		} else {
			result <- false
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("[Worker%v] process [task%v] timeout", workerID, task.id)
		return false
	case ret := <-result:
		if !ret {
			log.Printf("[Worker%v] process [task%v] 失败", workerID, task.id)
			return false
		}
		log.Printf("[Worker%v] process [task%v] 成功", workerID, task.id)
		return true
	}
}

func addWorker(workerID int, taskQ chan Task) {
	go func() {
		workerIDLE := time.NewTimer(WorkerIDLE)
		defer workerIDLE.Stop()
		for {
			select {
			case <-workerIDLE.C:
				log.Printf("[Worker%v]空闲5ms自动退出", workerID)
				mu.Lock()
				currentWorker--
				mu.Unlock()
				return
			case task := <-taskQ:
				// 有任务时worker为活动状态 重新计时
				if !workerIDLE.Stop() {
					<-workerIDLE.C
				}
				workerIDLE.Reset(WorkerIDLE)
				processCTX, cancel := context.WithTimeout(context.Background(), TaskTimeout)
				ret := processTask(processCTX, workerID, task)
				cancel()
				if !ret {
					task.retryCount++
					if task.retryCount <= 3 {
						TaskQueue <- task
					} else {
						log.Printf("[Task%v]重试3次都失败，写入死信推队列", task.id)
						DeadLetterQ <- task
					}
				}
			}
		}
	}()
}

// 任务分发器
func dispatcher(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(CheckInterval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("dispatcher context超时退出")
			return
		case <-ticker.C:  // 定时触发
			log.Printf("到达检测周期")
			log.Printf("task长度%v, 当前worker数量%v", len(TaskQueue), currentWorker)
			if len(TaskQueue) > 0 && currentWorker < MaxWorkers {
				addWorkers := MaxWorkers - currentWorker
				for i := 0; i < addWorkers; i++ {
					mu.Lock()
					currentWorker++
					mu.Unlock()
					addWorker(currentWorker, TaskQueue)
				}
			}
		}
	}
}

func main() {
	var wg sync.WaitGroup
	ctx, _ := context.WithTimeout(context.Background(), time.Second*20)
	wg.Add(1)
	go dispatcher(ctx, &wg)
	for i := 0; i < 100; i++ {
		TaskQueue <- Task{id: i}
	}
	wg.Wait()
	time.Sleep(time.Second * 3)
	close(DeadLetterQ)
	log.Println(len(DeadLetterQ))
	for v := range DeadLetterQ {
		log.Printf("%v in deadLetterQueue", v.id)
	}
	log.Println("all Done exit")
}
