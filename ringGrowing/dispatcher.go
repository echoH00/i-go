package main

type dispatcher struct {
	processors []*processor
}

func newDispatcher() *dispatcher {
	return &dispatcher{}
}

func (d *dispatcher) dispatch(obj interface{}) {
	for i := 0; i < len(d.processors); i++ {
		d.processors[i].add(obj)
	}
}

func (d *dispatcher) addProcessor(handler func(obj interface{})) {
	p := newProcessor(handler)
	d.processors = append(d.processors, p)
}

func (d *dispatcher) run(stopCh chan struct{}) {
	for i := 0; i < len(d.processors); i++ {
		go d.processors[i].run()
		go d.processors[i].pop()
	}
	<-stopCh
	for i := 0; i < len(d.processors); i++ {
		close(d.processors[i].addCh)
	}
}

type processor struct {
	nextCh  chan interface{}
	addCh   chan interface{}
	buffer  *RingGrowing
	handler func(obj interface{})
}

func newProcessor(handler func(obj interface{})) *processor {
	return &processor{
		nextCh:  make(chan interface{}),
		addCh:   make(chan interface{}),
		buffer:  NewRingGrowing(1024 * 1024),
		handler: handler,
	}
}

// 消费者nextCh中的数据,
func (p *processor) run() {
	for {
		select {
		case obj, ok := <-p.nextCh:
			if !ok {
				return
			}
			p.handler(obj)
		}
	}
}

func (p *processor) add(obj interface{}) {
	p.addCh <- obj
}

/*
第 1 次 select：nextCh = nil, 进入case2
- obj = obj1, pendingObj=obj1 激活nextCh

第 2 次 select: nextCh != nil 进入case1
- nextCh无数据， obj1发送成功 buffer中无数据 pendingObj = nil, nextCh = nil

第 3 次 select: nextCh = nil 进入case2, pendingObj = nil
- obj = obj2, pendingObj=obj2, 激活nextCh

第 4 次 select: nextCh != nil, 假设obj1还没有被消费，进入case2
- pendingObj=obj2, 把obj3写入buffer

第 5 次 select: nextCh != nil, 假设obj消费完成， 进入case1
- obj2 发送成功，从buffer中读到obj3， pendingObj=obj3

第 6 次 select: nextCh !=nil, 如果obj2消费完成则进入case1， 否则进入case2
*/

func (p *processor) pop() {
	defer close(p.nextCh)
	var (
		nextCh     chan interface{}
		pendingObj interface{}
		ok         bool
	)
	for {
		select {
		case nextCh <- pendingObj:
			pendingObj, ok = p.buffer.ReadOne()
			if !ok {
				nextCh = nil
			}
		case obj, ok := <-p.addCh:
			if !ok {
				return
			}
			if pendingObj == nil {
				nextCh = p.nextCh
				pendingObj = obj
			} else {
				p.buffer.WriteOne(obj)
			}
		}
	}
}
