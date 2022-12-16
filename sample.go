package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

//custom map
type Map struct {
	syncMap sync.Map
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

//functions for our custom map
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

//rounds is a regular map that will be accessed using mutexes because it doesnt
//fall under sync.Map's use cases
var failures = 0
var output = make([][]float64, 0)

//mutex to protect the number of failures and the output
var failMutex = &sync.RWMutex{}
var outputMutex = &sync.RWMutex{}

//nodes is a custom struct of type Map which makes it have channels as values
var nodes Map

//channel iterface to store it inside of a sync.Map
type channel chan interface{}

//initalize each node's channel and entries in both maps
func createNode(i int, waitsFor int) {
	//make a channel and store it in the nodes map
	dataChannel := make(channel, waitsFor)
	nodes.Store(i, dataChannel)
}

//simulate delay with rand
func simulateDelay() {
	n := rand.Float64()
	time.Sleep(time.Duration(n/10) * time.Second)
}

func sendValue(self int, to int, round int, val []float64) {

	//sending to self has already been done in findConsensus()
	if self == to {
		return
	}
	//if we are sending to anyone else, we must simulate delay
	simulateDelay()
	//sleeping for any amount of time helps this function not halt, not sure why

	channel := nodes.Load(to)
	val = append(val, float64(round), float64(self))
	channel <- val

	//fmt.Printf("Message successfully sent from %d to %d for round %d\n", self, to, round)

}
func receiveValues(id int, c chan interface{}, m map[int][][]float64, mutex *sync.RWMutex) {
	counter := 0
	for {
		newVal := <-c
		fmt.Println(newVal)
		x, _ := getFloatSwitchOnly(newVal.([]float64)[0])
		y, _ := getFloatSwitchOnly(newVal.([]float64)[1])
		round, _ := getFloatSwitchOnly(newVal.([]float64)[2])
		mutex.Lock()

		if _, prs := m[int(round)]; prs {
			m[int(round)] = append(m[int(round)], []float64{x, y})
		} else {
			m[int(round)] = [][]float64{{x, y}}
		}
		fmt.Println(counter)
		counter += 1
		mutex.Unlock()
	}
}
func findAverage(l [][]float64, stop int) (x, y float64) {
	var sumX, sumY float64
	for j := 0; j < stop; j++ {
		sumX += l[j][0]
		sumY += l[j][1]
	}
	return sumX / float64(stop), sumY / float64(stop)
}
func Shuffle(slice []int) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for len(slice) > 0 {
		n := len(slice)
		randIndex := r.Intn(n)
		slice[n-1], slice[randIndex] = slice[randIndex], slice[n-1]
		slice = slice[:n-1]
	}
}
func findConsensus(id, N, f, r, maxRounds int, initVal []float64, start time.Time) {
	m := make(map[int][][]float64)
	self := nodes.Load(id)
	mutex := &sync.RWMutex{}
	go receiveValues(id, self, m, mutex)
	for i := 2; i <= 1+maxRounds; i++ {
		initVal = append(initVal, float64(i), float64(id))

		self <- initVal
		slice := make([]int, 0)
		for j := 0; j < N; j++ {
			slice = append(slice, j)
		}
		Shuffle(slice)
		for j := 0; j < N; j++ {
			go sendValue(id, slice[j], i, initVal)
		}
		var x, y float64
		for {
			mutex.RLock()
			if len(m[i]) >= N-f {
				mutex.RUnlock()
				x, y = findAverage(m[i], N-f)
				break
			}
			mutex.RUnlock()

		}
		outputMutex.Lock()
		failMutex.Lock()
		if rand.Float64() > 0.8 && failures < f {
			output = append(output, []float64{float64(id), x, y, float64(i), 1, float64(time.Since(start) / 100000)})
			failures += 1
			outputMutex.Unlock()
			failMutex.Unlock()
			break
		} else {
			output = append(output, []float64{float64(id), x, y, float64(i), 0, float64(time.Since(start) / 100000)})
		}
		failMutex.Unlock()
		outputMutex.Unlock()
		initVal = []float64{x, y}
	}
}

func main() {
	//TODO: ERROR CHECKING

	rand.Seed(time.Now().UnixNano())

	//iterate through all the values to initialize the nodes
	in := "{\n    \"F\":2,\n    \"Values\": [[20,20],[20,40],[40,40],[40,20], [20,20],[20,40],[40,40],[40,20]],\n    \"R\": 2\n}"
	var input Input
	err := json.Unmarshal([]byte(in), &input)
	if err != nil {
		fmt.Println(err.Error())
	}
	for i, _ := range input.Values {
		createNode(i, len(input.Values)-input.F)
	}
	//iterate through all the values since now that the nodes have been created,
	//approx consensus can begin
	for i, val := range input.Values {
		//go findConsensus(i, len(input.Values), input.F, 0, input.R, val, time.Now())
		//output = append(output, []float64{float64(i), x, y, float64(i), 0, float64(time.Since(start) / 100000)})
		if rand.Float64() > 0.8 && failures < input.F {
			failMutex.Lock()
			outputMutex.Lock()
			output = append(output, []float64{float64(i), val[0], val[1], 1, 1, 0})
			outputMutex.Unlock()

			failures += 1
			failMutex.Unlock()

		} else {
			outputMutex.Lock()
			output = append(output, []float64{float64(i), val[0], val[1], 1, 0, 0})
			outputMutex.Unlock()
			go findConsensus(i, len(input.Values), input.F, 1, input.R, val, time.Now())
		}
	}

	time.Sleep(1 * time.Second)
	failMutex.RLock()
	failMutex.RUnlock()
	fmt.Println(output)
}

type Input struct {
	F      int         `json:"F"`
	R      int         `json:"R"`
	Values [][]float64 `json:"Values"`
}
type ResponseBody struct {
	Output [][]float64 `json:"output"`
}
