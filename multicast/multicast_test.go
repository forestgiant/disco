package multicast

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"
)

var testMulticastAddress = "[ff12::9000]:30001"

func TestListen(t *testing.T) {
	listener := &Multicast{Address: testMulticastAddress}
	respCh, err := listener.Listen(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	sender := &Multicast{Address: testMulticastAddress}
	if err := sender.Send(context.TODO(), []byte{}); err != nil {
		t.Fatal("Multicast Send should fail", err)
	}

	for {
		select {
		case <-respCh:
			listener.Done()
			sender.Done()
			return
		}
	}

}

func TestSend(t *testing.T) {
	var tests = []struct {
		m         *Multicast
		payload   []byte
		shouldErr bool
	}{
		{&Multicast{}, nil, true},
		{&Multicast{Address: testMulticastAddress}, []byte("Hello TestSendAndListen"), false},
		{&Multicast{Address: testMulticastAddress}, []byte("Say hello again"), false},
		{&Multicast{Address: testMulticastAddress}, []byte("123412341234"), false},
	}

	var found = []struct {
		m         *Multicast
		payload   []byte
		shouldErr bool
	}{}

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	errCh := make(chan error, 1)

	// Don't block for test
	go func() {
		listener := &Multicast{Address: testMulticastAddress}

		// Create a listener
		respCh, err := listener.Listen(ctx)
		if err != nil {
			errCh <- err
		}

		// Check if the resp sent over the response channel matches the test's payload
		go func() {
			for {
				select {
				case resp := <-respCh:
					for _, test := range tests {
						skip := false
						// Check the slice of test to see if the response equals what is expected
						if bytes.Equal(resp.Payload, test.payload) {
							// Skip if it's already been found
							for _, f := range found {
								if bytes.Equal(resp.Payload, f.payload) {
									skip = true
								}
							}

							if skip {
								continue
							}

							found = append(found, test)
							test.m.Stop()
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
			if !test.shouldErr {
				wg.Add(1)
				if err := test.m.Send(ctx, test.payload); err != nil {
					t.Fatal("Multicast Send failed", err)
				}

			} else {
				if err := test.m.Send(ctx, test.payload); err == nil {
					t.Fatal("Multicast Send should fail", err)
				}
			}
		}

		wg.Wait() // Block until all test multicasts are stopped
		listener.Stop()
		cancel()
	}()

	// Block until the ctx is canceled or we receive an error, such as a timeout
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil {
				t.Fatal("err during Send()", err)
			}
			return
		case <-time.After(5000 * time.Millisecond):
			t.Fatal("	 timed out")
			return
		}
	}
}

func TestListenCtxDone(t *testing.T) {
	// Create a listener
	ctx, cancel := context.WithCancel(context.Background())
	listener := &Multicast{Address: testMulticastAddress}
	_, err := listener.Listen(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test context Done()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			}
		}
	}()
	cancel()
}

func TestListenStop(t *testing.T) {
	// Create a listener
	listener := &Multicast{Address: testMulticastAddress}
	_, err := listener.Listen(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	// Test listener Done()
	go func() {
		for {
			select {
			case <-listener.Done():
				return
			}
		}
	}()
	listener.Stop()
}

func TestCtxCancelFunc(t *testing.T) {
	m := &Multicast{Address: testMulticastAddress}

	payload := []byte("Hello TestCtxCancelFunc")
	ctx, cancel := context.WithCancel(context.Background())
	if err := m.Send(ctx, payload); err != nil {
		t.Fatal("Send error", err)
	}
	time.AfterFunc(100*time.Millisecond, func() { cancel() })
	timeout := time.AfterFunc(200*time.Millisecond, func() { t.Fatal("TestStopChan timedout") })

	// Block until the stopCh is closed
	for {
		select {
		case <-ctx.Done():
			timeout.Stop() // cancel the timeout
			return
		}
	}
}
