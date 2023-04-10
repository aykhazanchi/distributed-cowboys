package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"

	"github.com/kataras/iris/v12"
	"golang.org/x/exp/slices"
)

var filePath string = "/config/config.json"

type Cowboy struct {
	Name    string `json:"name"`
	Health  int    `json:"health"`
	Damage  int    `json:"damage"`
	IsAlive bool   `json:"is_alive"`
	Winner  string `json:"winner"`
}

var winner Cowboy
var cowboys []Cowboy
var cowboysMutex sync.RWMutex
var fileMutex sync.RWMutex

type Start struct {
	Start bool `json:"start"`
}

var startShooting bool = false

var registered []string
var registerMutex sync.RWMutex

// Initialize the expected number of cowboys
func readCowboysFromFile() error {
	fileMutex.Lock()
	defer fileMutex.Unlock()
	jsonFile, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Server: Error reading the config file", err)
	}
	fileBytes, _ := ioutil.ReadAll(jsonFile)

	err = json.Unmarshal(fileBytes, &cowboys)
	if err != nil {
		log.Fatalf("Server: Error reading the config file", err)
	}
	log.Printf("\nServer: All cowboys expected for a new shootout: %v", cowboys)
	log.Printf("\nServer: Now waiting for registrations.")
	return nil
}

// function to send a random cowboy to requester
// the random cowboy sent cannot be a dead cowboy or the same cowboy as the requester
func getRandomCowboy(name string) (target Cowboy, err error) {
	cowboysMutex.RLock()
	defer cowboysMutex.RUnlock()
	var targetCowboys []Cowboy
	for i := range cowboys {
		if cowboys[i].IsAlive && cowboys[i].Name != name {
			targetCowboys = append(targetCowboys, cowboys[i])
		}
	}
	if len(targetCowboys) == 0 {
		log.Printf("\nServer: All cowboys except one are dead.")
		// find only alive cowboy
		for i := range cowboys {
			if cowboys[i].IsAlive {
				targetCowboys = append(targetCowboys, cowboys[i])
			}
		}
		log.Printf("\nServer: Cowboy %v has won the shootout. ", targetCowboys[0].Name)
		startShooting = false
		return targetCowboys[0], nil
	} else {
		target = targetCowboys[rand.Intn(len(targetCowboys))]
		return target, nil
	}
}

// Handler function for returning a target cowboy to a requester
func getCowboyHandler(ctx iris.Context) {
	if !startShooting {
		ctx.JSON(winner)
	}
	name := ctx.URLParam("name")
	if name == "" {
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("'name' query parameter is required")
		return
	}
	cowboy, err := getRandomCowboy(name)
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString(err.Error())
		return
	}
	ctx.JSON(cowboy)
}

// Handler function for receiving a cowboy after its shot
// Parses the body to get the cowboy and then calls handleShot()
func handleShotHandler(ctx iris.Context) {
	var cb Cowboy
	err := ctx.ReadBody(&cb)
	if err != nil {
		ctx.StopWithProblem(iris.StatusBadRequest,
			iris.NewProblem().Title("Parser issue").Detail(err.Error()))
		return
	}
	log.Printf("Server: %v got shot. Updating health and status of %v.", cb.Name, cb.Name)
	err = handleShot(&cb)
	if err != nil {
		fmt.Errorf("Server: Ran into an error handling the shot: ", err)
		ctx.StatusCode(http.StatusInternalServerError)
	}
	ctx.StatusCode(http.StatusOK)
}

func handleShot(receivedCowboy *Cowboy) error {
	// here a cowboy is received and the server is responsible for updating its health and status
	cowboysMutex.RLock()
	defer cowboysMutex.RUnlock()
	var stillAlives []Cowboy
	for i := range cowboys {
		if receivedCowboy.Name == cowboys[i].Name {
			cowboys[i].Health = receivedCowboy.Health
			cowboys[i].IsAlive = receivedCowboy.IsAlive
		}
		// To save iterations we check in the same loop if any are alive
		if cowboys[i].IsAlive {
			stillAlives = append(stillAlives, cowboys[i])
		}
	}
	// If only one is alive means the shootout is over
	if len(stillAlives) == 1 {
		startShooting = false
		winner = stillAlives[0]
		winner.Winner = "true"
		log.Printf("\nServer: Winner of the shootout is %v \n\n\n", winner.Name)
		// reset the shootout so the rounds can restart
		registered = nil
		readCowboysFromFile()
	}

	//updateCowboys()

	return nil
}

// Function to write updated cowboy values back to the persistent store (file)
// This is no longer needed as the shootout resets itself now
// Leaving it here for reference
func updateCowboys() error {
	fileMutex.Lock()
	defer fileMutex.Unlock()
	data, err := json.Marshal(cowboys)
	if err != nil {
		return err
	}
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// Handler to check if winner is declared
func checkWinnerHandler(ctx iris.Context) {
	if !startShooting {
		// if startShooting is back to "false" means a winner is declared
		// Return winner
		ctx.JSON(winner)
	} else {
		// No winner yet, keep shooting
		ctx.JSON(iris.Map{"winner": "false"})
	}
}

// Handler to handle registrations for new cowboys joining shootout
func registerHandler(ctx iris.Context) {
	registerMutex.Lock()
	defer registerMutex.Unlock()

	// We want to assign an identity to each cowboy that registers
	for i := range cowboys {
		// Iterate through cowboys and assign a new identity to each client checking in
		// If an identity is assigned it's added to "registered" slice
		// If identity already in registered slice, it cannot be assigned, find another one
		if !slices.Contains(registered, cowboys[i].Name) {
			registered = append(registered, cowboys[i].Name)
			log.Printf("\nServer: Registered %v as ready.", registered)
			ctx.JSON(cowboys[i])
			break
		}
	}

	// When all have registered, the lengths of expected cowboys and registered are equal
	// Begin shootout at that point
	// until then the server waits and they keep checking every second
	if len(registered) == len(cowboys) {
		startShooting = true
	}
}

// Function to allow cowboys to check if shooting can begin
// Returns false always until all cowboys are registered
func startHandler(ctx iris.Context) {
	var startRound Start
	startRound.Start = startShooting

	ctx.JSON(startRound)
}

func main() {
	err := readCowboysFromFile()
	if err != nil {
		log.Fatalf("Server: Error reading cowboys from file: %v", err)
	}

	app := iris.Default()
	app.Get("/register", registerHandler)
	app.Get("/start", startHandler)
	app.Get("/cowboys", getCowboyHandler)
	app.Get("/winner", checkWinnerHandler)
	app.Post("/update", handleShotHandler)
	app.Run(iris.Addr(":8080"))
}
