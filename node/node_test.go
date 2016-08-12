package node

import (
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
		{&Node{MulticastAddress: "[ff12::9000]:21090"}, &Node{MulticastAddress: "[ff12::9000]:21090"}, true},
		{&Node{MulticastAddress: "[ff12::9000]:21090"}, &Node{MulticastAddress: ""}, false},
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
		IPv4Address:      "127.0.0.1",
		MulticastAddress: testMulticastAddress,
		ErrChan:          make(chan error),
	}

	// create a second test node
	n2 := &Node{
		IPv4Address:      "9.0.0.1",
		MulticastAddress: testMulticastAddress,
		ErrChan:          make(chan error),
	}

	wg := new(sync.WaitGroup)

	// Listen for n2 Multicast
	results := make(chan *Node)
	n1.Listen(results)
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Select will block until a result comes in
		select {
		case rn := <-results:
			fmt.Println("this node received a ping from", rn)
			// listen for n2
			if Equal(rn, n2) {
				fmt.Println("stop rn", n2)
				if err := n2.StopMulticast(); err != nil {
					t.Error(err)
				}
			}
		}
	}()

	n2.Listen(results)
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Select will block until a result comes in
		select {
		case rn := <-results:
			fmt.Println("node2 received a ping from", rn)
			// listen for n1
			if Equal(rn, n1) {
				fmt.Println("stop!!!", n1)
				if err := n1.StopMulticast(); err != nil {
					t.Error(err)
				}
			}
		}
	}()

	// Now ping both listening nodes
	// Encode node to be sent via multicast
	if err := n1.Multicast(); err != nil {
		t.Fatal("n1 ping error", err)
	}
	if err := n2.Multicast(); err != nil {
		t.Fatal("n2 ping error", err)
	}

	fmt.Println("wait till we get a successful ping")
	wg.Wait()
}
