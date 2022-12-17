package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

//custom map that allows key-value pairs to be int->channels
type Map struct {
	syncMap sync.Map
}

//standard functions for our custom map
func (m *Map) Load(i int) chan interface{} {
	val, ok := m.syncMap.Load(i)
	if ok {
		return val.(chan interface{})
	} else {
		return nil
	}
}
func (m *Map) Store(i int, value chan interface{}) {
	m.syncMap.Store(i, value)
}
func (m *Map) Exists(i int) bool {
	_, ok := m.syncMap.Load(i)
	return ok
}
func (m *Map) Delete(i int) {
	m.syncMap.Delete(i)
}

//helper function to get values from interface{} to float64s
var errUnexpectedType = errors.New("Non-numeric type could not be converted to float")

func getFloatSwitchOnly(unk interface{}) (float64, error) {
	switch i := unk.(type) {
	case float64:
		return i, nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint:
		return float64(i), nil
	default:
		return math.NaN(), errUnexpectedType
	}
}

//Our implementation forces the number of failures to be capped,
//this variable keeps track of how many nodes have failed
var failures = 0

//output is a slice of slices, where each value represents a single node at one specific round,
//front end will iterate through the output (after it's been transformed into a string to be part of the json)
//and plot each value as a point on the graph
var output = make([][]float64, 0)

//mutex to change the number of failed nodes
var failMutex = &sync.RWMutex{}

//mutex to append to the output
var outputMutex = &sync.RWMutex{}

//nodes is a custom struct of type Map which makes it have channels as values
var nodes Map

//channel iterface to store it inside of a sync.Map
type channel chan interface{}

//initalize each node's channel and store it into map
func createNode(i int, waitsFor int) {
	dataChannel := make(channel, waitsFor)
	nodes.Store(i, dataChannel)
}

//simulate delay with rand
func simulateDelay() {
	n := rand.Float64()
	time.Sleep(time.Duration(n/10) * time.Second)
}

//sending values from node A to node B, regardless of which round each node is on
func sendValue(self int, to int, round int, val []float64) {

	//sending to self has already been done in findConsensus(), so we can return
	if self == to {
		return
	}
	//if we are sending to anyone else, we must simulate delay
	simulateDelay()

	//load the receiving node's channel that we are sending to
	channel := nodes.Load(to)
	val = append(val, float64(round), float64(self))
	channel <- val

	//fmt.Printf("Message successfully sent from %d to %d for round %d\n", self, to, round)

}

//function for receiving values in findConsensus(), reads values from a channel and
//places them into the map based  on which round the message was sent for
func receiveValues(id int, c chan interface{}, m map[int][][]float64, mutex *sync.RWMutex) {
	//infinite for loop to receive values
	for {
		newVal := <-c

		//convert the x,y and round from interface{} to floats
		x, _ := getFloatSwitchOnly(newVal.([]float64)[0])
		y, _ := getFloatSwitchOnly(newVal.([]float64)[1])
		round, _ := getFloatSwitchOnly(newVal.([]float64)[2])

		//lock the mutex to insert this value into the map
		mutex.Lock()

		//if this is the first value of round R in the map, then initialize round R as a key
		if _, prs := m[int(round)]; prs {
			m[int(round)] = append(m[int(round)], []float64{x, y})
			//otherwise, append it to the list for the existing round R
		} else {
			m[int(round)] = [][]float64{{x, y}}
		}
		//unlock the mutex so findConsensus() can read the map
		mutex.Unlock()
	}
}

//shuffles a list so that the order in which a node will send out its value is randomized
//helps to increase variance, because in a for loop from 0...N-1 we saw that most nodes would
//average the values of the first N-f values because those are likely to be the values first received
//because they're the first that are sent
func Shuffle(slice []int) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for len(slice) > 0 {
		n := len(slice)
		randIndex := r.Intn(n)
		slice[n-1], slice[randIndex] = slice[randIndex], slice[n-1]
		slice = slice[:n-1]
	}
}

//find the average of N-f (x,y) coordinate pairs
func findAverage(l [][]float64, stop int) (x, y float64) {
	var sumX, sumY float64
	for j := 0; j < stop; j++ {
		sumX += l[j][0]
		sumY += l[j][1]
	}
	return sumX / float64(stop), sumY / float64(stop)
}

//ran as a goroutine for each node so they all run consensus concurrently
func findConsensus(id, N, f, r, maxRounds int, initVal []float64, start time.Time) {

	//seed the random function because we use randomness to determine if a node fails
	rand.Seed(time.Now().UnixNano())

	//map to track which values this node has received
	m := make(map[int][][]float64)
	//mutex to lock/unlock accesses to the map above
	mutex := &sync.RWMutex{}

	//self is the channel for this node
	self := nodes.Load(id)

	//continuously receiveValues
	go receiveValues(id, self, m, mutex)

	//create the slice to determine the order in which messages are sent to nodes
	slice := make([]int, 0)
	for j := 0; j < N; j++ {
		slice = append(slice, j)
	}

	//initial values are considered round 1, so run consensus for R more rounds
	for i := 2; i <= maxRounds+1; i++ {
		initVal = append(initVal, float64(i), float64(id))
		self <- initVal

		//shuffle so that the order in which messages are sent is randomized for every round
		Shuffle(slice)
		for j := 0; j < N; j++ {
			go sendValue(id, slice[j], i, initVal)
		}

		var x, y float64
		for {
			//lock the map mutex so that it can read if it has received N-f messages for this round
			mutex.RLock()
			if len(m[i]) >= N-f {
				mutex.RUnlock()
				//find the average x and y if it has received N-f messages
				//and then break, continue in for loop if it hasn't
				x, y = findAverage(m[i], N-f)
				break
			}
			mutex.RUnlock()

		}
		//lock the output and fail mutexes because every node runs findConsensus(), and can therefore
		//change the output and number of failures so far
		outputMutex.Lock()
		failMutex.Lock()

		//if there haven't been f failures yet, then there is a 20% chance that this node fails

		if rand.Float64() > 0.8 && failures < f {
			//[node ID, x, y, round, 0/1 depending on failed or not, delay since findConsensus() was ran] is the format for the output
			//find the difference between the current time and when this function was called, convert to ms
			output = append(output, []float64{float64(id), x, y, float64(i), 1, float64(time.Since(start) / 100000)})
			failures += 1
			outputMutex.Unlock()
			failMutex.Unlock()
			//after failing, the for loop breaks so the node stops running future rounds.
			break
		} else {
			output = append(output, []float64{float64(id), x, y, float64(i), 0, float64(time.Since(start) / 100000)})
		}
		//unlock the mutexes
		failMutex.Unlock()
		outputMutex.Unlock()

		//update initVal, which is sent to all nodes, to the new updated values from this round of consensus
		initVal = []float64{x, y}
	}
}

//purpose of main is to run the function handler
func main() {
	lambda.Start(handler)
}

//handler takes in a request and returns an event which is mainly associated with a status
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	//log.Println(request.Body)

	//take the request.body and try to unmarshal it using json to fit the input struct
	var input Input
	err := json.Unmarshal([]byte(request.Body), &input)

	//the only requests that trigger this error from the front end are CORS pre checks, so return
	//a 200 so that the actual request can be processed
	if err != nil {
		log.Println("Cannot unmarshal")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
		}, nil
	}

	//create a node for every value supplied in the input
	for i, _ := range input.Values {
		createNode(i, len(input.Values)-input.F)
	}

	//iterate through all the values since now that the nodes have been created,
	//approx consensus can begin
	for i, val := range input.Values {

		//same option as in findConsensus(), this allows nodes to fail in the first round
		if rand.Float64() > 0.8 && failures < input.F {
			failMutex.Lock()
			outputMutex.Lock()
			//format for output is [node ID, x, y, round, 0/1 depending on failed or not, delay since findConsensus() was ran],
			//in this case since it is the first round, the time elapsed is 0 since it is the first round and the node failed
			output = append(output, []float64{float64(i), val[0], val[1], 1, 1, 0})
			outputMutex.Unlock()

			failures += 1
			failMutex.Unlock()

		} else {
			//if the node doesn't fail, then findConsensus is called and it runs the second round
			outputMutex.Lock()
			output = append(output, []float64{float64(i), val[0], val[1], 1, 0, 0})
			outputMutex.Unlock()
			go findConsensus(i, len(input.Values), input.F, 1, input.R, val, time.Now())
		}
	}

	//sleep for one second to allow for consensus goroutines to finish running,
	//explanation for this approach in final report
	time.Sleep(1 * time.Second)

	//fit our output list into the struct
	responseBody := ResponseBody{
		Output: output,
	}

	jbytes, err := json.Marshal(responseBody)
	if err != nil {
		log.Println("Cannot marshal")
		return events.APIGatewayProxyResponse{}, err
	}

	//convert the marshalled json into a string that can be the body of the response
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

	//clear out failures and output because they are declared at the file level,
	//not the function level, so subsequent lambda calls will use previous various
	output = nil
	failures = 0
	return response, nil
}

//struct for the input
type Input struct {
	F      int         `json:"F"`
	R      int         `json:"R"`
	Values [][]float64 `json:"Values"`
}

//struct for the output
type ResponseBody struct {
	Output [][]float64 `json:"output"`
}
