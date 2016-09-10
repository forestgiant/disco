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

func TestRegister(t *testing.T) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc() // stop disco
	d, err := NewDisco(testMulticastAddress)
	if err != nil {
		t.Fatal(err)
	}

	n := &node.Node{}

	d.Register(ctx, n)
	d.Deregister(n)
	nodes := d.GetRegistered()
	if len(nodes) != 0 {
		t.Errorf("TestDeregister: All nodes should be deregistered. Received: %b, Should be: %b \n",
			len(nodes), 0)
	}
}

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
	fmt.Println("here?")

	go func() {
		// Select will block until a result comes in
		for {
			select {
			case rn := <-discoveredChan:
				for _, test := range tests {
					if rn.Equal(test.n) {
						test.n.Stop() // stop the node from multicasting
						fmt.Println("?", rn, test.n)
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

	// Discover nodes
	for _, test := range tests {
		// Add to the WaitGroup for each test that should pass and add it to the nodes to verify
		if !test.shouldErr {
			wg.Add(1)

			if err := d.Register(ctx, test.n); err != nil {
				t.Fatal("Multicast error", err)
			}
		} else {
			if err := d.Register(ctx, test.n); err == nil {
				t.Fatal("Multicast of node should fail", err)
			}
		}

	}

	wg.Wait()
}
