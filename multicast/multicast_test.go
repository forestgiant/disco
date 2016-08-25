package multicast

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSendAndListen(t *testing.T) {
	testMulticastAddress := "[ff12::9000]:21090"

	var tests = []struct {
		m         *Multicast
		payload   []byte
		shouldErr bool
	}{
		{&Multicast{}, nil, true},
		{&Multicast{
			Address: testMulticastAddress,
			Delay:   3,
		}, []byte("Hello World"), false},
		{&Multicast{
			Address: testMulticastAddress,
			Delay:   0,
		}, []byte("Say hello again"), false},
		{&Multicast{
			Address: testMulticastAddress,
			Delay:   1,
		}, []byte("123412341234"), false},
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	sendErrors := make(chan error, 1)

	// Don't block for test
	go func() {
		for _, test := range tests {
			if !test.shouldErr {
				wg.Add(1)

				// Each multicast test will create a listener
				respCh, err := test.m.Listen(ctx)
				if err != nil {
					sendErrors <- err
				}

				// For each test we're going to check if the resp sent over the response
				// channel matches the test's payload
				go func(test struct {
					m         *Multicast
					payload   []byte
					shouldErr bool
				}) {
					defer wg.Done()

					for {
						select {
						case resp := <-respCh:
							if bytes.Equal(resp.Payload, test.payload) {
								fmt.Printf("test %s. resp %s \n",
									string(test.payload), string(resp.Payload))

								test.m.Stop()
							}
						case <-ctx.Done():
							return
						case <-test.m.Done():
							// If the test.m is done check to see if it errored
							if test.m.Err() != nil {
								sendErrors <- test.m.Err()
							}
							return
						case <-time.After(100 * time.Millisecond):
							sendErrors <- errors.New("TestSendAndListen timed out")
							return
						}
					}
				}(test)

				if err := test.m.Send(ctx, test.payload); err != nil {
					t.Fatal("Multicast Send should fail", err)
				}

			} else {
				if err := test.m.Send(ctx, test.payload); err == nil {
					t.Fatal("Multicast Send should fail", err)
				}
			}
		}

		wg.Wait() // Block until all test multicasts are stopped
		cancel()
	}()

	// Block until the ctx is canceled or we receive an error, such as a timeout
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-sendErrors:
			if err != nil {
				t.Fatal("err during Send()", err)
			}
			return
		}
	}
}

func TestStop(t *testing.T) {
	testMulticastAddress := "[ff12::9000]:21090"

	m := &Multicast{
		Address: testMulticastAddress,
		Delay:   3,
	}

	payload := []byte("Hello World")
	if err := m.Send(context.TODO(), payload); err != nil {
		t.Fatal("Send error", err)
	}
	time.AfterFunc(100*time.Millisecond, func() { m.Stop() })
	timeout := time.AfterFunc(200*time.Millisecond, func() { t.Fatal("TestStopChan timedout") })

	// Block until the stopCh is closed
	for {
		select {
		case <-m.Done():
			timeout.Stop() // cancel the timeout
			if m.Err() != nil {
				t.Fatal("m.Err():", m.Err())
			}
			return
		}
	}
}

func TestCtxCancelFunc(t *testing.T) {
	testMulticastAddress := "[ff12::9000]:21090"

	m := &Multicast{
		Address: testMulticastAddress,
		Delay:   3,
	}

	payload := []byte("Hello World")
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
