package multicast

import (
	"fmt"
	"log"
	"os"
	"testing"
)

const testMulticastAddress = "[ff12::9000]:21090"

var testMulticast *Multicast

func TestMain(m *testing.M) {
	testMulticast = &Multicast{
		Address:      testMulticastAddress,
		Retries:      3,
		Timeout:      3,
		StopPingChan: make(chan struct{}),
		StopPongChan: make(chan struct{}),
	}
	// Run all test
	t := m.Run()

	os.Exit(t)
}

func TestPingPong(t *testing.T) {
	payload := []byte("Hello World")
	waitChan := make(chan struct{})
	errChan := make(chan error)
	respChan := make(chan Response)
	shutdownChan := make(chan struct{})
	defer close(shutdownChan)

	go testMulticast.Pong(respChan, errChan)

	go func() {
		for {
			select {
			case resp := <-respChan:
				fmt.Println("Received Ping from:", resp.SrcIP, "they said:", string(resp.Payload))
				if string(resp.Payload) != string(payload) {
					t.Errorf("message didn't send correctly. Should be %s. Received %s",
						string(payload), string(resp.Payload))
				}

				close(waitChan)
			case <-shutdownChan:
				return
			}

		}
	}()

	// Print pong errors
	go func(errc chan error) {
		for {
			select {
			case err := <-errc:
				fmt.Println("Pong Error:", err)
			case <-shutdownChan:
				return
			}
		}
	}(errChan)

	errChan = make(chan error)
	go testMulticast.Ping(payload, errChan)

	// Print ping error
	go func(errc chan error) {
		for {
			select {
			case err := <-errc:
				// fmt.Println()
				t.Error(err)
				log.Fatal("Ping Error:", err)
			case <-shutdownChan:
				return
			}
		}
	}(errChan)

	// Block until the waitChan is closed by the successful pong callback
	<-waitChan
}

func TestStopPong(t *testing.T) {
	testMulticast.StopPong()
}

func TestStopPing(t *testing.T) {
	testMulticast.StopPing()
}
