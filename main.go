package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/pion/webrtc/v2"
	"github.com/povilasv/prommod"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func Init() {

	// Generate pem file for https

	if _, err := os.Stat("cert.pem"); os.IsNotExist(err) {
		fmt.Println("Generating perm")
		genPem()
	}

	if _, err := os.Stat("key.pem"); os.IsNotExist(err) {
		fmt.Println("Generating perm")
		genPem()
	}

	// Create a MediaEngine object to configure the supported codec
	m = webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	// Create the API object with the MediaEngine
	api = webrtc.NewAPI(webrtc.WithMediaEngine(m))

}

func main() {
	Init()
	if err := prometheus.Register(prommod.NewCollector("sfu-ws")); err != nil {
		panic(err)
	}

	port := flag.String("p", "8443", "https port")
	flag.Parse()

	http.Handle("/metrics", promhttp.Handler())

	// Websocket handle func
	http.HandleFunc("/ws", ws)

	// web handle func
	http.HandleFunc("/sfu.js", js)
	http.HandleFunc("/", web)
	http.HandleFunc("/alice", alice)
	http.HandleFunc("/bob", bob)

	// Support https, so we can test by lan
	fmt.Println("Web listening :" + *port)
	panic(http.ListenAndServeTLS(":"+*port, "cert.pem", "key.pem", nil))
}
