package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"docker-agent/docker"

	"github.com/gorilla/mux"
)

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}

func listDeployments(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: listDeployments")
	json.NewEncoder(w).Encode(docker.GetServicesStatus())
}

func stopDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["id"]
	fmt.Println("Stopping service " + serviceName)
	docker.StopService(serviceName)
	fmt.Fprintf(w, "Service stopped")
}

func createDeployment(w http.ResponseWriter, r *http.Request) {

	var service docker.Service
	err := json.NewDecoder(r.Body).Decode(&service)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("New service " + service.Name + " " + service.Image)
	docker.DeployService(service.Image, service.Name, service.Instances)
	json.NewEncoder(w).Encode(docker.GetServices())
}

func updateDeployment(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	serviceId := vars["id"]
	instances, errParam := strconv.Atoi(vars["instances"])

	if errParam != nil {
		http.Error(w, errParam.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("Update service " + serviceId)
	docker.UpdateService(serviceId, instances)
	json.NewEncoder(w).Encode(docker.GetServices())
}

func handleRequests() {

	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/", homePage)
	myRouter.HandleFunc("/deployments", listDeployments)
	myRouter.HandleFunc("/deployment", createDeployment).Methods("POST")
	myRouter.HandleFunc("/deployment/{id}/{instances}", updateDeployment).Methods("PUT")
	myRouter.HandleFunc("/deployment/{id}", stopDeployment).Methods("DELETE")

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	go docker.UpdateServicesMonitor(10000)
	handleRequests()
}
