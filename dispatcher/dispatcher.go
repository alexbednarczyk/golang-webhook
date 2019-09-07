package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// WebhookRequest used to keep track of webhook distinations
type WebhookRequest struct {
	Name        string
	Destination string
}

// Dispatcher used to created a dispatcher for webhook messages
type Dispatcher struct {
	Client       *http.Client
	Destinations map[string]string
	MU           *sync.Mutex
}

// Start is a go routine for handling dispatcher messages
func (d *Dispatcher) Start(ctx context.Context, wg *sync.WaitGroup, newDistinationChannel chan []byte, dispatchChannel chan []byte) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case distination := <-newDistinationChannel:
			fmt.Println("== New Distination Added ==")
			newDistination := WebhookRequest{}
			err := json.Unmarshal(distination, &newDistination)
			if err != nil {
				panic(err)
			}
			d.add(newDistination.Name, newDistination.Destination)
		case msg := <-dispatchChannel:
			fmt.Println("== Message Received ==")
			d.dispatch(msg)
		}
	}
}
func (d *Dispatcher) add(name, destination string) {
	d.MU.Lock()
	d.Destinations[name] = destination
	d.MU.Unlock()
}

func (d *Dispatcher) dispatch(msg []byte) {
	d.MU.Lock()
	defer d.MU.Unlock()

	for user, destination := range d.Destinations {
		go func(user, destination string) {
			req, err := http.NewRequest("POST", destination, bytes.NewBufferString(string(msg)))
			if err != nil {
				// probably don't allow creating invalid destinations
				return
			}
			resp, err := d.Client.Do(req)
			if resp.StatusCode == 500 {
				fmt.Printf("remove: %s from %s\n", user, destination)
				delete(d.Destinations, user)
			}
			if err != nil {
				// should probably check response status code and retry if it's timeout or 500
				return
			}
			fmt.Printf("Webhook to '%s' dispatched, response code: %d \n", destination, resp.StatusCode)
		}(user, destination)
	}
}
