package disco

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/forestgiant/disco/multicast"
	"github.com/forestgiant/disco/node"
)

// Disco represents a list of discovered devices
type Disco struct {
	mu             sync.Mutex      // protects members
	members        []*node.Node    // stores all nodes registered
	discoveredChan chan *node.Node // node.Serve() sends nodes to this chan
}

// Members returns all node in members slice
func (d *Disco) Members() []*node.Node {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.members
}

// Discover listens for multicast sends and registers any nodes it finds
func (d *Disco) Discover(ctx context.Context, multicastAddress string) (<-chan *node.Node, error) {
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

	if d.discoveredChan == nil {
		d.discoveredChan = make(chan *node.Node)
	}

	results := make(chan *node.Node)

	m := &multicast.Multicast{Address: multicastAddress}
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
				rn, err := node.DecodeNode(resp.Payload)
				if err != nil {
					continue
				}

				// If we receive a node registration then register or keep alive
				if rn.Action == node.RegisterAction {
					rn.SrcIP = resp.SrcIP // set the source address
					if d.addToMembers(rn) {
						d.register(results, rn)
					} else {
						if index := d.indexOfMember(rn); index != -1 {
							d.mu.Lock()
							d.members[index].KeepRegistered()
							d.mu.Unlock()
						}
					}
				} else {
					// If we receive an explicit deregister multicast remove it and send to results
					if d.deregister(rn) {
						results <- rn
					}
				}
			case <-ctx.Done():
				return
			}

		}
	}()

	return results, nil
}

// register adds newly discovered nodes to the d.members slice and sends the node
// over the result chan. Then it creates a new goroutine for each node that checks
// if it can read on it's registerCh. If it can't within rn.SendInterval * 3 it deregisters
func (d *Disco) register(results chan *node.Node, rn *node.Node) {
	// If it's new to the members send it as a result
	rn.Action = node.RegisterAction

	d.mu.Lock()
	d.members = append(d.members, rn)
	d.mu.Unlock()

	go func() {
		for {
			rn.Mutex.Lock()
			rn.Heartbeat = time.NewTimer(rn.SendInterval * 3)
			rn.Mutex.Unlock()

			select {
			case <-rn.RegisterCh():
				rn.Heartbeat.Stop()
				continue
			case <-rn.Heartbeat.C:
				rn.Heartbeat.Stop()
				// Deregister if it times out
				rn.Action = node.DeregisterAction
				d.deregister(rn)
				results <- rn
				return
			}
		}
	}()

	results <- rn
}

// deregister takes a node and removes it from the d.members slice
func (d *Disco) deregister(n *node.Node) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove node from regsistered
	for i := len(d.members) - 1; i >= 0; i-- {
		m := d.members[i]
		// make sure the node we sent matches
		if m.Equal(n) {
			// stop heartbeat and remove from member slice
			m.Mutex.Lock()
			m.Heartbeat.Stop()
			m.Mutex.Unlock()
			d.members = append(d.members[:i], d.members[i+1:]...)

			return true
		}
	}

	return false
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

// indexOfMember checks if a node is in the d.members slice
// and returns it's index, if it isn't there it returns -1
func (d *Disco) indexOfMember(n *node.Node) int {
	for i, a := range d.Members() {
		if a.Equal(n) {
			return i
		}
	}
	return -1
}
