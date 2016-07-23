package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"gitlab.fg/go/disco/multicast"
)

const testMulticastAddress = "[ff12::9000]:21090"

func TestStartPong(t *testing.T) {
	// Setup a test node
	n := &Node{
		IPv4Address:      "127.0.0.1",
		MulticastAddress: testMulticastAddress,
		ErrChan:          make(chan error),
	}

	waitChan := make(chan struct{})

	err := n.Listen(func(n *Node) {
		fmt.Println("found this node", n)
		close(waitChan)
	})
	if err != nil {
		t.Fatal("listen error", err)
	}

	// Encode node to be sent via multicast
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err = enc.Encode(n)
	if err != nil {
		t.Fatal(err)
	}

	testMulticast := &multicast.Multicast{
		Retries:      3,
		Timeout:      3,
		StopPingChan: make(chan struct{}),
		StopPongChan: make(chan struct{}),
	}
	go testMulticast.Ping(testMulticastAddress, buf.Bytes(), n.ErrChan)

	fmt.Println("wait till we get a successful ping")
	<-waitChan

}
