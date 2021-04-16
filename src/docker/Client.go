package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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

func init() {
	ctx = context.Background()
	cli, _ = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

func GetServices() map[string]Service {
	return services
}

func DeployService(imageName string, serviceName string, instanceCount int) {
	services[serviceName] = deployService(imageName, serviceName, instanceCount)
}

func GetServicesStatus() map[string]Service {
	refreshServiceStatus()
	return GetServices()
}

func UpdateService(serviceName string, instances int) {
	services[serviceName] = updateService(services[serviceName], instances)
}

func StopService(serviceName string) {
	stopService(services[serviceName])
	delete(services, serviceName)
}

func UpdateServicesMonitor(delay time.Duration) {
	for {
		time.Sleep(time.Duration(delay * time.Millisecond))
		updateStatus(services)
	}
}

func refreshServiceStatus() {

	for _, service := range services {
		for containerId, container := range service.Containers {

			dockerContainer := findContainerById(container.Id)

			if dockerContainer.ID != "" {
				container.DockerStatus = dockerContainer.Status
			} else {
				container.DockerStatus = "Not Found"
			}
			service.Containers[containerId] = container
		}
	}
}

func findContainerById(containerId string) types.Container {
	var filter = filters.NewArgs()
	filter.Add("id", containerId)
	containers, _ := cli.ContainerList(ctx, types.ContainerListOptions{Filters: filter})
	if len(containers) > 0 {
		return containers[0]
	} else {
		return types.Container{}
	}
}

func getRandomContainerFromService(containers map[string]Container) string {
	keys := reflect.ValueOf(containers).MapKeys()
	return keys[0].Interface().(string)
}

func updateService(service Service, instanceCount int) Service {

	fmt.Println(fmt.Sprint("Update service ", service.Name, " from ", service.Instances, " to ", instanceCount, " instances "))

	pullImage(service.Image)

	for service.Instances > instanceCount {

		containerid := getRandomContainerFromService(service.Containers)

		stopContainer(service, service.Containers[containerid])
		delete(service.Containers, containerid)
		service.Instances -= 1
	}

	for service.Instances < instanceCount {
		service = startContainer(service)
	}

	return service
}

func deployService(imageName string, serviceName string, instanceCount int) Service {
	containersAdded := make(map[string]Container)

	pullImage(imageName)

	for index := 0; index < instanceCount; index++ {
		var container = deployContainer(imageName, serviceName)
		containersAdded[container.Id] = container
	}

	return Service{serviceName, imageName, instanceCount, containersAdded}
}

func pullImage(imageName string) {
	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, out)
}

func generateContainerNameByService(serviceName string) string {
	return fmt.Sprint(serviceName, "_", time.Now().UnixNano())
}

func startContainer(service Service) Service {
	var labels = make(map[string]string)
	labels["deployment"] = service.Name

	var container_name = generateContainerNameByService(service.Name)

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

func deployContainer(imageName string, serviceName string) Container {
	var labels = make(map[string]string)
	labels["deployment"] = serviceName

	var container_name = generateContainerNameByService(serviceName)

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

func stopService(service Service) {
	for _, container := range service.Containers {
		if container.Status == "active" {
			container = stopContainer(service, container)
			service.Instances -= 1
		}
	}
}

func stopContainer(service Service, container Container) Container {
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

				dockerContainer := findContainerById(container.Id)

				if dockerContainer.ID == "" {
					fmt.Println(container.Id, "\tDoesn't exist in the docker runtime ")
					service.Instances -= 1
					delete(service.Containers, container.Id)
					service = startContainer(service)
				} else {
					fmt.Println(container.Id, "\t", container.Status, "\t\t", dockerContainer.Status)
				}
			}
			services[key] = service
		}
	}
	return services
}
