package main

type RingGrowing struct {
	buffer    []interface{}
	readPos   int
	writePos  int
	elemCount int
}

func NewRingGrowing(elemcount int) *RingGrowing {
	return &RingGrowing{
		buffer:    make([]interface{}, elemcount),
		readPos:   0,
		writePos:  0,
		elemCount: 0,
	}
}

func (r *RingGrowing) WriteOne(val interface{}) {
	if r.elemCount == len(r.buffer) {
		r.grow()
	}
	r.buffer[r.writePos] = val
	r.writePos = (r.writePos + 1) % len(r.buffer)
	r.elemCount++
}

func (r *RingGrowing) ReadOne() (interface{}, bool) {
	if r.elemCount == 0 {
		return nil, false
	}
	val := r.buffer[r.readPos]
	r.readPos = (r.readPos + 1) % len(r.buffer)
	r.elemCount--
	return val, true
}

func (r *RingGrowing) grow() {
	oldCap := len(r.buffer)
	// 两倍扩容
	newCap := oldCap * 2
	newBuf := make([]interface{}, newCap)

	for i := 0; i < r.elemCount; i++ {
		newBuf[i] = r.buffer[(r.readPos+i)/oldCap]
	}
	r.buffer = newBuf
	r.readPos = 0
	r.writePos = r.elemCount
}
