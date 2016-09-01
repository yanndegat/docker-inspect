package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
)

var (
	BindPort                    int
	TlsVerify                   bool
	Endpoint, Cert, Key, Cacert string
	Client                      *docker.Client
)

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

	if os.Getenv("DOCKER_TLS_CACERT") == "" {
		envCacert = os.Getenv("DOCKER_TLS_CACERT")
	}

	if os.Getenv("DOCKER_TLS_CERT") == "" {
		envCert = os.Getenv("DOCKER_TLS_CERT")
	}

	if os.Getenv("DOCKER_TLS_KEY") == "" {
		envKey = os.Getenv("DOCKER_TLS_Key")
	}

	flag.IntVar(&BindPort, "p", envBindPort, "bind port")
	flag.BoolVar(&TlsVerify, "tlsVerify", envTlsverify, "bind port")
	flag.StringVar(&Endpoint, "h", envEndpoint, "docker host")
	flag.StringVar(&Cert, "cert", envCert, "docker host")
	flag.StringVar(&Key, "key", envKey, "docker host")
	flag.StringVar(&Cacert, envCacert, "./cacert.pem", "docker host")
	flag.Parse()
}

var validPath = regexp.MustCompile("^/container/([a-zA-Z0-9]+)$")

func containerHandler(w http.ResponseWriter, r *http.Request) {
	m := validPath.FindStringSubmatch(r.URL.Path)
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
		client, err := docker.NewTLSClient(Endpoint, Cert, Key, Cacert)
		if err != nil {
			log.Fatal("Failed to connect to docker:", err)
			os.Exit(1)
		} else {
			Client = client
		}
	} else {
		log.Printf("Connecting to docker through endpoint: %s.", Endpoint)
		client, err := docker.NewClient(Endpoint)
		if err != nil {
			log.Fatal("Failed to connect to docker:", err)
			os.Exit(1)
		} else {
			Client = client
		}
	}

	http.HandleFunc("/container/", containerHandler)

	log.Printf("Listening on port: %s", strconv.Itoa(BindPort))
	http.ListenAndServe(":"+strconv.Itoa(BindPort), nil)
}
