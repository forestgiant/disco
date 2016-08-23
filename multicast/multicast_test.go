package multicast

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

const testMulticastAddress = "[ff12::9000]:21090"

func TestPingPong(t *testing.T) {
	testMulticast := &Multicast{
		Address: testMulticastAddress,
		Delay:   3,
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	payload := []byte("Hello World")
	errChan := make(chan error)
	respChan := make(chan Response)
	ctx, cancelFunc := context.WithCancel(context.Background())

	go testMulticast.Pong(ctx, respChan, errChan)

	go func() {
		for {
			select {
			case resp := <-respChan:
				fmt.Println("Received Ping from:", resp.SrcIP, "they said:", string(resp.Payload))
				if string(resp.Payload) != string(payload) {
					t.Errorf("message didn't send correctly. Should be %s. Received %s",
						string(payload), string(resp.Payload))
				}
				cancelFunc() // stop ping and pong
				return
			}
		}
	}()

	// Print pong errors
	go func(errc chan error) {
		defer wg.Done()
		for {
			select {
			case err := <-errc:
				fmt.Println("Pong Error:", err)
			case <-ctx.Done():
				return
			}
		}
	}(errChan)

	errChan = make(chan error)
	go testMulticast.Ping(ctx, payload, errChan)

	// Print ping error
	go func(errc chan error) {
		defer wg.Done()
		for {
			select {
			case err := <-errc:
				// fmt.Println()
				// t.Error(err)
				fmt.Println("Ping Error:", err)
			case <-ctx.Done():
				return
			}
		}
	}(errChan)

	// Block until the waitChan is closed by the successful pong callback
	wg.Wait()
}
