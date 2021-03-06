package main

import (
	"encoding/json"
	"errors"
	"fmt"
	datatypes "github.com/Ringleadr/ringleadr-datatypes"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var hostCheck string
var applicationList apps

type apps struct {
	sync.Mutex
	applications []datatypes.Application
}

func (a *apps) getApps() []datatypes.Application {
	a.Lock()
	toReturn := a.applications
	a.Unlock()
	return toReturn
}

func (a *apps) setApps(app []datatypes.Application) {
	a.Lock()
	a.applications = app
	a.Unlock()
}

// Get the port to listen on
func getListenAddress() string {
	return ":14442"
}

/*
	Reverse Proxy Logic
*/

// Serve a reverse proxy for a given url
func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {
	log.Println("Trying to redirect to", target)
	// parse the targetURL
	targetURL, _ := url.Parse(target)

	director := func(newReq *http.Request) {
		newReq.URL = targetURL
		newReq.URL.RawQuery = targetURL.RawQuery
		if _, ok := newReq.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			newReq.Header.Set("User-Agent", "")
		}
	}
	proxy := &httputil.ReverseProxy{Director: director}

	// Update the headers to allow for SSL redirection
	req.URL.Host = targetURL.Host
	req.URL.Scheme = targetURL.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = targetURL.Host

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}

// Given a request send it to the appropriate url
func handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	split := strings.Split(req.URL.String(), "/")
	if len(split) != 3 {
		_, _ = fmt.Fprintln(res, "Malformed URL")
		return
	}
	appName := split[1]
	compName := split[2]

	reqURL := req.Header.Get("X-agogos-query")
	if reqURL == "" {
		_, _ = fmt.Fprintln(res, "Missing agogos-url header")
		return
	}

	appCopy := applicationList.getApps()
	var ipAddr []string
	for _, app := range appCopy {
		if app.Node != hostCheck {
			continue
		}
		if app.Name != appName {
			continue
		}
		for _, comp := range app.Components {
			if comp.Name == compName {
				ipAddr = append(ipAddr, comp.NetworkInfo["bridge"]...)
			}
		}
	}

	if len(ipAddr) == 0 {
		_, _ = fmt.Fprintln(res, "not on this node")
		return
	}

	randomIP := ipAddr[rand.Intn(len(ipAddr))]
	formatURL := strings.Replace(reqURL, compName, randomIP, 1)

	serveReverseProxy(formatURL, res, req)
}

func main() {
	host := os.Getenv("AGOGOS_HOSTNAME")
	if host == "" {
		panic("AGOGOS_HOSTNAME not set. Exiting")
	}
	hostCheck = host

	//start watcher
	go appWatcher()

	// start server
	log.Println("Serving on", getListenAddress())
	http.HandleFunc("/", handleRequestAndRedirect)
	if err := http.ListenAndServe(getListenAddress(), nil); err != nil {
		panic(err)
	}
}

func appWatcher() {
	for {
		time.Sleep(5 * time.Second)
		resp, err := getRequest("http://host.docker.internal:14440/applications")
		if err != nil {
			log.Println("Error fetching applications: ", err.Error())
			continue
		}

		var a []datatypes.Application
		if err = json.Unmarshal(resp, &a); err != nil {
			log.Println("Error unmarshalling json response: ", err.Error())
			continue
		}
		applicationList.setApps(a)
	}
}

func getRequest(address string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-agogos-disable-log", "true")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(strconv.Itoa(resp.StatusCode) + " " + string(body))
	}

	return body, nil
}
