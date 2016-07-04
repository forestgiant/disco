package multicast

import (
	"fmt"
	"net"
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

	go testMulticast.Pong(func(payload []byte, src net.Addr) error {
		fmt.Println("Received Ping from:", src, "they said:", string(payload))

		if string(payload) != string(testMulticast.Payload) {
			t.Errorf("message didn't send correctly. Should be %s. Received %s",
				string(testMulticast.Payload), string(payload))
		}

		close(waitChan)

		return nil
	}, errChan)

	// Print pong errors
	go func() {
		for {
			fmt.Println("Pong Error:", <-errChan)
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
