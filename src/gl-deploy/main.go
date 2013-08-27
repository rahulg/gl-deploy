package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	SUCCESS_DATA = `{"message": "200: OK"}`
)

type Config struct {
	RepoBase     string
	RepoName     string
	RepoURL      string
	Branch       string
	Address      string
	DeployScript string
	UpdateScript string
}

var (
	REPO_DIR  string
	CONF_FILE string
	config    Config
	gitEvent  chan int
)

func main() {
	flag.StringVar(&CONF_FILE, "conf", "config.json", "Configuration file to use")
	flag.Parse()
	data, err := ioutil.ReadFile(CONF_FILE)
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatalln(err)
	}
	REPO_DIR = filepath.Join(config.RepoBase, config.RepoName)
	gitEvent = make(chan int)
	go eventLoop()
	http.HandleFunc("/update", CommitReceived)
	http.ListenAndServe(config.Address, nil)
}

type Incoming struct {
	Ref string
}

func CommitReceived(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	bodyData, _ := ioutil.ReadAll(req.Body)
	inc := Incoming{}
	json.Unmarshal(bodyData, &inc)

	if inc.Ref != "refs/heads/"+config.Branch {
		log.Println("GET /update. Ref =", inc.Ref, "IGNORING")
	} else {
		log.Println("GET /update. Ref =", inc.Ref, "FETCHING")
		gitEvent <- 1
	}
	rw.Write([]byte(SUCCESS_DATA))

}

func eventLoop() {
	for {
		_, ok := <-gitEvent

		if !ok {
			log.Fatalln("Channel closed!")
		}

		_, err := os.Stat(REPO_DIR)
		if os.IsNotExist(err) {
			os.Chdir(config.RepoBase)
			o, err := exec.Command("git", "clone", config.RepoURL, config.RepoName).CombinedOutput()
			if err != nil {
				log.Println(string(o))
				log.Println(err)
				continue
			}
			os.Chdir(REPO_DIR)
			o, err = exec.Command("git", "checkout", config.Branch).CombinedOutput()
			if err != nil {
				log.Println(string(o))
				log.Println(err)
				continue
			}
			o, err = exec.Command("bash", config.DeployScript).CombinedOutput()
			if err != nil {
				log.Println(string(o))
				log.Println(err)
				continue
			}
		} else {
			os.Chdir(REPO_DIR)
			o, err := exec.Command("git", "fetch").CombinedOutput()
			if err != nil {
				log.Println(string(o))
				log.Println(err)
				continue
			}
			o, err = exec.Command("git", "reset", "--hard", "origin/"+config.Branch).CombinedOutput()
			if err != nil {
				log.Println(string(o))
				log.Println(err)
				continue
			}
			o, err = exec.Command("bash", config.UpdateScript).CombinedOutput()
			if err != nil {
				log.Println(string(o))
				log.Println(err)
				continue
			}
		}
	}
}
