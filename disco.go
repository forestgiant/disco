package disco

import (
	"fmt"

	"gitlab.fg/go/disco/multicast"
	"gitlab.fg/go/disco/node"
)

// Disco represents a list of discovered devices
type Disco struct {
	registered     []*node.Node
	writeNodeChan  chan *node.Node   // Writes a node to the register slice
	deleteNodeChan chan *node.Node   // Deletes a node to the register slice
	readNodesChan  chan struct{}     // Returns a slice of *Nodes in the register
	nodesChan      chan []*node.Node // Returns a slice of *Nodes in the register
	closeChan      chan struct{}     // Returns the monitorRegister goroutine
	discoveredChan chan *node.Node   // node.Serve() sends nodes to this chan
	// registeredChan chan []*node.Node
}

// NewDisco setups and creates a *Disco yo
func NewDisco() (*Disco, error) {
	d := new(Disco)
	d.closeChan = make(chan struct{})
	d.writeNodeChan = make(chan *node.Node)
	d.deleteNodeChan = make(chan *node.Node)
	d.readNodesChan = make(chan struct{})
	d.nodesChan = make(chan []*node.Node)
	d.discoveredChan = make(chan *node.Node)
	// d.registeredChan = make(chan []*node.Node)

	// goroutine to listen for register node changes
	go d.monitorRegister()

	return d, nil
}

func (d *Disco) monitorRegister() {
	for {
		select {
		case n := <-d.writeNodeChan:
			// Now that it's registered tell others about us
			n.Serve(func(n *node.Node) {
				fmt.Println("server being called?")
				d.discoveredChan <- n
			})

			// Add to registered slice
			d.registered = append(d.registered, n)

			// Tell the wait group we're finished with the registered nodes
		case n := <-d.deleteNodeChan:
			// Remove node from regsistered
			for i, no := range d.registered {
				// make sure the node we sent matches
				if no == n {
					// remove it from the slice
					d.registered = append(d.registered[:i], d.registered[i+1:]...)
				}
			}
		case <-d.readNodesChan:
			registered := make([]*node.Node, len(d.registered))
			copy(registered, d.registered)
			d.nodesChan <- registered
		case <-d.closeChan:
			fmt.Println("Cleanup!")
			return
		}
	}
}

// Register takes a node and registers it to be discovered
func (d *Disco) Register(n *node.Node) {
	// Send the node over the nodeChan with the action register
	d.writeNodeChan <- n
}

// Deregister takes a node and deregisters it
func (d *Disco) Deregister(n *node.Node) {
	// Send the node over the nodeChan with the action deregister
	d.deleteNodeChan <- n
}

// GetRegistered returns all nodes that are registered
func (d *Disco) GetRegistered() []*node.Node {
	// Write to the readNodesChan to update the nodes
	d.readNodesChan <- struct{}{}

	return <-d.nodesChan
}

// Discover uses multicast to find all other nodes that are registered
func (d *Disco) Discover(multicastAddress string) (<-chan *node.Node, <-chan error) {
	// n := new(node.Node)
	m := multicast.NewMulticast(multicastAddress)
	m.Ping()

	errc := make(chan error, 1)

	return d.discoveredChan, errc
}

// Stop closes the NodeChan to stop receiving nodes found
func (d *Disco) Stop() {
	close(d.closeChan)
}
