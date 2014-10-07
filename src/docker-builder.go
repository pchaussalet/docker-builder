package main

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"flag"
	"os"
	"fmt"
	"encoding/json"
	"io"
	"strings"
	"errors"
	"encoding/base64"
	"crypto/rand"
	"math/big"
)

var workingDir string

func main() {
	flags := flag.NewFlagSet("remote", flag.ExitOnError)
	host := flags.String("h", "127.0.0.1:14243", "Server address in the form ip:port")
	workingDir = *flags.String("w", "/tmp", "Working directory")
	flags.Parse(os.Args[1:])
	Start(*host)
}

func Start(host string) {
	r := mux.NewRouter()
	r.HandleFunc("/build", buildImage).Methods("POST")
	http.Handle("/", r)
	log.Printf("Listening on %v...", host)
	err := http.ListenAndServe(host, nil)
	if err != nil {
		log.Fatalf("An error has occured : %v", err)
	}
}

func sendJsonResponse(payload interface{}, w http.ResponseWriter, status int) {
	result, err := json.Marshal(payload)
	if err != nil {
		sendError(err, w)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	w.Write(result)
}

func sendError(err error, w http.ResponseWriter) {
	w.WriteHeader(500)
	log.Println(err)
}

func postDocker(url, contentType string, body io.ReadCloser) error {
	resp, err := http.DefaultClient.Post(url, contentType, body)
	if err != nil {
		return err
	} else if resp.StatusCode > 299 {
		return errors.New(fmt.Sprintf("Error received from builder: %v", resp.Status))
	}
	return nil
}

func buildImage(w http.ResponseWriter, r *http.Request) {
	uid, _ := rand.Int(rand.Reader, big.NewInt(999999999))
	log.Printf("[%09d]Build start", uid)
	registryHost := r.Header.Get("x-registry")
	tag := r.Header.Get("x-tag")
	registryTag := fmt.Sprintf("%v/%v", registryHost, tag)

	err := postDocker(fmt.Sprintf("http://127.0.0.1:4243/build?t=%v", tag), "application/tar", r.Body)
	if err != nil {
		sendError(err, w)
		return
	}

	err = postDocker(fmt.Sprintf("http://127.0.0.1:4243/images/%v/tag?repo=%v", tag, registryTag), "application/json", nil)
	if err != nil {
		sendError(err, w)
		return
	}

	elements := strings.Split(registryTag, ":")
	version := elements[len(elements)-1]
	image := strings.Join(elements[:len(elements)-1], ":")
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:4243/images/%v/push?tag=%v", image, version), nil)
	req.Header.Add("X-Registry-Auth", base64.StdEncoding.EncodeToString(([]byte)(strings.Replace("{'auth':'','email':''}", "'", "\"", 0))))
	http.DefaultClient.Do(req)
	if err != nil {
		sendError(err, w)
		return
	}
	log.Printf("[%09d]Build end", uid)
}

