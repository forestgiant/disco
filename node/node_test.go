package node

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

const testMulticastAddress = "[ff12::9000]:21090"

func TestEqual(t *testing.T) {
	var tests = []struct {
		a        *Node
		b        *Node
		expected bool
	}{
		{&Node{}, &Node{}, true},
		{&Node{IPv4Address: "127.0.0.1"}, &Node{IPv4Address: "127.0.0.1"}, true},
		{&Node{IPv4Address: "127.0.0.1"}, &Node{IPv4Address: ""}, false},
		{&Node{IPv6Address: "fe80::aebc:32ff:fe93:4365"}, &Node{IPv6Address: "fe80::aebc:32ff:fe93:4365"}, true},
		{&Node{IPv6Address: "fe80::aebc:32ff:fe93:4365"}, &Node{IPv6Address: ""}, false},
	}

	for _, test := range tests {
		actual := Equal(test.a, test.b)
		if actual != test.expected {
			t.Errorf("Compare failed %v should equal %v.", test.a, test.b)
		}
	}
}

func TestStartPong(t *testing.T) {
	// Setup two test nodes that wait to listen for the others
	// If one hear the other it decrements the WaitGroup
	n1 := &Node{
		IPv4Address: "127.0.0.1",
		ErrChan:     make(chan error),
	}

	// create a second test node
	n2 := &Node{
		IPv4Address: "9.0.0.1",
		ErrChan:     make(chan error),
	}

	wg := new(sync.WaitGroup)
	results := make(chan *Node)
	ctx, cancelFunc := context.WithCancel(context.Background())
	Listen(ctx, testMulticastAddress, results)
	wg.Add(2)

	go func() {
		// Select will block until a result comes in
		for {
			select {
			case rn := <-results:
				fmt.Println("node2 received a ping from", rn)
				// listen for n1
				if Equal(rn, n1) {
					fmt.Println("Found n1!!!", n1)
					wg.Done()
				}

				if Equal(rn, n2) {
					fmt.Println("Found n2!!!", n2)
					wg.Done()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Now ping both listening nodes
	// Encode node to be sent via multicast
	if err := n1.Multicast(ctx, testMulticastAddress); err != nil {
		t.Fatal("n1 ping error", err)
	}
	if err := n2.Multicast(ctx, testMulticastAddress); err != nil {
		t.Fatal("n2 ping error", err)
	}

	wg.Wait()
	cancelFunc()
}
