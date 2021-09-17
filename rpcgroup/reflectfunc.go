package rpcgroup

import (
	"encoding/gob"
	"log"
	"net/rpc"
	"reflect"
	"runtime"
)

var funcMap = make(map[string]reflect.Value)

func GetFunctionNameOrString(f interface{}) string {
	if reflect.ValueOf(f).Kind() == reflect.Func {
		return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	} else {
		return f.(string)
	}
}

// Register registers a function to call over network.
// Every function that is called by RPC must be registered before its call.
// Return value: registered function name
func Register(f interface{}) string {
	name := GetFunctionNameOrString(f)
	RegisterAs(name, f)
	return name
}

// Register registers a function using a name
func RegisterAs(name string, f interface{}) {
	if reflect.ValueOf(f).Kind() != reflect.Func {
		log.Panicf("In Register(%s, %v): Not a function", name, f)
	}
	if _, ok := funcMap[name]; ok {
		log.Panicf("In Register(%s, %v): The function is already registered", name, f)
	}
	funcMap[name] = reflect.ValueOf(f)
}

// Call a function named as "name" with argments params...
// Before calling this function, you need to first call Register.
func Call(name string, params ...interface{}) []interface{} {
	f, ok := funcMap[name]
	if !ok {
		log.Panicf("No such a function: %s", name)
	}
	argv := make([]reflect.Value, len(params))
	for i, param := range params {
		argv[i] = reflect.ValueOf(param)
	}
	resReflectValue := f.Call(argv)
	res := make([]interface{}, len(resReflectValue))
	for i := range resReflectValue {
		res[i] = resReflectValue[i].Interface()
	}
	return res
}

// GobRegister registers your struct If error "panic: gob: type not registered for interface: YOUR STRUCT" happens.
// (Example:  cluster.GobRegister(MyStruct{}))
func GobRegister(d interface{}) {
	gob.Register(d)
}

type CallArgs struct {
	Name string
	Arg  []interface{}
}

type FunctionCallRequest struct {
	CallArgs
	Channel chan *rpc.Call
}

// For net/rpc
type Dummy struct {
}

func (t *Dummy) Call(args *CallArgs, reply *[]interface{}) error {
	*reply = Call(args.Name, args.Arg...)
	return nil
}
