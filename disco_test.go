package disco

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"gitlab.fg/go/disco/node"
)

var testMulticastAddress = "[ff12::9000]:21090"

// func Test_register(t *testing.T) {
// 	ctx, cancelFunc := context.WithCancel(context.Background())
// 	defer cancelFunc() // stop disco
// 	d, err := NewDisco(testMulticastAddress)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	n := &node.Node{}

// 	d.register(ctx, n)
// 	d.deregister(n)
// 	nodes := d.Members()
// 	if len(nodes) != 0 {
// 		t.Errorf("TestDeregister: All nodes should be deregistered. Received: %b, Should be: %b \n",
// 			len(nodes), 0)
// 	}
// }

func TestDiscover(t *testing.T) {
	var tests = []struct {
		n         *node.Node
		shouldErr bool
	}{
		{&node.Node{}, false},
		{&node.Node{SendInterval: 2 * time.Second}, false},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc() // stop disco
	wg := &sync.WaitGroup{}
	d, err := NewDisco(testMulticastAddress)
	if err != nil {
		t.Fatal(err)
	}
	discoveredChan, err := d.Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		// Select will block until a result comes in
		for {
			select {
			case rn := <-discoveredChan:
				for _, test := range tests {
					if rn.Equal(test.n) {
						test.n.Stop() // stop the node from multicasting
						wg.Done()
					} else {
						fmt.Println("not equal")
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Multicast nodes so they can be discovered
	for _, test := range tests {
		// Add to the WaitGroup for each test that should pass and add it to the nodes to verify
		if !test.shouldErr {
			wg.Add(1)

			if err := test.n.Multicast(ctx, testMulticastAddress); err != nil {
				t.Fatal("Multicast error", err)
			}
		} else {
			if err := test.n.Multicast(ctx, testMulticastAddress); err == nil {
				t.Fatal("Multicast of node should fail", err)
			}
		}

	}

	wg.Wait()
}

func TestDiscoverSameNode(t *testing.T) {
	var tests = []struct {
		n         *node.Node
		shouldErr bool
	}{
		{&node.Node{}, false},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc() // stop disco
	wg := &sync.WaitGroup{}
	d, err := NewDisco(testMulticastAddress)
	if err != nil {
		t.Fatal(err)
	}
	discoveredChan, err := d.Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		// Select will block until a result comes in
		for {
			select {
			case rn := <-discoveredChan:
				for _, test := range tests {
					if rn.Equal(test.n) {
						test.n.Stop() // stop the node from multicasting
						wg.Done()
					} else {
						fmt.Println("not equal")
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Multicast nodes so they can be discovered
	for _, test := range tests {
		// Add to the WaitGroup for each test that should pass and add it to the nodes to verify
		if !test.shouldErr {
			wg.Add(1)

			if err := test.n.Multicast(ctx, testMulticastAddress); err != nil {
				t.Fatal("Multicast error", err)
			}
		} else {
			if err := test.n.Multicast(ctx, testMulticastAddress); err == nil {
				t.Fatal("Multicast of node should fail", err)
			}
		}

	}

	wg.Wait()
}
