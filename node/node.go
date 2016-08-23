package node

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"net"

	"gitlab.fg/go/disco/multicast"
)

// Modes of a node being registered
// const (
// 	RegisterAction   = iota
// 	DeregisterAction = iota
// )

// ListenCallback called when a listener is pinged
// type ListenCallback func(rn *Node) error

// Node represents a machine registered with Disco
type Node struct {
	IPv6Address  string
	IPv4Address  string
	SrcIP        net.IP
	ErrChan      chan error
	shutdownChan chan struct{}
	multicast    *multicast.Multicast

	// RespondLocalPing bool // Used for testing
}

// Equal compares nodes
func Equal(a, b *Node) bool {
	if a.IPv4Address != b.IPv4Address {
		return false
	}

	if a.IPv6Address != b.IPv6Address {
		return false
	}

	return true
}

// Listen enables the node to listen for other Pings from multicast sends
func Listen(ctx context.Context, multicastAddress string, results chan<- *Node) error {
	// Start monitoring for multicast
	if multicastAddress == "" {
		return errors.New("must have multicast address")
	}

	m := &multicast.Multicast{
		Address: multicastAddress,
		Delay:   3,
	}

	respChan := make(chan multicast.Response)
	errChan := make(chan error)

	go m.Pong(ctx, respChan, errChan)

	go func() {
		for {
			select {
			case resp := <-respChan:
				buffer := bytes.NewBuffer(resp.Payload)
				rn := new(Node)
				dec := gob.NewDecoder(buffer)
				err := dec.Decode(rn)
				if err != nil {
					errChan <- err
				}

				// Set the source address
				rn.SrcIP = resp.SrcIP
				results <- rn
			case <-ctx.Done():
				return
			}

		}
	}()

	return nil
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

// Multicast start the mulicast ping
func (n *Node) Multicast(ctx context.Context, multicastAddress string) error {
	m := &multicast.Multicast{
		Address: multicastAddress,
		Delay:   3,
	}

	// Encode node to be sent via multicast
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(n)
	if err != nil {
		return err
	}

	go m.Ping(ctx, buf.Bytes(), n.ErrChan)

	return nil
}

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
