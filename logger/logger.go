/* INSTRUCTION:
Put the following code in initialize()
	logid := GetNextLogID()
	StartLogger(logid)


It is possible to add user defined regions for trace by adding:
	defer trace.StartRegion(context.Background(), regionName).End()
For example, in order to add it to every handler of http, one can write a wrapper as follows:
	handleFunc := func(p goji.Pattern, h func(http.ResponseWriter, *http.Request)) {
		regionName := GetFunctionName(h)
		mux.HandleFunc(p, func(a http.ResponseWriter, b *http.Request) {
			defer trace.StartRegion(context.Background(), regionName).End()
			h(a, b)
		})
	}
If you are using echo, the following code works:
	func TraceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer trace.StartRegion(c.Request().Context(), c.Request().Method + " " + c.Path()).End()
			return next(c)
		}
	}
	func main() {
		e := echo.New()
		// ...
		e.Use(TraceMiddleware)
	}
*/
package logger

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/valyala/fasttemplate"
)

var startLoggerToken = make(chan bool, 1)
var stopLoggerToken = make(chan bool, 1)

var LoggerBashScript = "/usr/bin/logger.sh"
var LogFilePath = "/tmp/isucon/"

var UseTrace = false

func init() {
	startLoggerToken <- true
	os.MkdirAll(LogFilePath, os.ModePerm)
}

func ExecuteCommand(bashscript string) (string, error) {
	cmd := exec.Command("/bin/bash", "-s")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	go func() {
		stdin.Write([]byte(bashscript))
		stdin.Close()
	}()
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return string(stdoutStderr), err
	}
	return string(stdoutStderr), nil
}
func MustExecuteCommand(bashscript string) string {
	res, err := ExecuteCommand(bashscript)
	if err != nil {
		log.Fatalf("Error while executing %s: %s", bashscript, res)
	}
	return res
}
func GetNextLogID() string {
	res := MustExecuteCommand(LoggerBashScript + " nextid")
	return res
}

func GetFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func StartLogger(id string, BenchmarkTime int) {
	// try to stop and wait until we get token
L:
	for {
		select {
		case <-startLoggerToken:
			break L
		case stopLoggerToken <- true:
		}
	}
	// clear stop token
	select {
	case <-stopLoggerToken:
	default:
	}

	// start logger
	log.Print(MustExecuteCommand(LoggerBashScript + " start " + id))
	f, err := os.Create(filepath.Join(LogFilePath, "cpu.prof"))
	if err != nil {
		panic(err)
	}
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)
	pprof.StartCPUProfile(f)
	var f_trace *os.File
	if UseTrace {
		f_trace, err = os.Create(filepath.Join(LogFilePath, "trace.prof"))
		if err != nil {
			panic(err)
		}
		trace.Start(f_trace)
	}

	log.Println("Started logger")

	// stop logger after 60 sec or stop logger token is placed
	go func(id string) {
		terminated := false
		select {
		case <-stopLoggerToken:
			terminated = true
		case <-time.After(time.Second * time.Duration(BenchmarkTime)):
		}

		pprof.StopCPUProfile()
		err := f.Close()
		if err != nil {
			panic(err)
		}
		// dump other profiles
		runtime.GC()
		profile_list := []string{"goroutine", "heap", "threadcreate", "block", "mutex"} // "allocs"
		for _, s := range profile_list {
			file, err := os.Create(filepath.Join(LogFilePath, s+".prof"))
			if err != nil {
				panic(err)
			}
			pprof.Lookup(s).WriteTo(file, 0)
			file.Close()
		}
		if UseTrace {
			trace.Stop()
			if err := f_trace.Close(); err != nil {
				panic(err)
			}
		}

		if terminated {
			res, err := ExecuteCommand(LoggerBashScript + " term " + id)
			log.Println(res)
			if err != nil {
				log.Println(err)
			}
		} else {
			log.Print(MustExecuteCommand(LoggerBashScript + " stop " + id))
		}
		log.Println("Stopped logger")
		startLoggerToken <- true
	}(id)
}

type AlpTrace struct {
	template *fasttemplate.Template
	pool     *sync.Pool
	logfile  *os.File
}

// New creates AlpTrace
func New() *AlpTrace {
	os.MkdirAll(LogFilePath, os.ModePerm)
	format := `{"time":"${time}","method":"${method}","uri":"${uri}","status":${status},"response_time":${response_time},"body_bytes":${body_bytes}}` + "\n"
	filename := LogFilePath + "logalp.txt"
	logfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	return &AlpTrace{
		template: fasttemplate.New(format, "${", "}"),
		pool: &sync.Pool{New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		}},
		logfile: logfile,
	}
}

type AlpTraceRegion struct {
	parent *AlpTrace
	start  time.Time
}

func (c *AlpTrace) Start() *AlpTraceRegion {
	region := AlpTraceRegion{
		parent: c,
		start:  time.Now(),
	}
	return &region
}
func (r *AlpTraceRegion) Stop(method string, uri string, status int, body_bytes int64) {
	stop := time.Now()
	buf := r.parent.pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer r.parent.pool.Put(buf)

	if _, err := r.parent.template.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
		switch tag {
		case "time":
			return buf.WriteString(time.Now().Format(time.RFC3339Nano))
		case "uri":
			return buf.WriteString(uri)
		case "method":
			return buf.WriteString(method)
		case "status":
			return buf.WriteString(strconv.FormatInt(int64(status), 10))
		case "response_time":
			return buf.WriteString(strconv.FormatFloat(stop.Sub(r.start).Seconds(), 'f', 9, 64))
		case "body_bytes":
			return buf.WriteString(strconv.FormatInt(body_bytes, 10))
		}
		return 0, nil
	}); err != nil {
		panic(err)
	}

	if _, err := r.parent.logfile.Write(buf.Bytes()); err != nil {
		panic(err)
	}
}

var alptrace = New()

func AlpMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		t := alptrace.Start()
		err := next(c)
		t.Stop(c.Request().Method, c.Path(), c.Response().Status, c.Response().Size)
		return err
	}
}
