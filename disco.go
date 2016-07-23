package disco

import (
	"fmt"

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
			err := n.Listen(func(n *node.Node) {
				fmt.Println("serve being called?", n)
				d.discoveredChan <- n
			})

			if err != nil {
				fmt.Println("node serve error", err)
			}

			fmt.Println("where?", n)

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

					// Make sure the node isn't Serving multicast pings any more
					n.Shutdown()
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
	// for n := range nodes {
	d.writeNodeChan <- n
	// }
}

// Deregister takes a node and deregisters it
func (d *Disco) Deregister(n *node.Node) {
	// Send the node over the nodeChan with the action deregister
	// for n := range nodes {
	d.deleteNodeChan <- n
	// }
}

// GetRegistered returns all nodes that are registered
func (d *Disco) GetRegistered() []*node.Node {
	// Write to the readNodesChan to update the nodes
	d.readNodesChan <- struct{}{}

	return <-d.nodesChan
}

// Discover uses multicast to find all other nodes that are registered
func (d *Disco) Discover(multicastAddress string, errChan chan error) (nodes <-chan *node.Node) {
	// nodeChan := make(chan *node.Node)

	// go func() {
	// 	for {
	// 		select {
	// 		case node := <-d.discoveredChan:
	// 			fmt.Println("Discover: found node?", node)
	// 			// nodeChan <- node
	// 		case <-d.closeChan:
	// 		}
	// 	}
	// }()

	// Start sending pings from all the nodes registered
	registeredNodes := d.GetRegistered()
	for _, n := range registeredNodes {
		go n.Ping(errChan)
	}

	// errc := make(chan error, 1)

	return d.discoveredChan
	// return d.discoveredChan, errc
}

// Stop closes the NodeChan to stop receiving nodes found
func (d *Disco) Stop() {
	close(d.closeChan)
}
