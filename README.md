# docker-agent

How to build the container

``` sh
cd ./src
docker build -t docker-agent . 
```

How to run the container
``` sh
docker run -it --rm -p 10000:10000 -v /var/run/docker.sock:/var/run/docker.sock docker-agent
```

How to use the service 
docker-agent.json contains a collection of services invocations to be run pon postman
1. Create a new deployment
- POST localhost:10000/deployment
```
body
{
    "name": "deployment2",
    "image": "bfirsh/reticulate-splines",
    "instances": 3
}
```
2. Update an existing deployment
- PUT localhost:10000/deployment/:id/:instances
```
id: name of deplyoment
instances: number of new instances
```
3. List all deployments
- GET localhost:10000/deployments
``` json
{
  "deployment2":{
    "name":"deployment2",
    "image":"bfirsh/reticulate-splines",
    "instances":4,
    "containers":{
        "29306713b325a669cfd2dc4d8c7a749a6895d4d6e28fceb3d77df4c0a441f956":{
          "id":"29306713b325a669cfd2dc4d8c7a749a6895d4d6e28fceb3d77df4c0a441f956",
          "name":"deployment2",
          "status":"active",
          "Dockerstatus":"Up 56 seconds"
        },
        "a00929c54c28b3643772b1a4e2ccb8cedb9d86c0ced9fc08b802ff4f3589e54d":{
          "id":"a00929c54c28b3643772b1a4e2ccb8cedb9d86c0ced9fc08b802ff4f3589e54d",
          "name":"deployment2",
          "status":"active",
          "Dockerstatus":"Up 55 seconds"
        },
        "d07666338fbdb787830aae0a18b6cb8d3515b21301a43680f75fa433c0af5995":{
          "id":"d07666338fbdb787830aae0a18b6cb8d3515b21301a43680f75fa433c0af5995",
          "name":"deployment2",
          "status":"active",
          "Dockerstatus":"Up 55 seconds"
        },
        "e9e0c4004b50ed42f2746e591ffec4f36880b2c7da3ee3d6e361250eb466840f":{
          "id":"e9e0c4004b50ed42f2746e591ffec4f36880b2c7da3ee3d6e361250eb466840f",
          "name":"deployment2_1618587401461175000",
          "status":"active",
          "Dockerstatus":"Up 48 seconds"
        }
    }
  }
}
```
4. Delete a deploymet
- DEL localhost:10000/deployment/:id