package disco

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"gitlab.fg/go/disco/multicast"
	"gitlab.fg/go/disco/node"
)

// Disco represents a list of discovered devices
type Disco struct {
	mu               sync.Mutex
	multicastAddress string
	members          []*node.Node
	// members        map[string]chan struct{}
	closeChan      chan struct{}   // Returns the monitorRegister goroutine
	discoveredChan chan *node.Node // node.Serve() sends nodes to this chan
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
// func (d *Disco) register(ctx context.Context, n *node.Node) error {
// 	d.mu.Lock()
// 	defer d.mu.Unlock()

// 	d.members = append(d.members, n)
// 	return nil
// }

// Deregister takes a node and deregisters it
func (d *Disco) deregister(n *node.Node) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Remove node from regsistered
	for i, m := range d.members {
		// make sure the node we sent matches
		if m == n {
			// remove it from the slice
			d.members = append(d.members[:i], d.members[i+1:]...)
		}
	}
}

// Members returns all nodes that are registered
// TODO update to return Nodes
func (d *Disco) Members() []*node.Node {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.members
}

// Discover listens for multicast sends
// TODO should discover automatically keep track of the members and have a callback
// that is called anytime the membership changes?
func (d *Disco) Discover(ctx context.Context) (<-chan *node.Node, error) {
	// respChan := make(chan multicast.Response)
	// errChan := make(chan error)
	results := make(chan *node.Node)

	m := &multicast.Multicast{Address: d.multicastAddress}
	respChan, err := m.Listen(ctx)
	if err != nil {
		return nil, err
	}

	if d.members == nil {
		d.members = []*node.Node{}
	}

	go func() {
		for {
			select {
			case resp := <-respChan:
				buffer := bytes.NewBuffer(resp.Payload)
				rn := &node.Node{}
				dec := gob.NewDecoder(buffer)
				dec.Decode(rn)
				rn.SrcIP = resp.SrcIP // set the source address
				if d.addToMembers(rn) {
					d.register(results, rn)
				} else {
					if index := d.indexOfMember(rn); index != -1 {
						d.mu.Lock()
						d.members[index].RegisterChan <- struct{}{}
						d.mu.Unlock()
					}
				}
			case <-ctx.Done():
				return
			}

		}
	}()

	return results, nil
}

func (d *Disco) register(results chan *node.Node, rn *node.Node) {
	fmt.Println("registering", rn)

	// If it's new to the members send it as a result
	rn.Action = node.RegisterAction
	rn.RegisterChan = make(chan struct{})

	d.mu.Lock()
	d.members = append(d.members, rn)
	d.mu.Unlock()

	results <- rn

	go func() {
		for {
			t := time.NewTimer(rn.SendInterval * 2)
			select {
			case <-rn.RegisterChan:
				t.Stop()
				continue
			case <-t.C:
				t.Stop()
				// Deregister if it times out
				rn.Action = node.DeregisterAction
				d.deregister(rn)
				fmt.Println("Registration timeout out for", rn, " deregistering")
				results <- rn
				return
			}
		}
	}()
}

// Check if the members slice already has the node if it doesn't add it
func (d *Disco) addToMembers(n *node.Node) bool {
	for _, m := range d.Members() {
		if m.Equal(n) {
			return false // node is already a member
		}
	}

	return true
}

func (d *Disco) indexOfMember(n *node.Node) int {
	for i, a := range d.Members() {
		if a.Equal(n) {
			return i
		}
	}
	return -1
}
