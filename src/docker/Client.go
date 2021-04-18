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

type Deployment struct {
	Name       string               `json:"name"`
	Image      string               `json:"image"`
	Instances  int                  `json:"instances"`
	Containers map[string]Container `json:"containers"`
}

var ctx context.Context
var cli *client.Client

var stopInProgress bool

var deployments = make(map[string]Deployment)

func init() {
	ctx = context.Background()
	cli, _ = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	stopInProgress = false
}

func GetDeployments() map[string]Deployment {
	return deployments
}

func DeployDeployment(imageName string, deploymentName string, instanceCount int) {
	deployments[deploymentName] = deployDeployment(imageName, deploymentName, instanceCount)
}

func GetDeploymentsStatus() map[string]Deployment {
	refreshDeploymentStatus()
	return GetDeployments()
}

func UpdateDeployment(deploymentName string, instances int) {
	deployments[deploymentName] = updateDeployment(deployments[deploymentName], instances)
}

func StopDeployment(deploymentName string) {
	stopDeployment(deployments[deploymentName])
	delete(deployments, deploymentName)
}

func UpdateDeploymentsMonitor(delay time.Duration) {
	for {
		time.Sleep(time.Duration(delay * time.Millisecond))
		updateStatus(deployments)
	}
}

func refreshDeploymentStatus() {

	for _, deployment := range deployments {
		for containerId, container := range deployment.Containers {

			dockerContainer := findContainerById(container.Id)

			if dockerContainer.ID != "" {
				container.DockerStatus = dockerContainer.Status
			} else {
				container.DockerStatus = "Not Found"
			}
			deployment.Containers[containerId] = container
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

func getRandomContainerFromDeployment(containers map[string]Container) string {
	keys := reflect.ValueOf(containers).MapKeys()
	return keys[0].Interface().(string)
}

func updateDeployment(deployment Deployment, instanceCount int) Deployment {

	fmt.Println(fmt.Sprint("Updating deployment ", deployment.Name, " from ", deployment.Instances, " to ", instanceCount, " instances"))

	pullImage(deployment.Image)

	for deployment.Instances > instanceCount {

		containerid := getRandomContainerFromDeployment(deployment.Containers)

		stopContainer(deployment, deployment.Containers[containerid])
		delete(deployment.Containers, containerid)
		deployment.Instances -= 1
	}

	for deployment.Instances < instanceCount {
		deployment = startContainer(deployment)
	}

	fmt.Println(fmt.Sprint("Deployment ", deployment.Name, " updated from ", deployment.Instances, " to ", instanceCount, " instances"))

	return deployment
}

func deployDeployment(imageName string, deploymentName string, instanceCount int) Deployment {
	containersAdded := make(map[string]Container)

	pullImage(imageName)

	for index := 0; index < instanceCount; index++ {
		var container = deployContainer(imageName, deploymentName)
		containersAdded[container.Id] = container
	}

	return Deployment{deploymentName, imageName, instanceCount, containersAdded}
}

func pullImage(imageName string) {
	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, out)
}

func generateContainerNameByDeployment(deploymentName string) string {
	return fmt.Sprint(deploymentName, "_", time.Now().UnixNano())
}

func startContainer(deployment Deployment) Deployment {
	var labels = make(map[string]string)
	labels["deployment"] = deployment.Name

	var container_name = generateContainerNameByDeployment(deployment.Name)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: deployment.Image, Labels: labels}, nil, nil, nil, container_name)
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
	deployment.Instances += 1

	deployment.Containers[resp.ID] = Container{resp.ID, container_name, "active", "starting"}

	return deployment
}

func deployContainer(imageName string, deploymentName string) Container {
	var labels = make(map[string]string)
	labels["deployment"] = deploymentName

	var container_name = generateContainerNameByDeployment(deploymentName)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName, Labels: labels}, nil, nil, nil, container_name)
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	return Container{resp.ID, deploymentName, "active", "starting"}
}

func stopDeployment(deployment Deployment) {
	for _, container := range deployment.Containers {
		if container.Status == "active" {
			container = stopContainer(deployment, container)
			deployment.Instances -= 1
		}
	}
}

func stopContainer(deployment Deployment, container Container) Container {
	stopInProgress = true
	fmt.Println("Stopping container ", container.Id, "... ")

	if err := cli.ContainerStop(ctx, container.Id, nil); err != nil {
		panic(err)
	}
	container.Status = "stopped"

	if err := cli.ContainerRemove(ctx, container.Id, types.ContainerRemoveOptions{}); err != nil {
		panic(err)
	}

	fmt.Println("Container ", container.Id, " stopped")
	stopInProgress = false
	return container
}

func updateStatus(deployments map[string]Deployment) map[string]Deployment {
	if !stopInProgress {
		fmt.Println("\nDeployments status")
		for key, deployment := range deployments {
			fmt.Println("------------------------------------------------------------------------------------------------------------------------")
			fmt.Println("Deployment name: ", key, fmt.Sprint("\tImage : ", deployment.Image, "\tRunning : ", deployment.Instances, " instance(s)"))
			fmt.Println("Container Id\t\t\t\t\t\t\t\tDeployment Status\tDocker Status")
			fmt.Println("........................................................................................................................")

			for _, container := range deployment.Containers {
				if container.Status == "active" {

					dockerContainer := findContainerById(container.Id)

					if dockerContainer.ID == "" {
						fmt.Println(container.Id, "\tDoesn't exist in the docker runtime ")
						deployment.Instances -= 1
						delete(deployment.Containers, container.Id)
						deployment = startContainer(deployment)
					} else {
						fmt.Println(container.Id, "\t", container.Status, "\t\t", dockerContainer.Status)
					}
				}
				deployments[key] = deployment
			}
		}
	} else {
		fmt.Println("\nCheck deployments status on hold due to stop container action taking place")
	}
	return deployments
}
