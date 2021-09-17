package rpcgroup

import (
	"log"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

func Add(a int, b int) int {
	return a + b
}

type Counter struct {
	sync.Mutex
	value int
}

var counter Counter

func AddToCounter(value int) {
	counter.Lock()
	defer counter.Unlock()
	counter.value += value
}

var AddName = Register(Add)
var AddToCounterName = Register(AddToCounter)

var NoNameFunc = Register(func() int { return 10 })

func TestNoNameFunc(t *testing.T) {
	log.Println(NoNameFunc)
	if Call(NoNameFunc)[0].(int) != 10 {
		t.Fatal("unexpected")
	}
}

func TestCall(t *testing.T) {
	if Call(AddName, 10, 21)[0].(int) != 31 {
		t.Fatal("unexpected")
	}
}

func TestServerAndClient(t *testing.T) {
	Listen(12345)
	c := NewClient("localhost:12345")
	if c.Call(AddName, 10, 21)[0].(int) != 31 {
		t.Fatal("unexpected")
	}
}

func TestGroup(t *testing.T) {
	counter.value = 0
	group1 := New(12340, "localhost:12340", "localhost:12341")
	group2 := New(12341, "localhost:12340", "localhost:12341")
	group1.Call(AddToCounterName, 3)
	if counter.value != 6 {
		t.Fatal("unexpected")
	}
	group2.Call(AddToCounter, 10)
	if counter.value != 26 {
		t.Fatal("unexpected")
	}
}

func BenchmarkCallAll(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	port1 := 20000 + rand.Intn(10000)
	port2 := 20000 + rand.Intn(10000)
	group1 := New(port1, "localhost:"+strconv.Itoa(port1), "localhost:"+strconv.Itoa(port2))
	for _, c := range group1.Clients {
		c.RetryCount = 3
	}
	group2 := New(port2, "localhost:"+strconv.Itoa(port1), "localhost:"+strconv.Itoa(port2))
	for _, c := range group2.Clients {
		c.RetryCount = 3
	}
	if group1.Call(Add, 10, 21)[0][0].(int) != 31 {
		b.Fatal("unexpected")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := rand.Intn(100)
		y := rand.Intn(100)
		group1.Call(Add, x, y)
		group2.Call(Add, x, y)
	}
}

func TestGroup_Subgroup(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	port1 := 20000 + rand.Intn(10000)
	port2 := 20000 + rand.Intn(10000)
	group1 := New(port1, "localhost:"+strconv.Itoa(port1), "localhost:"+strconv.Itoa(port2))
	group2 := New(port2, "localhost:"+strconv.Itoa(port1), "localhost:"+strconv.Itoa(port2))
	for _, c := range group1.Clients {
		c.RetryCount = 3
	}
	for _, c := range group2.Clients {
		c.RetryCount = 3
	}

	counter.value = 0
	group1.Call(AddToCounter, 1)
	if counter.value != 2 {
		t.Fatal("unexpected")
	}
	counter.value = 0
	group1.Subgroup([]int{0}).Call(AddToCounter, 1)
	if counter.value != 1 {
		t.Fatal("unexpected")
	}
}
