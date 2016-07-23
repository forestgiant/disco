package node

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net"

	"gitlab.fg/go/disco/multicast"
)

// Modes of a node being registered
// const (
// 	RegisterAction   = iota
// 	DeregisterAction = iota
// )

// ListenCallback called when a listener is pinged
type ListenCallback func(n *Node)

// Node represents a machine registered with Disco
type Node struct {
	IPv6Address      string
	IPv4Address      string
	SrcIP            net.Addr
	MulticastAddress string
	ErrChan          chan error
	shutdownChan     chan struct{}
	// Action           int
	multicast *multicast.Multicast

	RespondLocalPing bool // Used for testing
}

// GobEncode gob interface
func (n *Node) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	err := encoder.Encode(n.IPv4Address)
	if err != nil {
		return nil, err
	}

	err = encoder.Encode(n.IPv6Address)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// GobDecode gob interface
func (n *Node) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&n.IPv4Address)
	if err != nil {
		return err
	}
	return decoder.Decode(&n.IPv6Address)
}

// Listen enables the node to listen for other Pings from multicast sends
func (n *Node) Listen(callback ListenCallback) error {
	// Start monitoring for multicast
	if n.MulticastAddress == "" {
		return errors.New("must have multicast address")
	}

	// m := multicast.NewMulticast(n.MulticastAddress)
	m := &multicast.Multicast{
		Retries:      3,
		Timeout:      3,
		StopPingChan: make(chan struct{}),
		StopPongChan: make(chan struct{}),
	}
	n.multicast = m

	n.shutdownChan = make(chan struct{})

	respChan := make(chan multicast.Response)

	go n.multicast.Pong(n.MulticastAddress, respChan, n.ErrChan)

	go func() {
		for {
			select {
			case resp := <-respChan:
				fmt.Println("Received Ping from:", resp.SrcIP, "they said:", string(resp.Payload))

				buffer := bytes.NewBuffer(resp.Payload)
				n := new(Node)
				dec := gob.NewDecoder(buffer)
				err := dec.Decode(n)
				if err != nil {
					// fmt.Println("Node callback error:", err)
					n.ErrChan <- err
				}
				// fmt.Println(n, err)

				// Set the source address
				// n.SrcAddress = srcIP

				callback(n)
				return
			case <-n.shutdownChan:
				return
			}

		}
	}()

	return nil
}

// Ping starts the mulicast ping
func (n *Node) Ping(errChan chan error) {
	// Encode node to be sent via multicast
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(n)
	if err != nil {
		errChan <- err
	}

	n.multicast.Ping(n.MulticastAddress, buf.Bytes(), errChan)
}

// Shutdown stops the serving of multicast pings
func (n *Node) Shutdown() {
	close(n.shutdownChan)
	n.multicast.StopPong()
	n.multicast.StopPing()
	fmt.Println("shutdown the ping pong!")
}

// Notify pings other nodes using multicast to let them know it's here
// func (n *Node) Notify() {
// 	m := multicast.NewMulticast(n.MulticastAddress)
// 	n.multicast = m
// 	go n.multicast.Ping()
// }

// func (n *Node) callback(listenCallback ListenCallback) multicast.PongCallback {
// 	return func(payload []byte, srcIP net.IP) error {
// 		// // Let's make sure the ping is coming from a different interface than us
// 		// intfs, err := net.Interfaces()
// 		// if err != nil {
// 		// 	return err
// 		// }
// 		//
// 		// for _, intf := range intfs {
// 		// 	// If the interface is a loopback or doesn't have multicasting let's skip it
// 		// 	if strings.Contains(intf.Flags.String(), net.FlagLoopback.String()) || !strings.Contains(intf.Flags.String(), net.FlagMulticast.String()) {
// 		// 		continue
// 		// 	}
// 		//
// 		// 	// Now let's check if the interface has an ipv6 address
// 		// 	var addrs []net.Addr
// 		// 	addrs, err = intf.Addrs()
// 		// 	if err != nil {
// 		// 		continue
// 		// 	}
// 		//
// 		// 	for _, address := range addrs {
// 		// 		if ipnet, ok := address.(*net.IPNet); ok {
// 		// 			if ipnet.IP.To4() == nil {
// 		// 				if ipnet.IP.Equal(srcIP) {
// 		// 					if !n.RespondLocalPing {
// 		// 						return errors.New("src and node addresses are the same")
// 		// 					}
// 		// 				}
// 		// 			}
// 		// 		}
// 		// 	}
// 		// }
//
// 		// fmt.Println("payload?", payload)
// 		buffer := bytes.NewBuffer(payload)
// 		n := new(Node)
// 		dec := gob.NewDecoder(buffer)
// 		err := dec.Decode(n)
// 		if err != nil {
// 			// fmt.Println("Node callback error:", err)
// 			return err
// 		}
// 		// fmt.Println(n, err)
//
// 		// Set the source address
// 		// n.SrcAddress = srcIP
//
// 		listenCallback(n)
//
// 		return nil
// 	}
// }
