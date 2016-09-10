package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.fg/go/disco"
	"gitlab.fg/go/disco/node"
)

// Discover other nodes with the disco package via multicast
// This creates a simple membership list of nodes
func main() {
	members := []*node.Node{}
	d, err := disco.NewDisco("[ff12::9000]:21099")
	ctx, cancelFunc := context.WithCancel(context.Background())

	// Save any discovered nodes to a member slice
	discoveredChan, err := d.Discover(ctx)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case n := <-discoveredChan:
				members = addToMemberlist(members, n)
				printMembers(members)
			}
		}
	}()

	// Get a unique address
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	// Register ourselve as a node
	n := &node.Node{Values: map[string]string{"Address": ln.Addr().String()}, SendInterval: 2 * time.Second}
	if err != nil {
		log.Fatal(err)
	}
	d.Register(ctx, n)

	// Listen for shutdown signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigs:
			cancelFunc()
		}
	}()

	// Select will block until a result comes in
	select {
	case <-ctx.Done():
		fmt.Println("Closing membership")
		return
		// os.Exit(0)
	}

}

// Check if the members slice already has the node if it doesn't add it
func addToMemberlist(members []*node.Node, n *node.Node) []*node.Node {
	for _, m := range members {
		if m.Equal(n) {
			return members // node is already a member
		}
	}

	return append(members, n)
}

// printMembers prints out information about the members found
func printMembers(members []*node.Node) {
	fmt.Printf("Updating members every 2 seconds. Currently found %d members. They are: \n", len(members))
	for _, m := range members {
		fmt.Println(m.Values["Address"])
	}
}
