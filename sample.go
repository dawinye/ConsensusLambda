package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"math/rand"
	"sync"
	"time"
)

var roundMutex = &sync.RWMutex{}
var outputMutex = &sync.RWMutex{}
var failMutex = &sync.RWMutex{}

var allChans = make(map[int]chan []float64)
var output = make([][]float64, 0)
var rounds = make(map[int]int)
var failures = 0

func simulateDelay() {
	n := rand.Float64()
	time.Sleep(time.Duration(n) * time.Second)
}
func sendValue(to, round int, vals []float64) {
	simulateDelay()
	//fmt.Printf("Started sending value %v to %d\n", vals, to)
	for {
		roundMutex.RLock()
		toRound := rounds[to]
		roundMutex.RUnlock()
		//send the value to the node
		if toRound == round {
			//fmt.Printf("Sending value %v to %d\n", vals, to)
			channel := allChans[to]
			channel <- vals
			break
		}
	}
}
func findConsensus(id, round, N, f, maxRounds int, xy []float64, failed bool) {
	start := time.Now()
	//fmt.Printf("consensus started for id: %d, round %d\n", id, round)
	self := allChans[id]
	self <- xy
	for j := 0; j < N; j++ {
		if id == j {
			continue
		}
		go sendValue(j, round, xy)
	}
	var sumX, sumY float64
	for {
		if len(self) == cap(self) {
			for j := 0; j < N-f; j++ {
				//if there are N-f messages in the channel, go through the channel
				//convert the items into floats and add them to the sum
				newVal := <-self
				sumX += newVal[0]
				sumY += newVal[1]
			}
			total := float64(N - f)
			//find the average
			sumX /= total
			sumY /= total
			break
		}
	}
	roundMutex.Lock()
	rounds[id] += 1
	roundMutex.Unlock()
	end := time.Since(start)
	outputMutex.Lock()
	if rand.Float64() > 0.8 && failures < f {
		fmt.Printf("Node %d has failed in round %d.\n", id, round)
		failMutex.Lock()
		failures += 1
		output = append(output, []float64{float64(id), sumX, sumY, float64(round), 1, float64(end) / 1000000})
		outputMutex.Unlock()
		failMutex.Unlock()
		return
	}
	fmt.Printf("Round %d for node %d took %s\n", round, id, end)
	output = append(output, []float64{float64(id), sumX, sumY, float64(round), 0, float64(end) / 1000000})
	outputMutex.Unlock()
	//fmt.Printf("consensus finished for id: %d, round %d, time: %s\n", id, round, end)
	if round < maxRounds {
		findConsensus(id, round+1, N, f, maxRounds, []float64{sumX, sumY}, false)
	}
	//findConsensus(id, round+1, N, f, []float64{sumX, sumY}, false)
}
func makeChannel(i, cap int) {
	receiver := make(chan []float64, cap)
	allChans[i] = receiver
}

type Input struct {
	F      int         `json:"f"`
	R      int         `json:"R"`
	Values [][]float64 `json:"values"`
}
type ResponseBody struct {
	Output [][]float64 `json:"output"`
}

func main() {
	string := "{\n    \"F\":3,\n    \"Values\": [[1,98],[2,1001],[1,98],[2,1001],[1,98],[2,1001],[1,98],[2,1001]],\n    \"R\": 25\n}"
	rand.Seed(0)
	var input Input
	err := json.Unmarshal([]byte(string), &input)
	if err != nil {

		fmt.Printf(err.Error())
	}
	for i, val := range input.Values {
		output = append(output, []float64{float64(i), val[0], val[1], 0, float64(0)})
		makeChannel(i, len(input.Values)-input.F)
		rounds[i] = 1
	}

	for i, value := range input.Values {
		go findConsensus(i, 1, len(input.Values), input.F, input.R, value, false)
	}

	time.Sleep(time.Millisecond * 5000)
	responseBody := ResponseBody{
		Output: output,
	}
	jbytes, err := json.Marshal(responseBody)
	if err != nil {
		println(err.Error())
	}
	str1 := fmt.Sprintf("%s", jbytes)
	response := events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       str1,
	}

	fmt.Println(response)
}
