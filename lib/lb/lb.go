package lb

import (
	"errors"
	"sync"
)

func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		instances: make(map[string]Algo, 0),
	}
}

type LoadBalancer struct {
	instances map[string]Algo
	Algo      string
	sync.RWMutex
}

func (lb *LoadBalancer) SetClient(id string, instance interface{}) error {
	lb.Lock()
	defer lb.Unlock()
	var l Algo
	var ok bool
	if l, ok = lb.instances[id]; !ok {
		l = GetLbAlgo(lb.Algo)
		lb.instances[id] = l
	}
	return l.Append(instance)
}

func (lb *LoadBalancer) RemoveClient(id string, instance interface{}) error {
	lb.Lock()
	defer lb.Unlock()
	var l Algo
	var ok bool
	if l, ok = lb.instances[id]; !ok {
		return errors.New("not found Client")
	}
	err := l.Remove(instance)
	if l.Empty() {
		delete(lb.instances, id)
	}
	return err
}

func (lb *LoadBalancer) GetClient(id string) (interface{}, error) {
	lb.Lock()
	l, ok := lb.instances[id]
	lb.Unlock()
	if !ok {
		return nil, errors.New("client can not found")
	}
	i, err := l.Next()
	if err != nil {
		return nil, err
	}
	return i, nil
}
