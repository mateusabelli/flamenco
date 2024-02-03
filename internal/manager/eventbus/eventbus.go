package eventbus

import (
	"sync"
)

type (
	EventTopic string
)

type Forwarder interface {
	Broadcast(topic EventTopic, payload interface{})
}

type Broker struct {
	forwarders []Forwarder
	mutex      sync.Mutex
}

func NewBroker() *Broker {
	return &Broker{
		forwarders: []Forwarder{},
		mutex:      sync.Mutex{},
	}
}

func (b *Broker) AddForwarder(forwarder Forwarder) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.forwarders = append(b.forwarders, forwarder)
}

func (b *Broker) broadcast(topic EventTopic, payload interface{}) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for _, forwarder := range b.forwarders {
		forwarder.Broadcast(topic, payload)
	}
}
