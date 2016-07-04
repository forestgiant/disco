package multicast

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/ipv6"
)

// Multicast struct
type Multicast struct {
	sync.Mutex
	Address      string
	Payload      []byte
	Retries      int
	Timeout      time.Duration
	stopPongChan chan struct{}
	stopPingChan chan struct{}
	pingCount    int
}

// PongCallback is passed to the Pong function
type PongCallback func(payload []byte, srcIP net.IP) error

// NewMulticast return a Multicast struct and create the wg and shutdownChan
func NewMulticast(address string) *Multicast {
	m := new(Multicast)
	m.Address = address
	m.Retries = 3
	m.Timeout = 3
	m.stopPingChan = make(chan struct{})
	m.stopPongChan = make(chan struct{})

	return m
}

// Ping out to try to find others listening
func (m *Multicast) Ping() {
	gaddr, err := net.ResolveUDPAddr("udp6", m.Address)
	if err != nil {
		fmt.Println(err)
		return
	}

	conn, err := net.ListenPacket("udp6", ":0")
	if err != nil {
		fmt.Println(err)
		return
	}

	pconn := ipv6.NewPacketConn(conn)

	wcm := &ipv6.ControlMessage{
		HopLimit: 1,
	}

	bs := m.Payload
	// for bs := range w.inbox {
	intfs, err := net.Interfaces()
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		select {
		default:
			// Keep trying every three seconds until shutdownChan is closed or
			// we tried 3 times
			if m.pingCount >= m.Retries {
				fmt.Println("Warning: Ping timed out without a response. Please verify your network interfaces have IPv6 support.")
				// return
			}

			// Track if any of the interfaces succesfully sent a message
			success := 0

			// Loop through all the interfaes
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

				if !containsIPv6(addrs) {
					continue
				}

				wcm.IfIndex = intf.Index
				pconn.SetWriteDeadline(time.Now().Add(time.Second))
				_, err = pconn.WriteTo(bs, wcm, gaddr)
				pconn.SetWriteDeadline(time.Time{})

				if err != nil {
					fmt.Println(err)
					continue
				}

				fmt.Println("Sending Ping on interface:", intf.Name, intf.Flags)
				// fmt.Printf("sent %d bytes to %v on %s \n", len(bs), gaddr, intf.Name)
				success++
			}

			if success <= 0 {
				fmt.Println("Couldn't find any IPv6 network intefaces")
				return
			}

			// fmt.Printf("Tried %d interfaces \n", success)

			// Let's wait 3 seconds before pinging again
			m.Lock()
			m.pingCount++
			m.Unlock()

			time.Sleep(time.Second * m.Timeout)
		case <-m.stopPingChan:
			return
		}
	}
}

// Pong when a multicast is received we serve it
func (m *Multicast) Pong(callback PongCallback, errc chan<- error) {
	gaddr, err := net.ResolveUDPAddr("udp6", m.Address)
	conn, err := net.ListenPacket("udp6", m.Address)
	if err != nil {
		// fmt.Println(err)
		errc <- err
	}

	intfs, err := net.Interfaces()
	if err != nil {
		// fmt.Println(err)
		errc <- err
	}

	pconn := ipv6.NewPacketConn(conn)
	joined := 0
	for _, intf := range intfs {
		err := pconn.JoinGroup(&intf, &net.UDPAddr{IP: gaddr.IP})
		if err != nil {
			// fmt.Println("IPv6 join", intf.Name, "failed:", err)
			// errChan <- err
		}
		// else {
		// 	fmt.Println("IPv6 join", intf.Name, "success")
		// }
		joined++
	}

	if joined == 0 {
		// fmt.Println("no multicast interfaces available")
		errc <- errors.New("no multicast interfaces available")
	}

	buf := make([]byte, 65536)
	for {
		select {
		default:
			n, _, src, err := pconn.ReadFrom(buf)
			if err != nil {
				// fmt.Println(err)
				// errChan <- err
				continue
			}

			// make a copy because we will overwrite buf
			b := make([]byte, n)
			copy(b, buf)

			// fmt.Printf("recv %d bytes from %s, message? %s \n", n, src, b)
			fmt.Printf("recv %d bytes from %s \n", n, src)

			// Received a ping. Decrement ping count
			m.Lock()
			m.pingCount--
			m.Unlock()

			// check if b is a valid address format
			payload := b
			err = callback(payload, src.(*net.UDPAddr).IP)
			if err != nil {
				// fmt.Println(err)
				// errChan <- err
				continue
			}
		case <-m.stopPingChan:
			return
		}
	}
}

// StopPing returns out of ping
func (m *Multicast) StopPing() {
	close(m.stopPingChan)
}

// StopPong returns out of ping
func (m *Multicast) StopPong() {
	close(m.stopPongChan)
}

// containsIPv6 checks to see if any net.Addr has an IPv6 address
func containsIPv6(addrs []net.Addr) bool {
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok {
			if !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() == nil {
					return true
				}
			}
		}
	}

	return false
}
