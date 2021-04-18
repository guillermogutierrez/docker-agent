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

func listDeployments(w http.ResponseWriter, r *http.Request) {
	fmt.Println("API Call: List Deployments")
	json.NewEncoder(w).Encode(docker.GetDeploymentsStatus())
}

func stopDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deploymentName := vars["id"]
	fmt.Println("API Call: Stop the deployment ", deploymentName)
	docker.StopDeployment(deploymentName)
}

func createDeployment(w http.ResponseWriter, r *http.Request) {
	var deployment docker.Deployment
	err := json.NewDecoder(r.Body).Decode(&deployment)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("API Call: Create a new deployment ", deployment.Name, " using the image ", deployment.Image, " and running ", deployment.Instances, " instance(s)")
	docker.DeployDeployment(deployment.Image, deployment.Name, deployment.Instances)
	json.NewEncoder(w).Encode(docker.GetDeployments())
}

func updateDeployment(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	deploymentId := vars["id"]
	instances, errParam := strconv.Atoi(vars["instances"])

	if errParam != nil {
		http.Error(w, errParam.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("API Call: Update Deployment ", deploymentId, " to ", instances, " instance(s)")
	docker.UpdateDeployment(deploymentId, instances)
	json.NewEncoder(w).Encode(docker.GetDeployments())
}

func handleRequests() {

	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/deployments", listDeployments)
	myRouter.HandleFunc("/deployment", createDeployment).Methods("POST")
	myRouter.HandleFunc("/deployment/{id}/{instances}", updateDeployment).Methods("PUT")
	myRouter.HandleFunc("/deployment/{id}", stopDeployment).Methods("DELETE")

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	go docker.UpdateDeploymentsMonitor(10000)
	handleRequests()
}
