# Distributed Cowboys

This is an implementation of a asynchronous, distributed system written in Golang where cowboys (individual pods) "shoot" at each other at random every second until only one survives. 

## Design

### Config
The system is designed with a Client-Server architecture where the [`server`](server.go) does the heavy lifting of updating "state" of the cowboys (clients). The initial state of the cowboys is passed as a configMap [`config.json`](k8s/config.yaml) into the cluster and mounted as a volume on each pod (including the cowboys). The config dictates how many cowboys are expected in the shootout. 


### Register
The [`cowboys`](cowboy.go) are completely idempotent and have no state until they register themselves with the server. As part of registration they're given a name by the server and they then obtain their state from the config using the given name. As a result, each pod is idempotent and any pod can get any name depending on how fast they register, thus making it easy for pods to join at any point.

Example of registration on the server
```
2023/04/10 21:45:53
Server: Registered [cowboy1] as ready.
2023/04/10 21:46:05
Server: Registered [cowboy1 cowboy2] as ready.
```

### Shootout
Once all expected cowboys are registered with the server the shootout begins. Each cowboy requests a target to shoot at from the server. A cowboy does not shoot at dead cowboys and does not shoot itself. After each shot the cowboy reduces the health of the target, checks whether it is dead or not, and reports it back to the server which then updates the status of the cowboy. As each cowboy repeats the cycle after every second and each cowboy pod can have different timings, all processes run asynchronously. Once a shootout is over, the winning cowboy and server both declare the winner and a new shootout can begin.

```
Server: Winner of the shootout is cowboy3

2023/04/10 21:46:51
Server: All cowboys expected for a new shootout: [{cowboy1 10 2 true } {cowboy2 10 3 true } {cowboy3 10 1 true } {cowboy4 10 2 true } {cowboy5 10 1 true }]
2023/04/10 21:46:51
Server: Now waiting for registrations.
```

### Mutex, Iris, and Other Thoughts
Two of the main features that this project relies on are Mutex and [`Iris framework`](https://www.iris-go.com/).I used Iris because I'd read it's the fastest web framework in Go at the moment. I also like its MVC approach to building APIs and its easy routing system. I used Mutex to preserve consistency as there are concurrent reads and writes happening on multiple objects that are maintained by the server. For example, the `cowboys` slice maintains the health updates of all the cowboys and is updated after each shot.

 All said, this was my first time writing a non-trivial application in Golang and as a result I took this opportunity to try some things I had been reading about before. Though it may not be the cleanest code, I actually really enjoyed Golang after I got a hang of it.

### Logs

Unfortunately I did not have time to implement a log collector but here are some logs that show what's happening on the server and on the cowboy pods.

```
# Server
2023/04/10 19:31:45 Server: cowboy1 got shot. Updating health and status of cowboy1.

---

# Cowboy
2023/04/10 21:03:14 Cowboy: I, cowboy4, killed cowboy2. Target is dead.
2023/04/10 21:03:15 Cowboy: Oh yeahh I, cowboy4, won the shootout
```

## Setup

To run the program the following tools are required to be installed on your machine.
- Docker or Docker Desktop
- Kind K8s cluster (minikube will work too)

Get the repo and cd into cowboys folder
```
git clone git@github.com:aykhazanchi/distributed-cowboys.git
cd distributed-cowboys
```

Create Kind cluster
```
kind create cluster --name dev
kubectl create namespace cowboys
kubectl config set-context --current --namespace=cowboys
```

Build the Docker images
```
docker build -t server:1.0 -f Dockerfile.server . 
docker build -t cowboy:1.0 -f Dockerfile.cowboy .
```

Load the Docker images into Kind
```
kind load docker-image server:1.0 cowboy:1.0 --name dev
```

In two separate terminals run the following to tail the logs (they go really fast)
```
# Server logs
kubectl logs -f `kubectl get pods | grep server | awk '{print $1}'`

#Cowboy logs
watch -n 1 kubectl logs --selector app=cowboy
```

Apply the manifests to start the shootout on the cluster
```
kubectl apply -f k8s/config.yaml,k8s/server.yaml,k8s/cowboys.yaml
```

## Cleanup

```
# Delete the K8s resources
kubectl delete -f k8s/cowboys.yaml,k8s/server.yaml,k8s/config.yaml

# Delete the cluster
kind delete cluster -n dev

# Delete the Docker images
docker rmi server:1.0 && docker rmi cowboy:1.0
```