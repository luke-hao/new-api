// migration-gate is a loopback-only, bounded request admission proxy.
package main

import (
	"flag"
	"github.com/QuantumNous/new-api/pkg/migrationgate"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:13000", "HTTP loopback listen address")
	target := flag.String("target", "http://127.0.0.1:3000", "initial upstream")
	socket := flag.String("control", "", "absolute private Unix socket path")
	queue := flag.Int("queue", 512, "maximum waiting requests")
	wait := flag.Duration("wait", 30*time.Second, "maximum admission wait")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || host != "127.0.0.1" || !filepath.IsAbs(*socket) {
		log.Fatal("loopback address and absolute control socket required")
	}
	g, err := migrationgate.New(*target, *queue, *wait)
	if err != nil {
		log.Fatal(err)
	}
	// Never unlink an existing socket: another gate may own live requests.
	listener, err := net.Listen("unix", *socket)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	defer os.Remove(*socket)
	if err = os.Chmod(*socket, 0600); err != nil {
		log.Fatal(err)
	}
	go func() {
		log.Print((&http.Server{Handler: http.HandlerFunc(g.Control), ReadHeaderTimeout: 5 * time.Second}).Serve(listener))
	}()
	log.Printf("admission starts paused on %s", *listen)
	log.Fatal((&http.Server{Addr: *listen, Handler: g, ReadHeaderTimeout: 10 * time.Second}).ListenAndServe())
}
