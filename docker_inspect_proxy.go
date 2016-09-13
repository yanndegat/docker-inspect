package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type HostConfig struct {
	PublicIP string
}

var (
	BindPort                    int
	TlsVerify                   bool
	Endpoint, Cert, Key, Cacert string
	Client                      *docker.Client
	Host                        HostConfig
)

func check(e error) {
	if e != nil {
		log.Fatal("Unexpected error:", e)
		panic(e)
	}
}

/* Example of a /proc/net/route
Iface   Destination     Gateway         Flags   RefCnt  Use     Metric  Mask            MTU     Window  IRTT
eth0    00000000        0101EB0A        0003    0       0       1024    00000000        0       0       0
eth0    0001EB0A        00000000        0001    0       0       0       00FFFFFF        0       0       0
eth0    0101EB0A        00000000        0005    0       0       1024    FFFFFFFF        0       0       0
docker0 000011AC        00000000        0001    0       0       0       0000FFFF        0       0       0
docker_gwbridge 000012AC        00000000        0001    0       0       0       0000FFFF        0       0       0
*/
func routes() ([]string, error) {
	dat, err := ioutil.ReadFile("/proc/net/route")
	if err != nil {
		return nil, err
	}
	// filters header line
	return strings.Split(string(dat), "\n")[1:], nil
}

func defaultRoute() ([]string, error) {
	routes, err := routes()
	if err != nil {
		return nil, err
	}
	for i := range routes {
		route := strings.Split(routes[i], "\t")
		if route[1] == "00000000" {
			return route, nil
		}
	}
	return nil, errors.New("default route not found")
}

func defaultInterface() (*net.Interface, error) {
	defaultRoute, e1 := defaultRoute()
	if e1 != nil {
		return nil, e1
	}

	interfaces, e2 := net.Interfaces()
	if e2 != nil {
		return nil, e2
	}

	for i := range interfaces {
		if defaultRoute[0] == interfaces[i].Name {
			return &interfaces[i], nil
		}
	}

	return nil, errors.New("default interface not found")
}

func defaultInterfaceAddr() (string, error) {
	defaultIf, e1 := defaultInterface()
	if e1 != nil {
		return "", e1
	}

	defaultAddrs, e2 := defaultIf.Addrs()
	if e2 != nil {
		return "", e2
	}

	// Addrs are AAA.BBB.CCC.DDD/BLOCK
	return strings.Split(defaultAddrs[0].String(), "/")[0], nil
}

func usage() {
	flag.Usage()
}

func setupFlags() {
	var (
		envBindPort                             int
		envEndpoint, envCacert, envCert, envKey string
		envTlsverify                            bool
	)

	if os.Getenv("BIND_PORT") == "" {
		envBindPort = 2204
	} else {
		port, err := strconv.Atoi(os.Getenv("BIND_PORT"))
		if err != nil {
			log.Fatal("Failed to parse BIND_PORT env var:", err)
			usage()
			os.Exit(1)
		} else {
			envBindPort = port
		}
	}

	if os.Getenv("DOCKER_TLS_VERIFY") == "" {
		envTlsverify = false
	} else {
		verify, err := strconv.ParseBool(os.Getenv("DOCKER_TLS_VERIFY"))
		if err != nil {
			log.Fatal("Failed to parse DOCKER_TLS_VERIFY env var:", err)
			usage()
			os.Exit(1)
		} else {
			envTlsverify = verify
		}
	}

	if os.Getenv("DOCKER_HOST") == "" {
		envEndpoint = "unix:///var/run/docker.sock"
	} else {
		envEndpoint = os.Getenv("DOCKER_HOST")
	}

	if os.Getenv("DOCKER_TLS_CACERT") != "" {
		envCacert = os.Getenv("DOCKER_TLS_CACERT")
	}

	if os.Getenv("DOCKER_TLS_CERT") != "" {
		envCert = os.Getenv("DOCKER_TLS_CERT")
	}

	if os.Getenv("DOCKER_TLS_KEY") != "" {
		envKey = os.Getenv("DOCKER_TLS_KEY")
	}

	flag.IntVar(&BindPort, "p", envBindPort, "bind port")
	flag.BoolVar(&TlsVerify, "tlsVerify", envTlsverify, "docker tls verify")
	flag.StringVar(&Endpoint, "h", envEndpoint, "docker host")
	flag.StringVar(&Cert, "cert", envCert, "docker tls cert")
	flag.StringVar(&Key, "key", envKey, "docker tls key")
	flag.StringVar(&Cacert, "cacert", envCacert, "docker tls cacert")
	flag.Parse()
}

var hostValidPath = regexp.MustCompile("^/host$")

func hostHandler(w http.ResponseWriter, r *http.Request) {
	if !hostValidPath.MatchString(r.URL.Path) {
		http.NotFound(w, r)
	} else {
		json, err := json.Marshal(Host)
		if err != nil {
			log.Println("Error while marshalling host config", err)
			http.Error(w, fmt.Sprintf("err:%s", err), 500)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(json)
	}
}

var containerValidPath = regexp.MustCompile("^/container/([a-zA-Z0-9]+)$")

func containerHandler(w http.ResponseWriter, r *http.Request) {
	m := containerValidPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return
	}
	containerId := m[1]
	container, err := Client.InspectContainer(containerId)
	if err != nil {
		log.Println("Error while inspecting container", err)
		http.Error(w, fmt.Sprintf("err:%s", err), 500)
	}

	json, err := json.Marshal(container)
	if err != nil {
		log.Println("Error while inspecting container", err)
		http.Error(w, fmt.Sprintf("err:%s", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func main() {
	setupFlags()
	if TlsVerify || Cert != "" {
		log.Printf("Connecting to TLS secured docker through endpoint: %s.", Endpoint)
		client, err := docker.NewTLSClient(Endpoint, Cert, Key, Cacert)
		if err != nil {
			log.Fatal("Failed to connect to docker:", err)
			os.Exit(1)
		} else {
			Client = client
		}
	} else {
		log.Printf("Connecting to insecure docker through endpoint: %s.", Endpoint)
		client, err := docker.NewClient(Endpoint)
		if err != nil {
			log.Fatal("Failed to connect to docker:", err)
			os.Exit(1)
		} else {
			Client = client
		}
	}

	pubIP, err := defaultInterfaceAddr()
	check(err)
	Host = HostConfig{PublicIP: pubIP}
	log.Printf("HostConfig is : %s", Host)

	http.HandleFunc("/container/", containerHandler)
	http.HandleFunc("/host", hostHandler)

	log.Printf("Listening on port: %s", strconv.Itoa(BindPort))
	http.ListenAndServe(":"+strconv.Itoa(BindPort), nil)
}
