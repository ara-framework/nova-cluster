package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gookit/color"
	"github.com/gorilla/mux"
	"github.com/imdario/mergo"
)

// ViewDefinition contains the server which can render it.
type ViewDefinition struct {
	Server string
}

// ViewJob is request view for hypernova.
type ViewJob struct {
	Name string      `json:"name"`
	Data interface{} `json:"data"`
}

// ViewJobResult is the view result from hypernova.
type ViewJobResult struct {
	Name     string       `json:"name"`
	Html     string       `json:"html"`
	Duration float32      `json:"duration"`
	Success  bool         `json:"success"`
	Error    ViewJobError `json:"error"`
}

// ViewJobError is an error happened during and after a view is requesting.
type ViewJobError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// BatchResponse is an respose which contains several view job results.
type BatchResponse struct {
	Results map[string]ViewJobResult `json:"results"`
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Add("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method != "OPTIONS" {
			next.ServeHTTP(w, r)
		}
	})
}

func BatchRequest(server string, batch map[string]ViewJob) BatchResponse {
	b, encodeErr := json.Marshal(batch)

	if encodeErr != nil {
		log.Fatal(encodeErr)
	}

	payload := string(b)

	var response BatchResponse

	resp, reqErr := http.Post(server, "application/json", strings.NewReader(payload))

	if reqErr != nil {
		log.Println(reqErr)
		response = BatchResponse{
			Results: make(map[string]ViewJobResult),
		}

		for uuid, job := range batch {
			response.Results[uuid] = ViewJobResult{
				Name:    job.Name,
				Success: false,
				Error: ViewJobError{
					Name:    "ConnectionRefused",
					Message: reqErr.Error(),
				},
			}
		}

		return response
	}

	defer resp.Body.Close()

	body, bodyErr := ioutil.ReadAll(resp.Body)

	if bodyErr != nil {
		log.Fatal(bodyErr)
	}

	json.Unmarshal(body, &response)

	return response
}

func GetViewDefintions() map[string]ViewDefinition {
	dat, err := ioutil.ReadFile(os.Getenv("CONFIG_FILE"))

	if err != nil {
		log.Fatal("Config file not found")
	}

	var viewDefinitions map[string]ViewDefinition

	json.Unmarshal(dat, &viewDefinitions)

	return viewDefinitions
}

func BatchHandler(viewDefinitions map[string]ViewDefinition) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)

		if err != nil {
			log.Fatal(err)
		}

		var viewRequests map[string]ViewJob

		json.Unmarshal(b, &viewRequests)

		aggregatedResponse := BatchResponse{
			Results: make(map[string]ViewJobResult),
		}

		batches := make(map[string]map[string]ViewJob)

		for uuid, job := range viewRequests {
			server := viewDefinitions[job.Name].Server

			if server == "" {
				aggregatedResponse.Results[uuid] = ViewJobResult{
					Name:    job.Name,
					Success: false,
					Error: ViewJobError{
						Name:    "ReferenceError",
						Message: "Component\"" + job.Name + "\" not registered in cluster",
					},
				}
				continue
			}

			if batches[server] == nil {
				batch := make(map[string]ViewJob)
				batches[server] = batch
			}

			batches[server][uuid] = job
		}

		var wg sync.WaitGroup
		var batchResponsesMap sync.Map

		for server, batch := range batches {
			wg.Add(1)

			go func(server string, batch map[string]ViewJob) {
				response := BatchRequest(server, batch)
				batchResponsesMap.Store(server, response)
				defer wg.Done()
			}(server, batch)
		}

		wg.Wait()

		for server := range batches {
			response, _ := batchResponsesMap.Load(server)

			batchResponse := response.(BatchResponse)
			mergo.Merge(&aggregatedResponse.Results, batchResponse.Results)
		}

		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(aggregatedResponse)
	}
}

func main() {

	router := mux.NewRouter()
	router.Use(corsMiddleware)

	viewDefinitions := GetViewDefintions()

	router.HandleFunc("/batch", BatchHandler(viewDefinitions)).Methods("OPTIONS", "POST")

	port := os.Getenv("PORT")

	if len(port) == 0 {
		port = "8000"
	}

	color.Info.Printf("Nova cluster running on http://0.0.0.0:%s\n", port)
	http.ListenAndServe(":"+port, router)
}
