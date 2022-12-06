package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"math/rand"
	"net/http"
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
	time.Sleep(time.Duration(n/10) * time.Second)
}
func sendValue(to, round int, vals []float64) {
	simulateDelay()
	for {
		roundMutex.RLock()
		toRound := rounds[to]
		roundMutex.RUnlock()
		//send the value to the node
		if toRound == round {
			channel := allChans[to]
			channel <- vals
			break
		}
	}
}
func findConsensus(id, round, N, f, maxRounds int, xy []float64, failed bool) {
	start := time.Now()
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
		log.Printf("Node %d has failed in round %d.\n", id, round)
		failMutex.Lock()
		failures += 1
		output = append(output, []float64{float64(id), sumX, sumY, float64(round), 1, float64(end) / 1000000})
		outputMutex.Unlock()
		failMutex.Unlock()
		return
	}
	output = append(output, []float64{float64(id), sumX, sumY, float64(round), 0, float64(end) / 1000000})
	outputMutex.Unlock()
	if round < maxRounds {
		findConsensus(id, round+1, N, f, maxRounds, []float64{sumX, sumY}, false)
	}
	//findConsensus(id, round+1, N, f, maxRounds, []float64{sumX, sumY}, false)

}
func makeChannel(i, cap int) {
	receiver := make(chan []float64, cap)
	allChans[i] = receiver
}
func main() {
	lambda.Start(handler)
}

//func helper()

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	//response := events.APIGatewayProxyResponse{
	//	StatusCode: http.StatusOK,
	//	Headers: map[string]string{
	//		"Access-Control-Allow-Origin":  "*",
	//		"Access-Control-Allow-Methods": "POST,GET,OPTIONS",
	//		"Access-Control-Allow-Headers": "Content-Type",
	//	},
	//	Body: "hello world",
	//}
	//
	//return response, nil
	log.Println(request.Body)
	var input Input
	err := json.Unmarshal([]byte(request.Body), &input)
	if err != nil {
		log.Println("Cannot unmarshal")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
		}, nil
	}
	for i, val := range input.Values {
		output = append(output, []float64{float64(i), val[0], val[1], 0, 0, 0})
		makeChannel(i, len(input.Values)-input.F)
		rounds[i] = 1
	}

	for i, value := range input.Values {
		go findConsensus(i, 1, len(input.Values), input.F, input.R, value, false)
	}

	time.Sleep(time.Millisecond * 12000)
	//sort.Slice(output, func(i, j int) bool {
	//	return output[i][3] < output[j][3]
	//})
	responseBody := ResponseBody{
		Output: output,
	}

	jbytes, err := json.Marshal(responseBody)
	if err != nil {
		log.Println("Cannot marshal")
		return events.APIGatewayProxyResponse{}, err
	}
	str := fmt.Sprintf("%s", jbytes)
	response := events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST,GET,OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
		Body: str,
	}
	log.Println(response)
	//return response, nil

	return response, nil
}

type Input struct {
	F      int         `json:"F"`
	R      int         `json:"R"`
	Values [][]float64 `json:"Values"`
}
type ResponseBody struct {
	Output [][]float64 `json:"output"`
}
