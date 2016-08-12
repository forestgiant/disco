package multicast

import (
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"golang.org/x/net/ipv6"
)

// Multicast struct
type Multicast struct {
	Address string
	// Payload      []byte
	Retries      int32
	Timeout      time.Duration
	StopPongChan chan struct{}
	StopPingChan chan struct{}
	// pingCount    *int32
}

// Response struct is sent over a channel when a pong is successful
type Response struct {
	Payload []byte
	SrcIP   net.IP
}

// ErrNoIPv6 error if no IPv6 interfaces were found
var ErrNoIPv6 = errors.New("Couldn't find any IPv6 network intefaces")

// PongCallback is passed to the Pong function
// type PongCallback func(payload []byte, srcIP net.IP) error

// NewMulticast return a Multicast struct and create the wg and shutdownChan
// func NewMulticast(address string) *Multicast {
// 	m := new(Multicast)
// 	m.Address = address
// 	m.Retries = 3
// 	m.Timeout = 3
// 	// m.pingCount = new(int32)
// 	m.stopPingChan = make(chan struct{})
// 	m.stopPongChan = make(chan struct{})
//
// 	return m
// }

// Ping out to try to find others listening
func (m *Multicast) Ping(payload []byte, errc chan<- error) {
	gaddr, err := net.ResolveUDPAddr("udp6", m.Address)
	if err != nil {
		// fmt.Println(err)
		errc <- err
	}

	conn, err := net.ListenPacket("udp6", ":0")
	if err != nil {
		// fmt.Println(err)
		errc <- err
	}

	pconn := ipv6.NewPacketConn(conn)

	wcm := &ipv6.ControlMessage{
		HopLimit: 1,
	}

	bs := payload
	// for bs := range w.inbox {
	intfs, err := net.Interfaces()
	if err != nil {
		// fmt.Println(err)
		errc <- err
	}

	for {
		select {
		default:
			// Keep trying every three seconds until shutdownChan is closed or
			// we tried 3 times
			// if atomic.LoadInt32(m.pingCount) >= m.Retries {
			// 	fmt.Println("Warning: Ping timed out without a response. Please verify your network interfaces have IPv6 support.")
			// 	// return
			// }

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
					// fmt.Println(err)
					continue
				}

				log.Println("Sending Ping on interface:", intf.Name, intf.Flags)
				// fmt.Printf("sent %d bytes to %v on %s \n", len(bs), gaddr, intf.Name)
				success++
			}

			if success <= 0 {
				// fmt.Println("Couldn't find any IPv6 network intefaces")
				errc <- ErrNoIPv6
			}

			// fmt.Printf("Tried %d interfaces \n", success)

			// Let's wait 3 seconds before pinging again
			// atomic.AddInt32(m.pingCount, 1)

			time.Sleep(time.Second * m.Timeout)
		case <-m.StopPingChan:
			return
		}
	}
}

// Pong when a multicast is received we serve it
func (m *Multicast) Pong(respChan chan<- Response, errc chan<- error) {
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
			log.Printf("recv %d bytes from %s \n", n, src)

			// Received a ping. Decrement ping count
			// atomic.AddInt32(m.pingCount, 1)

			// check if b is a valid address format
			payload := b
			resp := Response{
				Payload: payload,
				SrcIP:   src.(*net.UDPAddr).IP,
			}

			respChan <- resp
			// err = callback(payload, src.(*net.UDPAddr).IP)
			// if err != nil {
			// 	// fmt.Println(err)
			// 	// errChan <- err
			// 	continue
			// }
		case <-m.StopPongChan:
			return
		}
	}
}

// StopPing returns out of ping
func (m *Multicast) StopPing() {
	close(m.StopPingChan)
}

// StopPong returns out of ping
func (m *Multicast) StopPong() {
	close(m.StopPongChan)
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
