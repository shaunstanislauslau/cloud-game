package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/giongto35/cloud-game/config"
	"github.com/giongto35/cloud-game/overlord"
	"github.com/giongto35/cloud-game/worker"
	"github.com/gorilla/websocket"
)

const gamePath = "games"

// Time allowed to write a message to the peer.
var upgrader = websocket.Upgrader{}

func createOverlordConnection() (*websocket.Conn, error) {
	c, _, err := websocket.DefaultDialer.Dial(*config.OverlordHost, nil)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// initilizeOverlord setup an overlord server
func initilizeOverlord() {
	overlord := overlord.NewServer()

	http.HandleFunc("/", overlord.GetWeb)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// browser facing port
	go func() {
		http.HandleFunc("/ws", overlord.WS)
	}()

	// worker facing port
	http.HandleFunc("/wso", overlord.WSO)
	http.ListenAndServe(":8000", nil)
}

// initializeWorker setup a worker
func initializeWorker() {
	conn, err := createOverlordConnection()
	if err != nil {
		log.Println("Cannot connect to overlord")
		log.Println("Run as a single server")
	}

	port := rand.Int()%100 + 8000
	host := getOutboundIP().String() + ":" + strconv.Itoa(port)
	//config.WorkerPort = port
	log.Println("Listening ", host)

	worker := worker.NewHandler(conn, *config.IsDebug, gamePath, host)

	defer func() {
		log.Println("Close worker")
		worker.Close()
	}()

	go worker.Run()

	// Echo response back, this endpoint is to test latency with browser
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Frontend echoing...")
		w.Write([]byte("echo back"))
	})
	http.ListenAndServe(":"+strconv.Itoa(port), nil)
}

// get preferred outbound ip of this machine
func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)

	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP

}

func monitor() {
	c := time.Tick(time.Second)
	for range c {
		log.Printf("#goroutines: %d\n", runtime.NumGoroutine())
	}
}

func main() {
	flag.Parse()

	rand.Seed(time.Now().UTC().UnixNano())

	//if *config.IsMonitor {
	go monitor()
	//}
	// There are two server mode
	// Overlord is coordinator. If the OvelordHost Param is `overlord`, we spawn a new host as Overlord.
	// else we spawn new server as normal server connecting to OverlordHost.
	if *config.OverlordHost == "overlord" {
		log.Println("Running as overlord ")
		initilizeOverlord()
	} else {
		if strings.HasPrefix(*config.OverlordHost, "ws") && !strings.HasSuffix(*config.OverlordHost, "wso") {
			log.Fatal("Overlord connection is invalid. Should have the form `ws://.../wso`")
		}
		log.Println("Running as worker ")
		initializeWorker()
	}
}
