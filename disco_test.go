package disco

import (
	"context"
	"sync"
	"testing"

	"gitlab.fg/go/disco/node"
)

const testMulticastAddress = "[ff12::9000]:21090"

func TestRegister(t *testing.T) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc() // stop disco
	d, err := NewDisco(testMulticastAddress)
	if err != nil {
		t.Fatal(err)
	}

	n := new(node.Node)
	n.IPv4Address = "127.0.0.1"

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
		{&node.Node{IPv4Address: "9.0.0.1"}, false},
		{&node.Node{IPv4Address: "8.8.0.1"}, false},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc() // stop disco
	wg := &sync.WaitGroup{}
	d, err := NewDisco(testMulticastAddress)
	if err != nil {
		t.Fatal(err)
	}
	var checkNodes []*node.Node
	var mu sync.Mutex
	discoveredChan, errChan := d.Discover(ctx)

	go func() {
		// Select will block until a result comes in
		for {
			select {
			case rn := <-discoveredChan:
				mu.Lock()
				for _, n := range checkNodes {
					if node.Equal(rn, n) {
						n.Stop() // stop the node from multicasting
						wg.Done()
					}
				}
				mu.Unlock()

			case err := <-errChan:
				t.Fatal(err)
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
			mu.Lock()
			checkNodes = append(checkNodes, test.n)
			mu.Unlock()

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
