package main

import (
	"context"
	"docker"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/guillermogutierrez/docker-agent/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
)

type Container struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	DockerStatus string `json:"Dockerstatus"`
}

type Service struct {
	Name       string               `json:"name"`
	Image      string               `json:"image"`
	Instances  int                  `json:"instances"`
	Containers map[string]Container `json:"containers"`
}

var ctx context.Context
var cli *client.Client
var services = make(map[string]Service)

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}

func listDeployments(w http.ResponseWriter, r *http.Request) {
	refresh_service_status(&services)
	fmt.Println("Endpoint Hit: listDeployments")
	json.NewEncoder(w).Encode(services)
}

func stopDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["id"]
	fmt.Println("Stopping service " + key)
	stop_service(services[key])
	delete(services, key)
	fmt.Fprintf(w, "Service stopped")
}

func createDeployment(w http.ResponseWriter, r *http.Request) {

	var service Service
	err := json.NewDecoder(r.Body).Decode(&service)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("New service " + service.Name + " " + service.Image)

	services[service.Name] = deploy_service(service.Image, service.Name, service.Instances)
	json.NewEncoder(w).Encode(services)
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

	services[serviceId] = update_service(services[serviceId], instances)
	json.NewEncoder(w).Encode(services)
}

func handleRequests() {
	ctx = context.Background()
	cli, _ = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/", homePage)
	myRouter.HandleFunc("/deployments", listDeployments)
	myRouter.HandleFunc("/deployment", createDeployment).Methods("POST")
	myRouter.HandleFunc("/deployment/{id}/{instances}", updateDeployment).Methods("PUT")
	myRouter.HandleFunc("/deployment/{id}", stopDeployment).Methods("DELETE")

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func update_services_monitor(delay time.Duration) {
	for {
		time.Sleep(time.Duration(delay * time.Millisecond))
		updateStatus(services)
	}
}

func main() {
	fmt.Println(docker.Test())
	go update_services_monitor(10000)
	handleRequests()

}

func find_container_by_id(containerId string) types.Container {
	var filter = filters.NewArgs()
	filter.Add("id", containerId)
	containers, _ := cli.ContainerList(ctx, types.ContainerListOptions{Filters: filter})
	if len(containers) > 0 {
		return containers[0]
	} else {
		return types.Container{}
	}
}

func refresh_service_status(services *map[string]Service) {

	for _, service := range *services {
		for containerId, container := range service.Containers {

			dockerContainer := find_container_by_id(container.Id)

			if dockerContainer.ID != "" {
				container.DockerStatus = dockerContainer.Status
			} else {
				container.DockerStatus = "Not Found"
			}
			service.Containers[containerId] = container
		}
	}
}

func get_random_container_from_service(containers map[string]Container) string {
	keys := reflect.ValueOf(containers).MapKeys()
	return keys[0].Interface().(string)
}

func update_service(service Service, instanceCount int) Service {

	fmt.Println(fmt.Sprint("Update service ", service.Name, " from ", service.Instances, " to ", instanceCount, " instances "))

	pull_image(service.Image)

	for service.Instances > instanceCount {

		containerid := get_random_container_from_service(service.Containers)

		stop_container(service, service.Containers[containerid])
		delete(service.Containers, containerid)
		service.Instances -= 1
	}

	for service.Instances < instanceCount {
		service = start_container(service)
	}

	return service
}

func deploy_service(imageName string, serviceName string, instanceCount int) Service {
	containersAdded := make(map[string]Container)

	pull_image(imageName)

	for index := 0; index < instanceCount; index++ {
		var container = deploy_container(imageName, serviceName)
		containersAdded[container.Id] = container
	}

	return Service{serviceName, imageName, instanceCount, containersAdded}
}

func pull_image(imageName string) {
	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, out)
}

func generate_container_name_by_service(serviceName string) string {
	return fmt.Sprint(serviceName, "_", time.Now().UnixNano())
}

func start_container(service Service) Service {
	var labels = make(map[string]string)
	labels["deployment"] = service.Name

	var container_name = generate_container_name_by_service(service.Name)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: service.Image, Labels: labels}, nil, nil, nil, container_name)
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
	service.Instances += 1

	service.Containers[resp.ID] = Container{resp.ID, container_name, "active", "starting"}

	return service
}

func deploy_container(imageName string, serviceName string) Container {
	var labels = make(map[string]string)
	labels["deployment"] = serviceName

	var container_name = generate_container_name_by_service(serviceName)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName, Labels: labels}, nil, nil, nil, container_name)
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	return Container{resp.ID, serviceName, "active", "starting"}
}

func stop_service(service Service) {

	for _, container := range service.Containers {
		if container.Status == "active" {
			container = stop_container(service, container)
			service.Instances -= 1
		}
	}
}

func stop_container(service Service, container Container) Container {
	fmt.Print("Stopping container ", container.Id, "... ")

	if err := cli.ContainerStop(ctx, container.Id, nil); err != nil {
		panic(err)
	}
	container.Status = "stopped"

	if err := cli.ContainerRemove(ctx, container.Id, types.ContainerRemoveOptions{}); err != nil {
		panic(err)
	}

	fmt.Println("Success")
	return container
}

func updateStatus(services map[string]Service) map[string]Service {
	fmt.Println("\nCheck deployments status")
	for key, service := range services {
		fmt.Println("------------------------------------------------------------------------------------------------------------------------")
		fmt.Println("Deployment name: ", key, fmt.Sprint("\tImage : ", service.Image, "\tRunning : ", service.Instances, " instance(s)"))
		fmt.Println("Container Id\t\t\t\t\t\t\t\tDeployment Status\tDocker Status")
		fmt.Println("........................................................................................................................")

		for _, container := range service.Containers {
			if container.Status == "active" {

				dockerContainer := find_container_by_id(container.Id)

				if dockerContainer.ID == "" {
					fmt.Println(container.Id, "\tDoesn't exist in the docker runtime ")
					service.Instances -= 1
					delete(service.Containers, container.Id)
					service = start_container(service)
				} else {
					fmt.Println(container.Id, "\t", container.Status, "\t\t", dockerContainer.Status)
				}
			}
			services[key] = service
		}
	}
	return services
}
