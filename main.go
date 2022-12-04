package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"net/http"
	"sync"
	"time"
)

var roundMutex = &sync.RWMutex{}
var outputMutex = &sync.RWMutex{}

var allChans = make(map[int]chan []float64)
var output = make([][]float64, 0)
var rounds = make(map[int]int)

func simulateDelay() {

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
func findConsensus(id, round, N, f int, xy []float64, failed bool) {
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

	outputMutex.Lock()
	output = append(output, []float64{float64(id), sumX, sumY, float64(round), float64(0)})
	outputMutex.Unlock()

	findConsensus(id, round+1, N, f, []float64{sumX, sumY}, false)
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
	for i, _ := range input.Values {
		makeChannel(i, len(input.Values)-input.F)
	}
	for i, value := range input.Values {
		go findConsensus(i, 0, len(input.Values), input.F, value, false)
	}

	time.Sleep(time.Millisecond * 5000)
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
	F      int         `json:"f"`
	Values [][]float64 `json:"values"`
}
type ResponseBody struct {
	Output [][]float64 `json:"output"`
}
