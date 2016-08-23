package multicast

import (
	"context"
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
	Address string
	// Retries int32
	Delay time.Duration
}

// Response struct is sent over a channel when a pong is successful
type Response struct {
	Payload []byte
	SrcIP   net.IP
}

// ErrNoIPv6 error if no IPv6 interfaces were found
var ErrNoIPv6 = errors.New("Couldn't find any IPv6 network intefaces")

// Ping out to try to find others listening
func (m *Multicast) Ping(ctx context.Context, payload []byte, errc chan<- error) {
	gaddr, err := net.ResolveUDPAddr("udp6", m.Address)
	if err != nil {
		errc <- err
	}

	conn, err := net.ListenPacket("udp6", ":0")
	if err != nil {
		errc <- err
	}

	pconn := ipv6.NewPacketConn(conn)

	wcm := &ipv6.ControlMessage{
		HopLimit: 1,
	}

	bs := payload
	intfs, err := net.Interfaces()
	if err != nil {
		errc <- err
	}

	mu := sync.Mutex{}
	// pause is used to regulate the time between pings
	mu.Lock()
	pause := false
	mu.Unlock()

	for {
		select {
		default:
			// Track if any of the interfaces succesfully sent a message
			success := 0
			if pause {
				continue // If it's paused skip this
			}
			mu.Lock()
			pause := true
			mu.Unlock()

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

				fmt.Println("Sending Ping on interface:", intf.Name, intf.Flags)
				success++
			}

			if success <= 0 {
				errc <- ErrNoIPv6
			}

			time.AfterFunc(time.Second*m.Delay, func() {
				mu.Lock()
				defer mu.Unlock()
				pause = false
			})
		case <-ctx.Done():
			fmt.Println("ping done!!")
			return
		}
	}
}

// Pong when a multicast is received we serve it
func (m *Multicast) Pong(ctx context.Context, respChan chan<- Response, errc chan<- error) {
	gaddr, err := net.ResolveUDPAddr("udp6", m.Address)
	conn, err := net.ListenPacket("udp6", m.Address)
	if err != nil {
		errc <- err
	}

	intfs, err := net.Interfaces()
	if err != nil {
		errc <- err
	}

	pconn := ipv6.NewPacketConn(conn)
	joined := 0
	for _, intf := range intfs {
		// err := pconn.JoinGroup(&intf, &net.UDPAddr{IP: gaddr.IP})
		pconn.JoinGroup(&intf, &net.UDPAddr{IP: gaddr.IP})
		// if err != nil {
		// 	// fmt.Println("IPv6 join", intf.Name, "failed:", err)
		// 	// errChan <- err
		// }
		joined++
	}

	if joined == 0 {
		errc <- errors.New("no multicast interfaces available")
	}

	// if the context is done close the connection to stop the for loop from blocking
	go func() {
		select {
		case <-ctx.Done():
			pconn.Close()
			fmt.Println("close connection done!!")
			return
		}
	}()

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

			// check if b is a valid address format
			payload := b
			resp := Response{
				Payload: payload,
				SrcIP:   src.(*net.UDPAddr).IP,
			}

			respChan <- resp
		case <-ctx.Done():
			fmt.Println("pong done!!")
			return
		}
	}
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
