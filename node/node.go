package node

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/adler32"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/forestgiant/disco/multicast"
)

// Modes of a node being registered
const (
	RegisterAction   = iota
	DeregisterAction = iota
)

// Node represents a machine registered with Disco
type Node struct {
	// Values       Values
	Payload      []byte // max 256 bytes
	SrcIP        net.IP
	SendInterval time.Duration
	Action       int
	Heartbeat    *time.Timer
	Mutex        sync.Mutex
	ipv6         net.IP // set by localIPv4 function
	ipv4         net.IP // set by localIPv6 function
	mc           *multicast.Multicast
	mu           sync.Mutex // protect ipv4, ipv6, mc, SendInterval, registerCh
	registerCh   chan struct{}
	ticker       *time.Ticker
}

// Values stores any values passed to the node
type Values map[string]string

func (n *Node) init() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.registerCh == nil {
		n.registerCh = make(chan struct{})
	}
}

func (n *Node) String() string {
	return fmt.Sprintf("IPv4: %s, IPv6: %s, Action: %d, Payload: %s", n.ipv4, n.ipv6, n.Action, n.Payload)
}

// Equal compares nodes
func (n *Node) Equal(b *Node) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n != b {
		b.mu.Lock()
		defer b.mu.Unlock()
	}

	if !n.ipv4.Equal(b.ipv4) {
		return false
	}
	if !n.ipv6.Equal(b.ipv6) {
		return false
	}

	// Check if the payloads are the same
	if !bytes.Equal(n.Payload, b.Payload) {
		return false
	}

	return true
}

// Encode will convert a Node to a byte slice
// Checksum - 4 bytes
// IPv4 value - 4 bytes
// IPv6 value - 16 bytes
// Action value - 1 byte
// Send Interval - 8 bytes
// Payload (unknown length)
func (n *Node) Encode() []byte {
	payloadSize := len(n.Payload)
	buf := make([]byte, 33+payloadSize)

	// Add IPv4 to buffer
	index := 4 // after checksum
	for i, b := range n.IPv4().To4() {
		buf[index+i] = b
	}

	// Add IPv6 to buffer
	index = 8 // after ipv4
	for i, b := range n.IPv6() {
		buf[index+i] = b
	}

	// Add Action byte
	index = 24
	if n.Action == RegisterAction {
		buf[index] = 0
	} else {
		buf[index] = 1
	}

	// Add SendInterval int64
	index = 25
	for i := uint(0); i < 8; i++ {
		buf[index+int(i)] = byte(n.SendInterval >> (i * 8))
	}

	// Add Payload to buffer
	index = 33 // after SendInterval
	for i, b := range n.Payload {
		buf[index+i] = b
	}

	// Run a checksum on the existing buffer
	checksum := adler32.Checksum(buf[4:])

	// Add checksum to buffer
	for i := uint(0); i < 4; i++ {
		buf[int(i)] = byte(checksum >> (i * 8))
	}

	return buf
}

// DecodeNode decodes the bytes and returns a *Node struct
func DecodeNode(b []byte) (*Node, error) {
	// Verify checksum
	checksum := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	if checksum != adler32.Checksum(b[4:]) {
		return nil, errors.New("checksum didn't match")
	}

	// Get ipv4
	index := 4 // after checksum
	ipv4 := make(net.IP, net.IPv4len)
	for i := range ipv4 {
		ipv4[i] = b[index+i]
	}

	// Get ipv6
	index = 8 // after ipv4
	ipv6 := make(net.IP, net.IPv6len)
	for i := range ipv6 {
		ipv6[i] = b[index+i]
	}

	// If the ips returned are unspecified then return nil
	if ipv4.IsUnspecified() {
		ipv4 = nil
	}
	if ipv6.IsUnspecified() {
		ipv6 = nil
	}

	// Get Action type
	index = 24 // after ipv6
	action := RegisterAction
	if b[index] == 1 {
		action = DeregisterAction
	}

	// Decode SendInterval
	index = 25
	sendInterval := uint64(b[index+0]) | uint64(b[index+1])<<8 | uint64(b[index+2])<<16 | uint64(b[index+3])<<24 |
		uint64(b[index+4])<<32 | uint64(b[index+5])<<40 | uint64(b[index+6])<<48 | uint64(b[index+7])<<56

	// Get payload
	payload := b[33:]

	return &Node{ipv4: ipv4, ipv6: ipv6, Action: action, SendInterval: time.Duration(sendInterval), Payload: payload}, nil
}

// Done returns a channel that can be used to wait till Multicast is stopped
func (n *Node) Done() <-chan struct{} {
	return n.mc.Done()
}

// RegisterCh returns a channel to know if the node should stay registered
func (n *Node) RegisterCh() <-chan struct{} {
	n.init()
	return n.registerCh
}

// KeepRegistered sends an anonymous struct{} to registeredChan to indicate the node should stay registered
func (n *Node) KeepRegistered() {
	n.init()
	n.registerCh <- struct{}{}
}

// Multicast start the multicast ping
func (n *Node) Multicast(ctx context.Context, multicastAddress string) <-chan error {
	errCh := make(chan error, 1)

	n.mu.Lock()
	n.mc = &multicast.Multicast{Address: multicastAddress}
	n.ipv4 = localIPv4()
	n.ipv6 = localIPv6()

	// Stop existing ticker if it exists
	if n.ticker != nil {
		n.ticker.Stop()
	}

	if n.SendInterval.Seconds() == float64(0) {
		n.SendInterval = 1 * time.Second // default to 1 second
	}
	n.ticker = time.NewTicker(n.SendInterval)
	n.mu.Unlock()

	// Create send function
	send := func() error {
		// Check to see if IPv4 or 6 changed
		currentIPv4 := localIPv4()
		currentIPv6 := localIPv6()

		if !n.ipv4.Equal(currentIPv4) || !n.ipv6.Equal(currentIPv6) {
			// Multicast a deregister of previous node
			n.mu.Lock()
			n.Action = DeregisterAction
			n.mu.Unlock()
			if err := n.mc.Send(ctx, n.Encode()); err != nil {
				return err
			}
			n.mu.Lock()
			n.Action = RegisterAction
			n.ipv4 = currentIPv4
			n.ipv6 = currentIPv6
			n.mu.Unlock()
		}

		// Encode node to be sent via multicast
		if err := n.mc.Send(ctx, n.Encode()); err != nil {
			return err
		}

		return nil
	}

	go func() {
		for {
			select {
			case <-n.ticker.C:
				if err := send(); err != nil {
					errCh <- err
					continue
				}
			case <-n.Done():
				n.ticker.Stop()
				return
			}
		}
	}()

	// Call send right away
	send()

	return errCh
}

// Stop closes the StopCh to stop multicast sending
func (n *Node) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.ticker != nil {
		n.ticker.Stop()
	}
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
