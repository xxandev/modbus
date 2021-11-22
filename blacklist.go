package modbus

import (
	"sync"
	"time"
)

type blacklist struct { //concurrent map read and map write
	mutex   sync.Mutex
	limit   uint
	timeout uint
	ticker  *time.Ticker
	list    map[byte]uint
	nclean  func() error
	nblock  func(id byte) error
}

func (bl *blacklist) SetLimitFailedSendes(value uint) {
	bl.mutex.Lock()
	bl.limit = value
	bl.mutex.Unlock()
}

func (bl *blacklist) SetNoticeClean(fn func() error) {
	bl.mutex.Lock()
	bl.nclean = fn
	bl.mutex.Unlock()
}

func (bl *blacklist) SetDeviceBlock(fn func(id byte) error) {
	bl.mutex.Lock()
	bl.nblock = fn
	bl.mutex.Unlock()
}

func (bl *blacklist) init(limit, timeout uint) {
	bl.mutex.Lock()
	bl.limit = limit
	bl.timeout = timeout
	bl.list = make(map[byte]uint)
	if bl.timeout == 0 {
		bl.timeout = 60
	}
	bl.ticker = time.NewTicker(time.Duration(bl.timeout) * time.Minute)
	go func() {
		for range bl.ticker.C {
			if bl.nclean != nil {
				bl.nclean()
			}
			bl.Clean()
		}
	}()
	bl.mutex.Unlock()
}

func (bl *blacklist) Get(id byte) (blocked bool, notresponse uint) {
	bl.mutex.Lock()
	defer bl.mutex.Unlock()
	if bl.list == nil || bl.limit == 0 {
		return false, 0
	}
	if bl.list[id] == bl.limit {
		if bl.nblock != nil {
			bl.nblock(id)
		}
	}
	bl.list[id]++
	return bl.list[id] > bl.limit, bl.list[id]
}

func (bl *blacklist) ResetTimeoutClean() {
	bl.mutex.Lock()
	if bl.ticker != nil {
		bl.ticker.Reset(time.Duration(bl.timeout) * time.Minute)
	}
	bl.mutex.Unlock()
}

func (bl *blacklist) Plus(id byte) {
	bl.mutex.Lock()
	if bl.list != nil && bl.limit > 0 {
		bl.list[id]++
	}
	bl.mutex.Unlock()
}

func (bl *blacklist) Nullify(id byte) {
	bl.mutex.Lock()
	if bl.list != nil {
		bl.list[id] = 0
	}
	bl.mutex.Unlock()
}

func (bl *blacklist) Clean() {
	bl.mutex.Lock()
	if bl.nclean != nil {
		bl.nclean()
	}
	if bl.list != nil {
		for n := range bl.list {
			bl.list[n] = 0
		}
	}
	bl.mutex.Unlock()
}

func NewBlacklist(limitFailedSendes, timeoutClean uint) *blacklist {
	var bl blacklist
	bl.init(limitFailedSendes, timeoutClean)
	return &bl
}
