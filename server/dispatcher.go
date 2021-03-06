package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rukavina/mmock/definition"
	"github.com/rukavina/mmock/match"
	"github.com/rukavina/mmock/proxy"
	"github.com/rukavina/mmock/scenario"
	"github.com/rukavina/mmock/statistics"
	"github.com/rukavina/mmock/translate"
	"github.com/rukavina/mmock/vars"
)

//Dispatcher is the mock http server
type Dispatcher struct {
	IP         string
	Port       int
	PortTLS    int
	ConfigTLS  string
	Resolver   Resolver
	Translator translate.MessageTranslator
	Processor  vars.Processor
	Scenario   scenario.Director
	Spier      match.Spier
	Mlog       chan definition.Match
}

func (di Dispatcher) recordMatchData(msg definition.Match) {
	di.Mlog <- msg
}

func (di Dispatcher) randomStatusCode(currentStatus int) int {
	if time.Now().Second()%2 == 0 {
		rand.Seed(time.Now().Unix())
		return rand.Intn(4) + 500
	}
	return currentStatus
}

func (di Dispatcher) callWebHook(url string, match *definition.Match) {
	log.Printf("Running webhook: %s\n", url)
	content, err := json.Marshal(match)
	if err != nil {
		log.Println("Impossible encode the WebHook payload")
		return
	}
	reader := bytes.NewReader(content)
	resp, err := http.Post(url, "application/json", reader)
	if err != nil {
		log.Printf("Impossible send payload to: %s\n", url)
		return
	}
	log.Printf("WebHook response: %d\n", resp.StatusCode)
}

//ServerHTTP is the mock http server request handler.
//It uses the router to decide the matching mock and translator as adapter between the HTTP impelementation and the mock definition.
func (di *Dispatcher) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	mRequest := di.Translator.BuildRequestDefinitionFromHTTP(req)

	if mRequest.Path == "/favicon.ico" {
		return
	}

	log.Printf("New request: %s %s\n", req.Method, req.URL.String())

	mock, match := di.getMatchingResult(&mRequest)

	//save the match info
	di.Spier.Save(*match)

	//set new scenario
	if mock.Control.Scenario.NewState != "" {
		statistics.TrackScenarioFeature()
		di.Scenario.SetState(mock.Control.Scenario.Name, mock.Control.Scenario.NewState)
	}

	if mock.Control.WebHookURL != "" {
		go di.callWebHook(mock.Control.WebHookURL, match)
	}

	//translate request
	di.Translator.WriteHTTPResponseFromDefinition(match.Response, w)

	go di.recordMatchData(*match)
}

func (di *Dispatcher) getMatchingResult(request *definition.Request) (*definition.Mock, *definition.Match) {
	response := &definition.Response{}
	result := definition.Result{}
	mock, errs := di.Resolver.Resolve(request)
	if errs == nil {
		result.Found = true
	} else {
		result.Found = false
		result.Errors = errs
	}

	log.Printf("Mock match found: %s. Name : %s\n", strconv.FormatBool(result.Found), mock.URI)

	if result.Found {
		if len(mock.Control.ProxyBaseURL) > 0 {
			statistics.TrackProxyFeature()
			pr := proxy.Proxy{URL: mock.Control.ProxyBaseURL}
			response = pr.MakeRequest(mock.Request)
		} else {

			di.Processor.Eval(request, mock)
			if mock.Control.Crazy {
				log.Println("Running crazy mode")
				mock.Response.StatusCode = di.randomStatusCode(mock.Response.StatusCode)
			}
			if mock.Control.Delay > 0 {
				log.Println("Adding a delay")
				time.Sleep(time.Duration(mock.Control.Delay) * time.Second)
			}

			response = &mock.Response
		}

		statistics.TrackMockRequest()
	} else {
		response = &mock.Response
	}

	match := &definition.Match{Time: time.Now().Unix(), Request: request, Response: response, Result: result}

	return mock, match

}

//Start initialize the HTTP mock server
func (di Dispatcher) Start() {
	addr := fmt.Sprintf("%s:%d", di.IP, di.Port)
	addrTLS := fmt.Sprintf("%s:%d", di.IP, di.PortTLS)

	errCh := make(chan error)

	go func() {
		errCh <- http.ListenAndServe(addr, &di)
	}()

	go func() {
		err := di.listenAndServeTLS(addrTLS)
		if err != nil {
			log.Println("Impossible start the application.")
			errCh <- err
		}
	}()

	err := <-errCh

	if err != nil {
		log.Fatalf("ListenAndServe: " + err.Error())
	}

}

func (di Dispatcher) listenAndServeTLS(addrTLS string) error {
	tlsConfig := &tls.Config{}
	pattern := fmt.Sprintf("%s/*.crt", di.ConfigTLS)
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		log.Println("TLS certificates not found, impossible to start the TLS server.")
		return nil
	}

	for _, crt := range files {
		extension := filepath.Ext(crt)
		name := crt[0 : len(crt)-len(extension)]
		key := fmt.Sprint(name, ".key")
		log.Printf("Loading X509KeyPair (%s/%s)\n", filepath.Base(crt), filepath.Base(key))
		certificate, err := tls.LoadX509KeyPair(crt, key)
		if err != nil {
			return fmt.Errorf("Invalid certificate: %v", crt)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
	}
	tlsConfig.BuildNameToCertificate()

	server := http.Server{
		Addr:      addrTLS,
		TLSConfig: tlsConfig,
		Handler:   &di,
	}

	return server.ListenAndServeTLS("", "")
}
