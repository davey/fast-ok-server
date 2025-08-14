package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	totalRequests uint64
	totalBytes    uint64
	startTime     = time.Now()
)

type hostStats struct {
	requests uint64
	bytes    uint64
}

type methodStats struct {
	get   uint64
	post  uint64
	other uint64
}

var (
	hostMap sync.Map
	methods methodStats
)

func main() {
	addr := flag.String("addr", ":8080", "TCP address to listen on")
	statsEvery := flag.Duration("stats", 2*time.Second, "How often to print stats")
	readTimeout := flag.Duration("read-timeout", 5*time.Second, "Read timeout")
	writeTimeout := flag.Duration("write-timeout", 5*time.Second, "Write timeout")
	idleTimeout := flag.Duration("idle-timeout", 30*time.Second, "Idle timeout")
	topN := flag.Int("top", 5, "How many hosts to show per interval")
	flag.Parse()

	log.Printf("fast-ok-server starting on %s (GOMAXPROCS=%d)", *addr, runtime.GOMAXPROCS(0))

	h := func(ctx *fasthttp.RequestCtx) {
		host := strings.ToLower(string(ctx.Host()))
		if host == "" {
			host = "(no-host)"
		}

		headersLen := len(ctx.Request.Header.Header())
		bodyLen := len(ctx.Request.Body())
		methodLen := len(ctx.Method())
		uriLen := len(ctx.RequestURI())
		estReqLine := 11
		reqSize := uint64(headersLen + bodyLen + methodLen + uriLen + estReqLine)

		atomic.AddUint64(&totalBytes, reqSize)
		atomic.AddUint64(&totalRequests, 1)

		v, ok := hostMap.Load(host)
		if !ok {
			newHS := &hostStats{}
			if actual, loaded := hostMap.LoadOrStore(host, newHS); loaded {
				v = actual
			} else {
				v = newHS
			}
		}
		hs := v.(*hostStats)
		atomic.AddUint64(&hs.requests, 1)
		atomic.AddUint64(&hs.bytes, reqSize)

		switch string(ctx.Method()) {
		case fasthttp.MethodGet:
			atomic.AddUint64(&methods.get, 1)
		case fasthttp.MethodPost:
			atomic.AddUint64(&methods.post, 1)
		default:
			atomic.AddUint64(&methods.other, 1)
		}

		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("text/plain; charset=utf-8")
		ctx.SetBodyString("OK")
	}

	server := &fasthttp.Server{
		Handler:                       h,
		Name:                          "fast-ok-server",
		ReadTimeout:                   *readTimeout,
		WriteTimeout:                  *writeTimeout,
		IdleTimeout:                   *idleTimeout,
		NoDefaultServerHeader:         true,
		NoDefaultContentType:          true,
		DisableHeaderNamesNormalizing: true,
		ReduceMemoryUsage:             true,
		CloseOnShutdown:               true,
		LogAllErrors:                  false,
	}

	go func(interval time.Duration, top int) {
		prevSnapshots := make(map[string]hostStats)
		var prevTotalReq, prevTotalBytes uint64

		for range time.Tick(interval) {
			currTotalReq := atomic.LoadUint64(&totalRequests)
			currTotalBytes := atomic.LoadUint64(&totalBytes)
			dr := currTotalReq - prevTotalReq
			db := currTotalBytes - prevTotalBytes
			avg := 0.0
			if dr > 0 {
				avg = float64(db) / float64(dr)
			}

			mg := atomic.LoadUint64(&methods.get)
			mp := atomic.LoadUint64(&methods.post)
			mo := atomic.LoadUint64(&methods.other)

			type item struct {
				host  string
				req   uint64
				bytes uint64
				avg   float64
			}
			var items []item

			hostMap.Range(func(k, v any) bool {
				h := k.(string)
				hs := v.(*hostStats)
				currReq := atomic.LoadUint64(&hs.requests)
				currBytes := atomic.LoadUint64(&hs.bytes)
				prev := prevSnapshots[h]
				dreq := currReq - prev.requests
				dbytes := currBytes - prev.bytes
				if dreq > 0 {
					items = append(items, item{
						host:  h,
						req:   dreq,
						bytes: dbytes,
						avg:   float64(dbytes) / float64(dreq),
					})
				}
				prevSnapshots[h] = hostStats{requests: currReq, bytes: currBytes}
				return true
			})

			sort.Slice(items, func(i, j int) bool { return items[i].req > items[j].req })
			if len(items) > top {
				items = items[:top]
			}

			uptime := time.Since(startTime).Truncate(time.Second)
			log.Printf("stats: req/s ~ %d | bytes/s ~ %d | avg req %.1f B | totals: %d req, %d B | methods: GET=%d POST=%d OTHER=%d | uptime=%s",
				dr/uint64(interval.Seconds()),
				db/uint64(interval.Seconds()),
				avg,
				currTotalReq,
				currTotalBytes,
				mg, mp, mo,
				uptime,
			)

			if len(items) > 0 {
				for _, it := range items {
					log.Printf("  host: %-40s | req/s ~ %d | avg %.1f B | interval: %d req, %d B",
						it.host,
						it.req/uint64(interval.Seconds()),
						it.avg,
						it.req,
						it.bytes,
					)
				}
			}

			prevTotalReq, prevTotalBytes = currTotalReq, currTotalBytes
		}
	}(*statsEvery, *topN)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ln, err := net.Listen("tcp4", *addr)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}

	go func() {
		if err := server.Serve(ln); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down...")
	if err := server.Shutdown(); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	currReq := atomic.LoadUint64(&totalRequests)
	currBytes := atomic.LoadUint64(&totalBytes)
	avg := 0.0
	if currReq > 0 {
		avg = float64(currBytes) / float64(currReq)
	}
	fmt.Printf("final totals: %d requests, %d bytes, avg size %.1f bytes, uptime=%s\n",
		currReq,
		currBytes,
		avg,
		time.Since(startTime).Truncate(time.Second),
	)
}
