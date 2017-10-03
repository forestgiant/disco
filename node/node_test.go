package node

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forestgiant/disco/multicast"
)

const testMulticastAddress = "[ff12::9000]:30002"

func Test_init(t *testing.T) {
	n := &Node{}
	n.init()

	if n.registerCh == nil {
		t.Fatal("registerCh should not be nil after init() method is called")
	}
}

func TestEncodeDecode(t *testing.T) {
	var tests = []struct {
		n *Node
	}{
		{&Node{}},
		{&Node{
			ipv4:    net.ParseIP("127.0.0.1"),
			ipv6:    net.IPv6loopback,
			Payload: []byte("payload"),
		}},
		{&Node{
			ipv4:    net.ParseIP("127.0.0.1"),
			ipv6:    net.IPv6loopback,
			Action:  DeregisterAction,
			Payload: []byte("payload 2"),
		}},
	}

	var mu sync.Mutex
	for i, test := range tests {
		mu.Lock()
		tn := test.n
		bytes := tn.Encode()
		mu.Unlock()

		decodedN, err := DecodeNode(bytes)
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			mu.Lock()
			if !tn.IPv4().Equal(tn.ipv4) {
				t.Fatal()
			}
			mu.Unlock()
		}()

		mu.Lock()
		if !tn.Equal(decodedN) && tn.Action == decodedN.Action {
			t.Fatalf("Test %d failed. Nodes should be equal after decode. \n Received: %s, \n Expected: %s \n", i, decodedN, test.n)
		}
		mu.Unlock()
	}
}

func TestRegisterCh(t *testing.T) {
	n := &Node{}
	if n.RegisterCh() != n.registerCh {
		t.Fatal("RegisterCh() method should return n.registerCh")
	}
}

func TestKeepRegistered(t *testing.T) {
	n := &Node{}
	closeCh := make(chan struct{})
	errCh := make(chan error)
	go func() {
		defer close(closeCh)
		select {
		case <-n.RegisterCh():
		case <-time.After(100 * time.Millisecond):
			errCh <- errors.New("Test_register timed out")
		}
	}()

	n.KeepRegistered()

	// Block until closeCh is closed on a timed out happens
	for {
		select {
		case <-closeCh:
			return
		case err := <-errCh:
			t.Fatal(err)
		}
	}
}

func TestIPGetters(t *testing.T) {
	localIPv4 := localIPv4()
	localIPv6 := localIPv6()

	var tests = []struct {
		n    *Node
		ipv4 net.IP
		ipv6 net.IP
	}{
		{&Node{}, nil, nil},
		{&Node{}, []byte{}, []byte{}},
		{&Node{ipv4: localIPv4}, localIPv4, []byte{}},
		{&Node{ipv6: localIPv6}, []byte{}, localIPv6},
		{&Node{ipv4: localIPv4, ipv6: localIPv6}, localIPv4, localIPv6},
	}

	for _, test := range tests {
		ipv4 := test.n.IPv4()
		ipv6 := test.n.IPv6()
		if !test.n.ipv4.Equal(ipv4) || !test.n.ipv6.Equal(ipv6) {
			t.Error("IP Getter failed", test.n, ipv4, ipv6)
		}
	}
}

func TestEqual(t *testing.T) {
	var tests = []struct {
		a        *Node
		b        *Node
		expected bool
	}{
		{&Node{}, &Node{}, true},
		{&Node{Payload: []byte("foo, bar")}, &Node{Payload: []byte("foo, bar")}, true},
		{&Node{Payload: []byte("foo, bar")}, &Node{}, false},
	}

	for _, test := range tests {
		actual := test.a.Equal(test.b)
		if actual != test.expected {
			t.Errorf("Compare failed %v should equal %v.", test.a, test.b)
		}
	}
}

func Test_send(t *testing.T) {
	var tests = []struct {
		n          *Node
		shouldPass bool
	}{
		{&Node{}, false},
		{&Node{mc: &multicast.Multicast{Address: testMulticastAddress}}, true},
		{&Node{ipv4: net.IP{}, mc: &multicast.Multicast{Address: testMulticastAddress}}, true},
		{&Node{ipv4: net.IP{}, mc: &multicast.Multicast{}}, false},
		{&Node{ipv4: localIPv4(), ipv6: localIPv6(), mc: &multicast.Multicast{}}, false},
	}

	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithCancel(context.TODO())

	for _, test := range tests {
		err := test.n.send(ctx)
		if (err != nil) == test.shouldPass {
			t.Fatal(err)
		}

		test.n.Stop()
	}

	cancelFunc()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			t.Fatal(err)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("TestMulticastIPChange timed out")
			return
		}
	}
}
func TestMulticastWithError(t *testing.T) {
	n := &Node{}

	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithCancel(context.Background())

	n.mu.Lock()

	// Intentionally leave Address blank to error during multicast Send()
	n.mc = &multicast.Multicast{}

	// Set IPv4 and IPv6 to blank to make sure we prompt a deregister
	n.ipv4 = net.IP{}
	n.ipv6 = net.IP{}

	// Stop existing ticker if it exists
	if n.ticker != nil {
		n.ticker.Stop()
	}

	n.ticker = time.NewTicker(10 * time.Millisecond)
	n.mu.Unlock()

	var count uint64 = 0

	go func() {
		for {
			select {
			case <-n.ticker.C:
				atomic.AddUint64(&count, 1)
				if atomic.LoadUint64(&count) == 3 {
					cancelFunc()
				}

				if err := n.send(context.TODO()); err != nil {
					select {
					case errCh <- err:
					default:
						// Don't block
					}

				}
			case <-ctx.Done():
				n.ticker.Stop()
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		n.ticker.Stop()
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TestMultocastWithError timed out")
		return
	}
}
func TestMulticast(t *testing.T) {
	var tests = []struct {
		n *Node
	}{
		{&Node{SendInterval: 1 * time.Second}},
		{&Node{Payload: []byte("foo, bar"), SendInterval: 1 * time.Second}},
		{&Node{Payload: []byte("somekey, somevalue"), SendInterval: 1 * time.Second}},
	}

	var found = []struct {
		n *Node
	}{}

	ctx, cancelFunc := context.WithCancel(context.TODO())
	errCh := make(chan error, 1)
	wg := &sync.WaitGroup{}

	// Perform our test in a new goroutine so we don't block
	go func() {
		// Listen for nodes
		listener := &multicast.Multicast{Address: testMulticastAddress}
		results, err := listener.Listen(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Check if nodes received are the nodes we are testing
		go func() {
			for {
				select {
				case resp := <-results:
					rn, err := DecodeNode(resp.Payload)
					if err != nil {
						errCh <- err
					}

					for _, test := range tests {
						skip := false
						// Check the slice of test to see if the response equals what is expected
						if rn.Equal(test.n) {
							// Skip if it's already been found
							for _, f := range found {
								if rn.Equal(f.n) {
									skip = true
								}
							}

							if skip {
								continue
							}

							found = append(found, test)
							test.n.Stop()
							wg.Done()
						}
					}
				case <-ctx.Done():
					return
				case <-listener.Done():
					return
				}
			}
		}()

		for _, test := range tests {
			// Add to the WaitGroup for each test
			wg.Add(1)
			mErrCh := test.n.Multicast(ctx, testMulticastAddress)

			go func() {
				err := <-mErrCh
				errCh <- err
			}()
		}

		// Wait for all tests to complete
		wg.Wait()
		listener.Stop()
		cancelFunc()
	}()

	// Block until the ctx is canceled or we receive an error, such as a timeout
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			t.Fatal(err)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("TestMulticast timed out")
			return
		}
	}
}

func TestStop(t *testing.T) {
	n := &Node{Payload: []byte("foo, bar")}

	n.Multicast(context.TODO(), testMulticastAddress)
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
