/*
Example:
	var group = rpcgroup.New(5000, "app1:5000", "app2:5000")
	func init() {
		id := "0001"
		group.Call(InitializeFunction, id)
	}
	var InitializeFunction = rpcgroup.Register(func(id string) {
		StartLogger(id)
	})
*/
package rpcgroup

import (
	"fmt"
	"log"
	"sync"
)

type Group struct {
	MyHost  string
	Clients []*Client
}

// New is a constructor of Group.
// hosts must be specified by the "host:port" form.
func New(listenPort int, hosts ...string) *Group {
	c := &Group{
		MyHost:  fmt.Sprintf("%s:%d", Hostname(), listenPort),
		Clients: []*Client{},
	}
	Listen(listenPort)

	for _, host := range hosts {
		c.Clients = append(c.Clients, NewClient(host))
	}
	log_output := ""
	for i, host := range hosts {
		if i > 0 {
			log_output += ", "
		}
		log_output += host
		if host == c.MyHost {
			log_output += " (myself)"
		}
	}
	log.Println("rpcgroup connected to ", log_output)
	return c
}

// GroupWithoutListen is a constructor of Group that does not listen any port.
// It groups together all the hosts.
func GroupWithoutListen(hosts ...string) *Group {
	c := &Group{
		MyHost:  "",
		Clients: []*Client{},
	}
	for _, host := range hosts {
		c.Clients = append(c.Clients, NewClient(host))
	}
	return c
}

// Subgroup returns a sub-group of Group.
func (c *Group) Subgroup(indices []int) *Group {
	d := &Group{
		MyHost:  c.MyHost,
		Clients: []*Client{},
	}

	for _, i := range indices {
		d.Clients = append(d.Clients, c.Clients[i])
	}
	return d
}

// Client returns the id'th client (0-indexed)
func (c *Group) Client(id int) *Client {
	return c.Clients[id]
}

func (c *Group) Call(f interface{}, params ...interface{}) [][]interface{} {
	name := GetFunctionNameOrString(f)
	return c.call(name, params...)
}

func (c *Group) call(name string, params ...interface{}) [][]interface{} {
	var wg sync.WaitGroup
	var results = make([][]interface{}, len(c.Clients))
	wg.Add(len(c.Clients))
	for id, client := range c.Clients {
		go func(id int, client *Client) {
			if client.TargetHost == c.MyHost {
				// If the destination is the same host, do not use RPC; instead, just call the function.
				Call(name, params...)
			} else {
				results[id] = client.Call(name, params...)
			}
			wg.Done()
		}(id, client)
	}
	wg.Wait()
	return results
}
