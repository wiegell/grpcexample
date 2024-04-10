package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
)

func TestBlockingCall(t *testing.T) {
	// Output PID
	pid := os.Getpid()
	fmt.Printf("Current process ID: %d\n", pid)

	// Init
	ts := Setup(t, false)
	commonClient := SetupESDBtestingClient(t, ts.MappedPort.Port())
	_ = commonClient
	var successfulSubscriptionCounter int32

	t.Run("DoesNotTimeOut", func(t *testing.T) {
		var workerGroup sync.WaitGroup
		fatalCh := make(chan string)
		doneCh := make(chan bool)

		// Spawn 5 routines subscribing to data
		for i := 0; i < 5; i++ {
			workerGroup.Add(1)
			loopI := i
			workerClient := SetupESDBtestingClient(t, ts.MappedPort.Port())
			_ = workerClient

			go func() {
				defer workerGroup.Done()
				// Listen for data 100 times
				// If a single worker is used this will fail at 100, but not 99
				for j := 0; j <= 99; j++ {

					ctx, cancel := context.WithCancel(context.Background())
					_ = cancel
					subscriptionHasBeenSetupCh := make(chan error)

					go func() {
						// If the common client is used (quickly climbing to 100 connections), then the test fails:
						// uncomment this to see the test fail:
						_, err := commonClient.SubscribeToStream(ctx, "somestream", esdb.SubscribeToStreamOptions{From: esdb.End{}})

						// However if each worker has its own client 100+ connections are allowed (multiple TCP connections)
						// uncomment this to see the test pass:
						// _, err := workerClient.SubscribeToStream(ctx, "somestream", esdb.SubscribeToStreamOptions{From: esdb.End{}})

						subscriptionHasBeenSetupCh <- err
					}()

					select {
					case <-time.After(1000 * time.Millisecond):
						fatalCh <- "timeout at worker: " + strconv.Itoa(
							loopI) + ", iteration: " + strconv.Itoa(j) + ", global counter: " + strconv.Itoa(int(successfulSubscriptionCounter))
					case err := <-subscriptionHasBeenSetupCh:
						if err != nil {
							fatalCh <- "error setting up subscription: " + err.Error()
						}
						atomic.AddInt32(&successfulSubscriptionCounter, 1)
						// If the context is cancelled the test passes (subscriptions not kept open)
						// uncomment this to see the test pass:
						// cancel()
					}
				}
			}()
		}

		// Register when the worker group is done
		go func() {
			workerGroup.Wait()
			doneCh <- true // Signal that the WaitGroup is done
		}()

		// Listen for done or fatal
	Forloop:
		for {
			select {
			case fatal := <-fatalCh:
				t.Fatal(fatal)
			case <-doneCh:
				break Forloop
			}
		}

		fmt.Printf("Test passed with global counter at: %v", successfulSubscriptionCounter)
	})
}
