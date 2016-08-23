package disco

import (
	"context"
	"sync"

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
	d := new(Disco)
	d.multicastAddress = multicastAddress
	d.discoveredChan = make(chan *node.Node)

	return d, nil
}

// Register takes a node and registers it to be discovered
func (d *Disco) Register(ctx context.Context, n *node.Node) {
	// d.mu.Lock()
	// defer d.mu.Unlock()
	// set multicast address for node
	// n.MulticastAddress = d.multicastAddress
	n.Multicast(ctx, d.multicastAddress)

	// d.registered = append(d.registered, n)
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
func (d *Disco) Discover(ctx context.Context) (nodes <-chan *node.Node) {
	// Start sending pings from all the nodes registered
	// registeredNodes := d.GetRegistered()
	// for _, n := range registeredNodes {
	// 	fmt.Println("Start multicast for", n)
	// 	go n.Multicast()
	// }
	// Now that it's registered listen
	node.Listen(ctx, d.multicastAddress, d.discoveredChan)

	return d.discoveredChan
}
