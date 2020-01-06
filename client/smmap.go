package client

import (
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

type Timer struct {
	ReqRoute string
	ReqAt    time.Time
}

type SmStrTimer struct {
	sm sync.Map
}

func (m *SmStrTimer) Load(key string) (value *Timer, ok bool) {
	v, ok := m.sm.Load(key)
	if ok {
		rv, aok := v.(*Timer)
		if !aok {
			panic("type assert error")
		}
		return rv, true
	}
	return nil, false
}

func (m *SmStrTimer) LoadOrDefault(key string, defval *Timer) *Timer {
	v, ok := m.Load(key)
	if !ok {
		return defval
	}
	return v
}

func (m *SmStrTimer) LoadAndDelete(key string) (value *Timer, ok bool) {
	v, ok := m.sm.Load(key)
	if ok {
		rv, aok := v.(*Timer)
		if !aok {
			panic("type assert error")
		}
		m.sm.Delete(key)
		return rv, true
	}
	return nil, false
}

func (m *SmStrTimer) Store(key string, value *Timer) {
	m.sm.Store(key, value)
}

func (m *SmStrTimer) LoadOrStore(key string, value *Timer) (actual *Timer, loaded bool) {
	a, l := m.sm.LoadOrStore(key, value)
	v, ok := a.(*Timer)
	if !ok {
		panic("type assert error")
	}

	return v, l
}

func (m *SmStrTimer) Delete(key string) {
	m.sm.Delete(key)
}

func (m *SmStrTimer) Range(f func(key string, value *Timer) bool) {
	m.sm.Range(func(k, v interface{}) bool {
		rk, ok := k.(string)
		if !ok {
			panic("type assert error")
		}
		rv, ok := v.(*Timer)
		if !ok {
			panic("type assert error")
		}

		return f(rk, rv)
	})
}

func (m *SmStrTimer) Len() int {
	var total int
	m.sm.Range(func(_, _ interface{}) bool {
		total++
		return true
	})
	return total
}

type SmStrHistogram struct {
	sm sync.Map
}

func (m *SmStrHistogram) Load(key string) (value *metrics.Histogram, ok bool) {
	v, ok := m.sm.Load(key)
	if ok {
		rv, aok := v.(*metrics.Histogram)
		if !aok {
			panic("type assert error")
		}
		return rv, true
	}
	return nil, false
}

func (m *SmStrHistogram) LoadOrDefault(key string, defval *metrics.Histogram) *metrics.Histogram {
	v, ok := m.Load(key)
	if !ok {
		return defval
	}
	return v
}

func (m *SmStrHistogram) LoadAndDelete(key string) (value *metrics.Histogram, ok bool) {
	v, ok := m.sm.Load(key)
	if ok {
		rv, aok := v.(*metrics.Histogram)
		if !aok {
			panic("type assert error")
		}
		m.sm.Delete(key)
		return rv, true
	}
	return nil, false
}

func (m *SmStrHistogram) Store(key string, value *metrics.Histogram) {
	m.sm.Store(key, value)
}

func (m *SmStrHistogram) LoadOrStore(key string, value *metrics.Histogram) (actual *metrics.Histogram, loaded bool) {
	a, l := m.sm.LoadOrStore(key, value)
	v, ok := a.(*metrics.Histogram)
	if !ok {
		panic("type assert error")
	}

	return v, l
}

func (m *SmStrHistogram) Delete(key string) {
	m.sm.Delete(key)
}

func (m *SmStrHistogram) Range(f func(key string, value *metrics.Histogram) bool) {
	m.sm.Range(func(k, v interface{}) bool {
		rk, ok := k.(string)
		if !ok {
			panic("type assert error")
		}
		rv, ok := v.(*metrics.Histogram)
		if !ok {
			panic("type assert error")
		}

		return f(rk, rv)
	})
}

func (m *SmStrHistogram) Len() int {
	var total int
	m.sm.Range(func(_, _ interface{}) bool {
		total++
		return true
	})
	return total
}
