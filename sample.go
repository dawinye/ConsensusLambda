package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"sync"
	"time"
)

var roundMutex = &sync.RWMutex{}
var outputMutex = &sync.RWMutex{}

var allChans = make(map[int]chan []float64)
var output = make([][]float64, 5)
var rounds = make(map[int]int)

//func simulateDelay() {
//
//}
func sendValue(to, round int, vals []float64) {
	//simulateDelay()
	fmt.Printf("Started sending value %v to %d\n", vals, to)
	for {
		roundMutex.RLock()
		toRound := rounds[to]
		roundMutex.RUnlock()
		//send the value to the node
		if toRound == round {
			fmt.Printf("Sending value %v to %d\n", vals, to)
			channel := allChans[to]
			channel <- vals
			break
		}
	}
}
func findConsensus(id, round, N, f int, xy []float64, failed bool) {
	fmt.Printf("consensus started for id: %d, round %d\n", id, round)
	self := allChans[id]
	fmt.Println(self)

	self <- xy
	fmt.Println("here")
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

	outputMutex.Lock()
	output = append(output, []float64{float64(id), sumX, sumY, float64(round), float64(0)})
	outputMutex.Unlock()
	fmt.Printf("consensus finished for id: %d, round %d\n", id, round)
	findConsensus(id, round+1, N, f, []float64{sumX, sumY}, false)
}
func makeChannel(i, cap int) {
	receiver := make(chan []float64, cap)
	allChans[i] = receiver
}

type Input struct {
	F      int         `json:"f"`
	Values [][]float64 `json:"values"`
}
type ResponseBody struct {
	Output [][]float64 `json:"output"`
}

func main() {
	string := "{\n    \"F\": 4,\n    \"Values\": [[1,98],[2,1001],[1,98],[2,1001],[1,98],[2,1001],[1,98],[2,1001],[1,98],[2,1001],[1,98],[2,1001]]\n}"

	var input Input
	err := json.Unmarshal([]byte(string), &input)
	if err != nil {
		fmt.Printf(err.Error())
	}
	//values := make([][]float64, 2)
	//values[0] = []float64{1, 98}
	//values[1] = []float64{98, 1}

	for i, _ := range input.Values {
		makeChannel(i, 2)
		rounds[i] = 0
	}
	fmt.Println(rounds)
	for i, value := range input.Values {
		go findConsensus(i, 0, 2, 0, value, false)
	}

	time.Sleep(time.Millisecond * 1000)
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
