package node

import (
	"fmt"
	"testing"
)

const testMulticastAddress = "[ff12::9000]:21090"

func TestStartPong(t *testing.T) {
	// Setup a test node
	n := new(Node)
	n.IPv4Address = "127.0.0.1"
	n.MulticastAddress = testMulticastAddress
	n.IgnoreLocalPing = false // Make sure we ignoreLocalPing for testing

	waitChan := make(chan struct{})

	listenCallback := func(n *Node) {
		fmt.Println("found this node", n)
		close(waitChan)
	}
	err := n.Serve(listenCallback)
	if err != nil {
		t.Fatal("listen error", err)
	}

	go n.multicast.Ping()

	fmt.Println("wait till we get a successful ping")
	<-waitChan

}
