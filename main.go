package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/alexbednarczyk/golang-webhook/dispatcher"
)

const port = ":8090"
const messageChannelBufferSize = 1000

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := &http.Server{Addr: port, Handler: http.DefaultServeMux}
	wg := sync.WaitGroup{}
	newDistinationChannel := make(chan []byte, messageChannelBufferSize)
	dispatchChannel := make(chan []byte, messageChannelBufferSize)

	// webhook registration handler
	http.HandleFunc("/webhooks", func(resp http.ResponseWriter, req *http.Request) {
		dec := json.NewDecoder(req.Body)
		var wr dispatcher.WebhookRequest
		err := dec.Decode(&wr)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		out, err := json.Marshal(wr)
		if err != nil {
			panic(err)
		}
		newDistinationChannel <- out
	})

	// start dispatching webhooks
	webhookDispatcher := &dispatcher.Dispatcher{
		Client:       &http.Client{},
		Destinations: make(map[string]string),
		MU:           &sync.Mutex{},
	}

	// TODO: add ctx for cancel
	wg.Add(2)
	go webhookDispatcher.Start(ctx, &wg, newDistinationChannel, dispatchChannel)
	fmt.Printf("Create webhooks on http://localhost%s/webhooks \n", port)
	go userInput(cancel, &wg, dispatchChannel)

	// starting server
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("listen: %s\n", err)
		}
	}()

	wg.Wait()

	if err := srv.Shutdown(ctx); err != nil {
		// handle err
	}

	fmt.Printf("Exited %s.\n", "golang-webhook")
}

func userInput(cancel context.CancelFunc, wg *sync.WaitGroup, dispatchChannel chan []byte) {
	defer wg.Done()

	fmt.Println(`Type a message and press enter.
This message should appear in any other chat instances connected to the same
database.
Type "exit" to quit.`)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		if msg == "exit" {
			cancel()
			break
		}

		dispatchChannel <- []byte(msg)
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Error scanning from stdin:", err)
			break
		}
	}
}
