package rpcgroup

import (
	"log"
	"net/rpc"
	"strings"
	"time"
)

type Client struct {
	// The connection destination.  e.g., "localhost:5000"
	TargetHost string

	// The number of times that try to reconnect.  if this is negative, retry connection forever.
	RetryCount int64

	rpc_client *rpc.Client

	callChannel chan *FunctionCallRequest
}

func NewClient(TargetHost string) *Client {
	c := new(Client)
	c.TargetHost = TargetHost
	c.RetryCount = -1
	c.callChannel = make(chan *FunctionCallRequest, 1000)
	go func() {
		c.serve()
	}()
	return c
}

// Connect connects to the server if the connection is not established yet.
func (c *Client) Connect() {
	retry := c.RetryCount
	for c.rpc_client == nil {
		client, err := rpc.DialHTTP("tcp", c.TargetHost)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				log.Println("connection refused: ", err)
				time.Sleep(1 * time.Second)
			} else {
				log.Println("rpcgroup: unknown error: ", err)
			}
			if retry == 0 {
				log.Fatal("DialHTTP failed: ", err)
			}
			retry -= 1
		} else {
			c.rpc_client = client
		}
	}
}

func (c *Client) serve() {
	for {
		// Do not connect until request comes
		callRequest := <-c.callChannel
		c.Connect()
		reply := new([]interface{})
		c.rpc_client.Go("Dummy.Call", callRequest.CallArgs, reply, callRequest.Channel)
	}
}

func (c *Client) Call(name string, params ...interface{}) []interface{} {
	channel := make(chan *rpc.Call, 1)
	c.callChannel <- &FunctionCallRequest{
		CallArgs: CallArgs{
			Name: name,
			Arg:  params,
		},
		Channel: channel,
	}
	callResponse := <-channel
	if callResponse.Error != nil {
		log.Fatal("rpc error:", callResponse.Error)
	}

	return *callResponse.Reply.(*[]interface{})
}
