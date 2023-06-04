package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
)

var subnet string
var proxyAddress string

type transportPool struct {
	pool *sync.Pool
}

func newTransportPool() *transportPool {
	return &transportPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return &http.Transport{}
			},
		},
	}
}

func (t *transportPool) get() *http.Transport {
	return t.pool.Get().(*http.Transport)
}

func (t *transportPool) put(transport *http.Transport) {
	t.pool.Put(transport)
}

func init() {
	flag.StringVar(&proxyAddress, "proxy-address", "127.0.0.1:4747", "Proxy listen address, e.g., 127.0.0.1:4747")
	flag.StringVar(&subnet, "subnet", "", "IPv6 subnet, e.g., 4747:4747:4747:4747::/64")
	flag.Parse()
	if subnet == "" {
		log.Fatalf("Subnet is required")
	}
}
func getRandomIPFromSubnet(subnet string) (string, error) {
	subnetParts := strings.Split(subnet, "/")
	prefix := subnetParts[0]
	mask, err := strconv.Atoi(subnetParts[1])
	if err != nil {
		return "", fmt.Errorf("failed to parse subnet mask: %v", err)
	}

	// Calculate the number of random bits.
	randomBits := 128 - mask
	if randomBits <= 0 {
		return "", fmt.Errorf("subnet mask is too large")
	}

	// Calculate the number of random bytes.
	randomBytes := randomBits / 8
	if randomBits%8 > 0 {
		randomBytes++
	}

	// Generate the random part.
	randomPart := make([]byte, randomBytes)
	if _, err := rand.Read(randomPart); err != nil {
		return "", fmt.Errorf("failed to generate random IPv6 address: %v", err)
	}

	// Clear the extra random bits.
	if extraBits := randomBits % 8; extraBits > 0 {
		randomPart[0] &= 0xFF >> extraBits
	}

	// Combine the prefix and the random part.
	prefixBytes := net.ParseIP(prefix).To16()
	for i := 0; i < len(randomPart); i++ {
		prefixBytes[16-len(randomPart)+i] |= randomPart[i]
	}

	return net.IP(prefixBytes).String(), nil
}

func handleRequestAndRedirect(res http.ResponseWriter, req *http.Request, transport *http.Transport) {
	randomIP, err := getRandomIPFromSubnet(subnet)
	if err != nil {
		log.Printf("Error generating random IPv6 address: %v", err)
		return
	}

	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP: net.ParseIP(randomIP),
		},
	}

	transport.DialContext = dialer.DialContext

	director := func(request *http.Request) {
		request.URL.Scheme = req.URL.Scheme
		request.URL.Host = req.URL.Host
		request.Host = req.Host
		request.RequestURI = ""
	}

	proxy := &httputil.ReverseProxy{Director: director, Transport: transport}
	proxy.ServeHTTP(res, req)
}

func main() {
	tPool := newTransportPool()

	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		transport := tPool.get()
		defer tPool.put(transport)

		handleRequestAndRedirect(res, req, transport)
	})

	log.Printf("Starting proxy server on %s", proxyAddress)
	if err := http.ListenAndServe(proxyAddress, nil); err != nil {
		log.Fatalf("Error starting proxy server: %v", err)
	}
}
