package multicast

import (
	"fmt"
	"os"
	"testing"
)

const testMulticastAddress = "[ff12::9000]:21090"

var testMulticast *Multicast

func TestMain(m *testing.M) {
	testMulticast = NewMulticast(testMulticastAddress)
	testMulticast.Payload = []byte("Hello World")

	// Run all test
	t := m.Run()

	os.Exit(t)
}

func TestPingPong(t *testing.T) {
	waitChan := make(chan struct{})
	errChan := make(chan error)
	respChan := make(chan Response)
	shutdownChan := make(chan struct{})
	defer close(shutdownChan)

	go testMulticast.Pong(respChan, errChan)

	go func() {
		for {
			select {
			default:
				resp := <-respChan
				fmt.Println("Received Ping from:", resp.SrcIP, "they said:", string(resp.Payload))
				if string(resp.Payload) != string(testMulticast.Payload) {
					t.Errorf("message didn't send correctly. Should be %s. Received %s",
						string(testMulticast.Payload), string(resp.Payload))
				}

				close(waitChan)
			case <-shutdownChan:
				return
			}

		}
	}()

	// Print pong errors
	go func() {
		for {
			select {
			default:
				fmt.Println("Pong Error:", <-errChan)
			case <-shutdownChan:
				return
			}
		}
	}()

	go testMulticast.Ping()

	// Block until the waitChan is closed by the successful pong callback
	<-waitChan
}

func TestStopPong(t *testing.T) {
	testMulticast.StopPong()
}

func TestStopPing(t *testing.T) {
	testMulticast.StopPing()
}
