package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GodlikePenguin/agogos-datatypes"
	"github.com/davecgh/go-spew/spew"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

var hostCheck string
var applicationList apps

type apps struct {
	sync.Mutex
	applications []Datatypes.Application
}

func (a *apps) getApps() []Datatypes.Application {
	a.Lock()
	toReturn := a.applications
	a.Unlock()
	return toReturn
}

func (a *apps) setApps(app []Datatypes.Application) {
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
	// parse the targetURL
	targetURL, _ := url.Parse(target)

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

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
	log.Println(req.URL.String())
	spew.Dump(req.Header)

	_, _ = fmt.Fprintln(res, "foo")

	//serveReverseProxy(url, res, req)
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

		var a []Datatypes.Application
		if err = json.Unmarshal(resp, &a); err != nil {
			log.Println("Error unmarshalling json response: ", err.Error())
			continue
		}
		applicationList.setApps(a)
	}
}

func getRequest(address string) ([]byte, error) {
	resp, err := http.Get(address)
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