# 丐版 WorkerPool

Go WorkerPool 实现，支持：

- 固定数量的 goroutine 初始化
- Worker 空闲超时自动退出
- 任务超时及失败自动重试（最多 3 次）
- 重试失败超过次数（默认 3 次）写入死信队列
