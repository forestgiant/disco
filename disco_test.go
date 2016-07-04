package disco

import (
	"fmt"
	"os"
	"testing"

	"gitlab.fg/go/disco/node"
)

var d *Disco

const testMulticastAddress = "[ff12::9000]:21090"

func TestMain(m *testing.M) {
	var err error
	d, err = NewDisco()
	if err != nil {
		fmt.Println("NewDisco errored")
		os.Exit(1)
	}

	// Run all test
	t := m.Run()

	os.Exit(t)
}

func TestRegister(t *testing.T) {
	n := new(node.Node)
	n.MulticastAddress = testMulticastAddress
	n.IPv4Address = "127.0.0.1"
	n.IgnoreLocalPing = false

	waitChan := make(chan struct{})

	go func() {
		d.Register(n)

		nodes := d.GetRegistered()
		r := nodes[0]

		if r.MulticastAddress != n.MulticastAddress {
			t.Errorf("TestRegister: MulticastAddress not equal. Received: %s, Should be: %s \n",
				r.MulticastAddress, n.MulticastAddress)
		}

		close(waitChan)
	}()

	// Wait till register is complete
	<-waitChan

	// Now let's Deregister the node
	go func() {
		d.Deregister(n)
		nodes := d.GetRegistered()
		if len(nodes) != 0 {
			t.Errorf("TestDeregister: All nodes should be deregistered. Received: %b, Should be: %b \n",
				len(nodes), 0)
		}
	}()
}

func TestDiscover(t *testing.T) {
	nodeChan, errChan := d.Discover(testMulticastAddress)
	go func() {
		select {
		case n := <-nodeChan:
			fmt.Println("Found a node!", n)
		case err := <-errChan:
			fmt.Println("Error!", err)
		}
	}()
}

func TestStop(t *testing.T) {
	// Stop everything!
	d.Stop()
}
