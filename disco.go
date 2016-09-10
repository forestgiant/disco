package disco

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"net"
	"sync"

	"gitlab.fg/go/disco/multicast"
	"gitlab.fg/go/disco/node"
)

// Disco represents a list of discovered devices
type Disco struct {
	mu               sync.Mutex
	multicastAddress string
	registered       []*node.Node
	closeChan        chan struct{}   // Returns the monitorRegister goroutine
	discoveredChan   chan *node.Node // node.Serve() sends nodes to this chan
}

// NewDisco setups and creates a *Disco yo
func NewDisco(multicastAddress string) (*Disco, error) {
	if multicastAddress == "" {
		return nil, errors.New("Address is blank")
	}

	ip, _, err := net.SplitHostPort(multicastAddress)
	if err != nil {
		return nil, err
	}

	if !net.ParseIP(ip).IsMulticast() {
		return nil, errors.New("multicastAddress is not valid")
	}

	d := new(Disco)
	d.multicastAddress = multicastAddress
	d.discoveredChan = make(chan *node.Node)

	return d, nil
}

// Register takes a node and registers it to be discovered
func (d *Disco) Register(ctx context.Context, n *node.Node) error {
	// d.mu.Lock()
	// defer d.mu.Unlock()
	// set multicast address for node
	// n.MulticastAddress = d.multicastAddress

	if err := n.Multicast(ctx, d.multicastAddress); err != nil {
		return err
	}

	d.registered = append(d.registered, n)
	return nil
}

// Deregister takes a node and deregisters it
func (d *Disco) Deregister(n *node.Node) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Remove node from regsistered
	for i, no := range d.registered {
		// make sure the node we sent matches
		if no == n {
			// remove it from the slice
			d.registered = append(d.registered[:i], d.registered[i+1:]...)
			n.Stop() // Stop the node multicasting
		}
	}
}

// GetRegistered returns all nodes that are registered
func (d *Disco) GetRegistered() []*node.Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.registered
}

// Discover uses multicast to find all other nodes that are registered
// func (d *Disco) Discover(ctx context.Context) (nodes <-chan *node.Node) {
// 	// Start sending pings from all the nodes registered
// 	// registeredNodes := d.GetRegistered()
// 	// for _, n := range registeredNodes {
// 	// 	fmt.Println("Start multicast for", n)
// 	// 	go n.Multicast()
// 	// }
// 	// Now that it's registered listen
// 	node.Listen(ctx, d.multicastAddress, d.discoveredChan)

// 	return d.discoveredChan
// }

// Discover listens for multicast sends
func (d *Disco) Discover(ctx context.Context) (<-chan *node.Node, error) {
	// respChan := make(chan multicast.Response)
	// errChan := make(chan error)
	results := make(chan *node.Node)

	m := &multicast.Multicast{Address: d.multicastAddress}
	respChan, err := m.Listen(ctx)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case resp := <-respChan:
				buffer := bytes.NewBuffer(resp.Payload)
				rn := &node.Node{}
				dec := gob.NewDecoder(buffer)
				dec.Decode(rn)

				// Set the source address
				rn.SrcIP = resp.SrcIP
				results <- rn
			case <-ctx.Done():
				return
			}

		}
	}()

	return results, nil
}
