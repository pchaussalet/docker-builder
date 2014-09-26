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
	"archive/tar"
	"io/ioutil"
	"strings"
	"crypto/rand"
	"crypto/md5"
	"math/big"
	"os/exec"
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

func createTempDir() (string, error) {
	randomInt, err := rand.Int(rand.Reader, big.NewInt(65536))
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%v", randomInt))))
	wd, err := ioutil.TempDir(workingDir, hash)
	if err != nil {
		return "", err
	}
	log.Printf("Extracting to %v\n", wd)
	return wd, nil
}

func extractBody(body io.Reader) (string, error) {
	tarReader := tar.NewReader(body)
	wd, err := createTempDir()
	if err != nil {
		return "", err
	}
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		destName := strings.Join([]string{wd, header.Name}, "/")
		fmt.Println([]string{wd, header.Name})
		mode := header.FileInfo().Mode()
		if header.FileInfo().IsDir() {
			os.MkdirAll(destName, mode)
		} else {
			destFile, err := os.Create(destName)
			if err != nil {
				return "", err
			}
			destFile.Chmod(mode)
			io.Copy(destFile, tarReader)
		}
	}
	return wd, nil
}

func buildImage(w http.ResponseWriter, r *http.Request) {
	wd, err := extractBody(r.Body)
	if err != nil {
		sendError(err, w)
		return
	}
	tag := r.Header.Get("x-tag")
	cmd := exec.Command("docker", "build", "-t", tag, wd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("%v - %v", cmd.Args, string(output))
		sendError(err, w)
		return
	}

	registryHost := r.Header.Get("x-registry")
	registryTag := fmt.Sprintf("%v/%v", registryHost, tag)
	cmd = exec.Command("docker", "tag", tag, registryTag)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%v - %v", cmd.Args, string(output))
		sendError(err, w)
		return
	}

	cmd = exec.Command("docker", "push", registryTag)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%v - %v", cmd.Args, string(output))
		sendError(err, w)
		return
	}

}

