package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var peopleMap = make(map[string]int)
var messageChan = make(chan message)

type WorkerPool interface {
	Run()
	AddTask()
}
type job struct {
	Request  *http.Request
	Response http.ResponseWriter
}

type workerPool struct {
	maxWorker int
	jobQueue  chan job
}

func NewWorkerPool(maxWorker int) *workerPool {
	return &workerPool{
		maxWorker: maxWorker,
		jobQueue:  make(chan job),
	}
}

func (wp *workerPool) Run() {
	for i := 0; i < wp.maxWorker; i++ {
		go func() {
			for job := range wp.jobQueue {
				handleRequest(job.Response, job.Request)
			}
		}()
	}
}

func (wp *workerPool) AddTask(task *job) {
	wp.jobQueue <- *task
}

type People struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}
type message struct {
	command  string
	key      string
	value    int
	response chan<- string
}

func run() {
	for mes := range messageChan {
		switch mes.command {
		case "get":
			res := "{"
			for k, v := range peopleMap {
				res += "\n" + k + ": " + strconv.Itoa(v)
			}
			res += "\n}"
			mes.response <- res
		case "put":
			peopleMap[mes.key] = mes.value
			mes.response <- fmt.Sprintf("%s:%d", mes.key, mes.value)
		case "delete":
			delete(peopleMap, mes.key)
			mes.response <- fmt.Sprintf("%s has deleted", mes.key)
		}
	}
}
func handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/delete/"):]
	if name == "" {
		http.Error(w, "empty string", http.StatusBadRequest)
	}
	res := make(chan string)
	defer close(res)
	messageChan <- message{command: "delete", key: name, response: res}
	fmt.Fprintln(w, <-res)
}
func handleRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handleRequest")
	if strings.Contains(r.URL.Path, "/delete/") {
		handleDelete(w, r)
		return
	}

	response := make(chan string)
	switch r.Method {
	case "GET":
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("--get")
			messageChan <- message{command: "get", response: response}
			responseData := <-response
			fmt.Fprintln(w, "hi")
			fmt.Fprintln(w, responseData)
			fmt.Println("Received response data:", responseData)
			fmt.Println("--finish--")
		}(w, r)
	case "POST":
		func(w http.ResponseWriter, r *http.Request) {
			people := requestDecoder(w, r)
			messageChan <- message{command: "put", key: people.Name, value: people.Age, response: response}
			fmt.Fprintln(w, <-response)
		}(w, r)
	case "PUT":
		func(w http.ResponseWriter, r *http.Request) {
			people := requestDecoder(w, r)
			messageChan <- message{command: "put", key: people.Name, value: people.Age, response: response}
			fmt.Fprintln(w, <-response)
		}(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
func NewServer() {
	peopleMap["yan"] = 18
	peopleMap["li"] = 19
	workerPool := NewWorkerPool(10)
	workerPool.Run()

	go run()
	server := &http.Server{Addr: ":8080"}
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		workerPool.AddTask(&job{Request: r, Response: w})
	})
	http.HandleFunc("/delete/", func(w http.ResponseWriter, r *http.Request) {
		workerPool.AddTask(&job{Request: r, Response: w})
	})
	terminate := make(chan os.Signal, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal("server error: ", err)
		}
	}()

	go func() {
		<-terminate
		err := server.Close()
		if err != nil {
			log.Fatal("server error: ", err)
		}
	}()

	<-terminate
}
func requestDecoder(w http.ResponseWriter, r *http.Request) People {
	decoder := json.NewDecoder(r.Body)
	var people People
	err := decoder.Decode(&people)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	return people
}
