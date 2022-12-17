# ConsensusLambda

This repo is meant to create a compressed golang file that can uploaded to AWS Lambda and return an output for a 2-D approximate consensus algorithm. 
sample.go and main.go are essentially the same file, except sample.go is meant to be run locally whereas it is harder to test main.go without making HTTP 
requests. To run sample.go, enter `go run sample.go` into the terminal (note that the inputs for values, f and r are defined in sample.go rather than command line inputs).
If you wish to make updates to main.go, enter `GOARCH=amd64 GOOS=linux go build main.go` to build it so that it can run on AWS Lambda, and then compress it 
into a .zip file. Then, navigate to the AWS Lambda console and upload the .zip, making sure the entry point for your lambda is set to main. The free tier of AWS
allows for up to 3008 MB of memory, and changes to the runtime of the program to make it longer than 15 seconds should be accompanied by changing the timeout
limit (which is 15 seconds by default). 

##Sources
I used the getFloatSwitchOnly() function from this StackExchange post https://stackoverflow.com/questions/20767724/converting-unknown-interface-to-float64-in-golang
