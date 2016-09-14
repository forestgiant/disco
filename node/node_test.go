package node

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"gitlab.fg/go/disco/multicast"
)

const testMulticastAddress = "[ff12::9000]:21090"

func TestEqual(t *testing.T) {
	var tests = []struct {
		a        *Node
		b        *Node
		expected bool
	}{
		{&Node{}, &Node{}, true},
		{&Node{Values: map[string]string{"foo": "v1", "bar": "v2"}}, &Node{Values: map[string]string{"foo": "v1", "bar": "v2"}}, true},
		{&Node{Values: map[string]string{"foo": "v1", "bar": "v2"}}, &Node{}, false},
	}

	for _, test := range tests {
		actual := test.a.Equal(test.b)
		if actual != test.expected {
			t.Errorf("Compare failed %v should equal %v.", test.a, test.b)
		}
	}
}

func TestMulticast(t *testing.T) {
	var tests = []struct {
		n                *Node
		multicastAddress string
		shouldErr        bool
	}{
		{&Node{SendInterval: 1 * time.Second}, "[ff12::9000]:21090", false},
		// {&Node{Values: map[string]string{"foo": "v1", "bar": "v2"}, SendInterval: 1 * time.Second}, "[ff12::9000]:21090", false},
		// {&Node{Values: map[string]string{"someKey": "someValue"}, SendInterval: 1 * time.Second}, "[ff12::9000]:21090", false},
		// {&Node{Values: map[string]string{"anotherKey": "anotherValue"}, SendInterval: 1 * time.Second}, ":21090", true},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	errChan := make(chan error, 1)
	wg := &sync.WaitGroup{}

	// Listen for nodes
	listener := &multicast.Multicast{Address: testMulticastAddress}
	results, err := listener.Listen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()

	// Check if nodes received are the nodes we are testing
	go func() {
		for {
			select {
			case resp := <-results:
				buffer := bytes.NewBuffer(resp.Payload)

				rn := &Node{}
				dec := gob.NewDecoder(buffer)
				if err := dec.Decode(rn); err != nil {
					errChan <- err
				}

				// Check if any nodes coming in are the ones we are waiting for
				for _, test := range tests {
					if rn.Equal(test.n) {
						test.n.Stop() // stop the node from multicasting
						wg.Done()
					}
				}
			case <-time.After(100 * time.Millisecond):
				errChan <- errors.New("TestMulticast timed out")
			case <-ctx.Done():
				return
			}
		}
	}()

	// Perform our test in a new goroutine so we don't block
	go func() {
		for _, test := range tests {
			// Add to the WaitGroup for each test that should pass and add it to the nodes to verify
			if !test.shouldErr {
				wg.Add(1)

				if err := test.n.Multicast(ctx, test.multicastAddress); err != nil {
					t.Fatal("Multicast error", err)
				}
			} else {
				if err := test.n.Multicast(ctx, test.multicastAddress); err == nil {
					t.Fatal("Multicast of node should fail", err)
				}
			}
		}

		wg.Wait()
		fmt.Println("stop waiting")
		cancelFunc()
	}()

	// Block until the ctx is canceled or we receive an error, such as a timeout
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			t.Fatal(err)
		}
	}
}

func TestStop(t *testing.T) {
	n := &Node{Values: map[string]string{"foo": "v1", "bar": "v2"}}

	if err := n.Multicast(context.TODO(), testMulticastAddress); err != nil {
		t.Fatal("Multicast error", err)
	}
	time.AfterFunc(100*time.Millisecond, func() { n.Stop() })
	timeout := time.AfterFunc(200*time.Millisecond, func() { t.Fatal("TestStopChan timedout") })

	// Block until the stopCh is closed
	for {
		select {
		case <-n.Done():
			timeout.Stop() // cancel the timeout
			return
		}
	}
}

func TestLocalIPv4(t *testing.T) {
	l := localIPv4()
	_, err := net.InterfaceAddrs()
	if err != nil {
		// if there was an error with the interface
		// then it should be loopback
		if !l.IsLoopback() {
			t.Error("LocalIP should be loopback")
		}
	}
}

func TestLocalIPv6(t *testing.T) {
	l := localIPv6()
	_, err := net.InterfaceAddrs()
	if err != nil {
		// if there was an error with the interface
		// then it should be loopback
		if !l.IsLoopback() {
			t.Error("LocalIP should be loopback")
		}
	}
}
