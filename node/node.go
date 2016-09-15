package node

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"gitlab.fg/go/disco/multicast"
)

// Modes of a node being registered
const (
	RegisterAction   = iota
	DeregisterAction = iota
)

// Node represents a machine registered with Disco
type Node struct {
	Values       Values
	SrcIP        net.IP
	SendInterval time.Duration
	Action       int
	ipv6         net.IP // TODO make this private and automatically set this
	ipv4         net.IP // TODO make this private and automatically set this
	mc           *multicast.Multicast
	mu           sync.Mutex // protect ipv4, ipv6, mc, SendInterval
	RegisterChan chan struct{}
}

// Values stores any values passed to the node
type Values map[string]string

func (n *Node) String() string {
	return fmt.Sprintf("IPv4: %s, IPv6: %s, Values: %s", n.ipv4, n.ipv6, n.Values)
}

// Equal compares nodes
func (n *Node) Equal(b *Node) bool {
	n.mu.Lock()
	b.mu.Lock()
	defer n.mu.Unlock()
	defer b.mu.Unlock()

	if !n.ipv4.Equal(b.ipv4) {
		return false
	}
	if !n.ipv6.Equal(b.ipv6) {
		return false
	}

	// if n.SendInterval != b.SendInterval {
	// 	return false
	// }

	// Check if the Values map is the same
	if len(n.Values) != len(b.Values) {
		return false
	}
	for k := range n.Values {
		v1 := n.Values[k]
		v2 := b.Values[k]
		if v1 != v2 {
			return false
		}
	}

	return true
}

// GobEncode gob interface
func (n *Node) GobEncode() ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)

	if err := encoder.Encode(n.SendInterval); err != nil {
		return nil, err
	}

	if err := encoder.Encode(n.ipv4); err != nil {
		return nil, err
	}

	if err := encoder.Encode(n.ipv6); err != nil {
		return nil, err
	}

	if err := encoder.Encode(n.Values); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// GobDecode gob interface
func (n *Node) GobDecode(buf []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)

	if err := decoder.Decode(&n.SendInterval); err != nil {
		return err
	}

	if err := decoder.Decode(&n.ipv4); err != nil {
		return err
	}

	if err := decoder.Decode(&n.ipv6); err != nil {
		return err
	}

	return decoder.Decode(&n.Values)
}

// Done returns a channel that can be used to wait till Multicast is stopped
func (n *Node) Done() <-chan struct{} {
	return n.mc.Done()
}

// Multicast start the mulicast ping
func (n *Node) Multicast(ctx context.Context, multicastAddress string) error {
	n.mu.Lock()
	n.ipv4 = localIPv4()
	n.ipv6 = localIPv6()

	if n.SendInterval.Seconds() == float64(0) {
		n.SendInterval = 1 * time.Second // default to 1 second
	}

	n.mu.Unlock()

	// Encode node to be sent via multicast
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(n)
	if err != nil {
		return err
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.mc = &multicast.Multicast{Address: multicastAddress}
	if err := n.mc.Send(ctx, n.SendInterval, buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// Stop closes the StopCh to stop multicast sending
func (n *Node) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.mc.Stop()
}

// IPv4 getter for ipv4Address
func (n *Node) IPv4() net.IP {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.ipv4
}

// IPv6 getter for ipv6Address
func (n *Node) IPv6() net.IP {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.ipv6
}

// localIPv4 return the ipv4 address of the computer
// If it can't get the local ip it returns 127.0.0.1
// https://github.com/forestgiant/netutil
func localIPv4() net.IP {
	loopback := net.ParseIP("127.0.0.1")

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return loopback
	}

	for _, addr := range addrs {
		// check the address type and make sure it's not loopback
		if ipnet, ok := addr.(*net.IPNet); ok {
			if !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.To4()
				}
			}
		}
	}

	return loopback
}

// localIPv6 return the ipv6 address of the computer
// If it can't get the local ip it returns net.IPv6loopback
// https://github.com/forestgiant/netutil
func localIPv6() net.IP {
	loopback := net.IPv6loopback

	intfs, err := net.Interfaces()
	if err != nil {
		return loopback
	}

	for _, intf := range intfs {
		// If the interface is a loopback or doesn't have multicasting let's skip it
		if strings.Contains(intf.Flags.String(), net.FlagLoopback.String()) || !strings.Contains(intf.Flags.String(), net.FlagMulticast.String()) {
			continue
		}

		// Now let's check if the interface has an ipv6 address
		addrs, err := intf.Addrs()
		if err != nil {
			continue
		}

		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok {
				if !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() == nil {
						return ipnet.IP
					}
				}
			}
		}
	}

	return loopback
}
