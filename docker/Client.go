package docker

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

var Services = make(map[string]Service)

func Test() string {
	return "Hi"
}
