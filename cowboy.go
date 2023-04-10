package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var serverHost = "server"
var serverPort = "8080"
var hostname string

var filePath string = "/config/config.json"

type Cowboy struct {
	Name    string `json:"name"`
	Health  int    `json:"health"`
	Damage  int    `json:"damage"`
	IsAlive bool   `json:"is_alive"`
	Winner  string `json:"winner"`
}

type Start struct {
	Start bool `json:"start"`
}

func getCowboy(myself, targetCowboy *Cowboy) (*Cowboy, error) {
	// make request to server to get a random cowboy
	url := fmt.Sprintf("http://%s:%s/cowboys?name=%s", serverHost, serverPort, myself.Name)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Errorf("Cowboy: Error making request: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	// read response from server
	body, err := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &targetCowboy)
	if err != nil {
		fmt.Errorf("Cowboy: Error during unmarshal of JSON: %s", err)
		return nil, err
	}

	if targetCowboy.Winner == "true" {
		log.Printf("Cowboy: Shootout is over...")
	}

	if strings.Contains(myself.Name, targetCowboy.Name) {
		log.Printf("Cowboy: I %s am the winner!! My stats: %s \n", targetCowboy.Name, targetCowboy)
	} else {
		log.Printf("Cowboy: Received cowboy %s to shoot.", *&targetCowboy.Name)
	}

	return targetCowboy, nil
}

func checkWinner() (bool, Cowboy, error) {
	// check winner from server
	var winner Cowboy
	url := fmt.Sprintf("http://%s:%s/winner", serverHost, serverPort)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Errorf("Cowboy: Error making request: %s", err)
		return false, winner, err
	}
	defer resp.Body.Close()
	// read response from server
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &winner)
	if err != nil {
		fmt.Errorf("Cowboy: Error unmarshaling JSON %s", err)
		return false, winner, err
	}
	if winner.Winner == "true" {
		return true, winner, nil
	}
	return false, winner, nil
}

func shootCowboy(myself, targetCowboy *Cowboy) (*Cowboy, error) {
	log.Printf("Cowboy: I, %s, just shot %s", myself.Name, targetCowboy.Name)
	targetCowboy.Health -= myself.Damage
	if targetCowboy.Health <= 0 {
		// declare target dead
		targetCowboy.Health = 0
		targetCowboy.IsAlive = false
		log.Printf("Cowboy: I, %s, killed %s. Target is dead.", myself.Name, targetCowboy.Name)
	}
	return targetCowboy, nil
}

func sendUpdateToServer(targetCowboy *Cowboy) (*Cowboy, error) {
	// Marshall JSON
	data, err := json.Marshal(&targetCowboy)
	if err != nil {
		fmt.Errorf("Cowboy: Error during json marshalling of target cowboy")
		return nil, err
	}
	postBody := []byte(data)
	// send update to server with POST request of target cowboy
	url := fmt.Sprintf("http://%s:%s/update", serverHost, serverPort)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(postBody))
	if err != nil {
		fmt.Errorf("Cowboy: Error making request: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	// read response from server
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Errorf("Cowboy: Error reading the request body: %s", err)
		return nil, err
	}
	return targetCowboy, nil
}

func setup(myself *Cowboy) error {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	fileBytes, _ := ioutil.ReadAll(jsonFile)
	var cowboys []Cowboy
	err = json.Unmarshal(fileBytes, &cowboys)
	if err != nil {
		return err
	}
	for _, cb := range cowboys {
		if strings.Contains(myself.Name, cb.Name) {
			myself.Health = cb.Health
			myself.Damage = cb.Damage
			myself.IsAlive = cb.IsAlive
		}
	}
	return nil
}

func register(myself *Cowboy) error {
	// make request to server to register myself
	url := fmt.Sprintf("http://%s:%s/register", serverHost, serverPort)
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		fmt.Errorf("Cowboy: Error making request: %s", err)
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Errorf("Cowboy: Something broke when receiving my registration: %s", err)
		return err
	}
	err = json.Unmarshal(body, &myself)
	if err != nil {
		fmt.Errorf("Cowboy: Error during unmarshal: registration unsuccessful: %s", err)
		return err
	}
	log.Printf("Cowboy: I, %s, have registered successfuly... waiting for signal to shoot...", myself.Name)
	return nil
}

func start() error {
	for {
		// check with server to see if shooting can start
		url := fmt.Sprintf("http://%s:%s/start", serverHost, serverPort)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Errorf("Cowboy: Error making request: %s", err)
			return err
		}
		defer resp.Body.Close()
		var startRound Start
		// read response from server
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Errorf("Cowboy: Something broke when receiving json %s", err)
			return err
		}
		err = json.Unmarshal(body, &startRound)
		if err != nil {
			fmt.Errorf("Cowboy: Error during unmarshal: %s", err)
			return err
		}
		if startRound.Start {
			break
		}
	}
	return nil
}

func main() {
	var myself Cowboy

	for {
		// Register myself with the server and get an identity
		err := register(&myself)
		if err != nil {
			fmt.Errorf("Cowboy: Error in registration. Trying again.")
			time.Sleep(time.Second)
		} else {
			// registered
			break
		}
	}

	// Set myself up using my obtained identity
	// Cowboy reads own stats from config.json based on name obtained from server
	err := setup(&myself)
	if err != nil {
		fmt.Errorf("Cowboy: Error setting up cowboy: %s", err)
	}

	// Check with server if shootout can start
	start()

	for {
		// Shooting runs every one second
		time.Sleep(time.Second)

		// Reset targetCowboy to get a new one
		var targetCowboy Cowboy
		targetCowboy.Name = ""
		targetCowboy.IsAlive = true
		targetCowboy.Health = -1
		targetCowboy.Damage = -1

		// If there are no winners, keep going
		won, winner, err := checkWinner()
		if err != nil {
			fmt.Errorf("Cowboy: Error checking winner: %s", err)
		}
		if won {
			if strings.Contains(myself.Name, winner.Name) {
				log.Printf("Cowboy: Oh yeahh I, %s, won the shootout", myself.Name)
			} else {
				log.Printf("Cowboy: Oh nooo, I'm dead.")
			}
			break
		}

		// Request a new cowboy
		_, err = getCowboy(&myself, &targetCowboy)
		if err != nil {
			fmt.Errorf("Cowboy: Error getting cowboy: %s", err)
			break
		}

		_, err = shootCowboy(&myself, &targetCowboy)
		if err != nil {
			fmt.Errorf("Cowboy: Error shooting cowboy: %s", err)
		}

		_, err = sendUpdateToServer(&targetCowboy)
		if err != nil {
			fmt.Errorf("Cowboy: Error sending update to server: %s", err)
		}

	}
}
