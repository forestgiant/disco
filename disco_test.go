package disco

import (
	"context"
	"fmt"
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
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc() // stop disco
	d, err := NewDisco(testMulticastAddress)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure we have a node registered for testing
	n1 := &node.Node{
		// MulticastAddress: testMulticastAddress,
		IPv4Address: "9.0.0.1",
	}
	d.Register(ctx, n1)

	n2 := &node.Node{
		// MulticastAddress: testMulticastAddress,
		IPv4Address: "8.8.0.1",
	}
	d.Register(ctx, n2)
	discoveredChan := d.Discover(ctx)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case n := <-discoveredChan:
				fmt.Println("Found a node!!!!", n)
				return
			}
		}
	}()

	wg.Wait()
}
