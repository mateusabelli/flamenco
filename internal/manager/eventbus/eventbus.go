package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"sync"
)

type (
	EventTopic string
)

// Listener is the interface for internal components that want to respond to events.
type Listener interface {
	OnEvent(topic EventTopic, payload interface{})
}

// Forwarder is the interface for components that forward events to external systems.
type Forwarder interface {
	Broadcast(topic EventTopic, payload interface{})
}

type Broker struct {
	listeners  []Listener
	forwarders []Forwarder
	mutex      sync.Mutex
}

func NewBroker() *Broker {
	return &Broker{
		listeners:  []Listener{},
		forwarders: []Forwarder{},
		mutex:      sync.Mutex{},
	}
}

func (b *Broker) AddForwarder(forwarder Forwarder) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.forwarders = append(b.forwarders, forwarder)
}

func (b *Broker) AddListener(listener Listener) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.listeners = append(b.listeners, listener)
}

func (b *Broker) broadcast(topic EventTopic, payload interface{}) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for _, listener := range b.listeners {
		listener.OnEvent(topic, payload)
	}

	for _, forwarder := range b.forwarders {
		forwarder.Broadcast(topic, payload)
	}
}
