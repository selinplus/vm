package main

import (
	"flag"
	"fmt"
	"github.com/pion/webrtc/v2"
	"github.com/povilasv/prommod"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
)

func checkError(err error) {

	if err != nil {
		log.Printf("err:-->%v", err)
	}
}

func init() {

	// Generate pem file for https
	//genPem()
	log.SetOutput(os.Stdout)
	// Create a MediaEngine object to configure the supported codec
	m = webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	// Create the API object with the MediaEngine
	api = webrtc.NewAPI(webrtc.WithMediaEngine(m))

}

func main() {
	if err := prometheus.Register(prommod.NewCollector("sfu_ws")); err != nil {
		panic(err)
	}

	port := flag.String("p", "8443", "https port")
	flag.Parse()

	js := http.FileServer(http.Dir("js"))
	css := http.FileServer(http.Dir("css"))
	img := http.FileServer(http.Dir("img"))
	http.Handle("/js/", http.StripPrefix("/js/",js))
	http.Handle("/css/", http.StripPrefix("/css/",css))
	http.Handle("/img/", http.StripPrefix("/img/",img))

	http.Handle("/metrics", promhttp.Handler())

	// Websocket handle func
	http.HandleFunc("/wspub", publisher)
	http.HandleFunc("/wssub", subcriber)

	// Html handle func
	http.HandleFunc("/", subcribe)
	http.HandleFunc("/pub", publish)

	// Support https, so we can test by lan
	fmt.Println("Web listening :" + *port)
	panic(http.ListenAndServeTLS(":"+*port, "/etc/letsencrypt/live/ytsw.info/fullchain.pem", "/etc/letsencrypt/live/ytsw.info/privkey.pem", nil))
	//panic(http.ListenAndServeTLS(":"+*port, "cert.pem", "key.pem", nil))
}
