package main

import (
	"log"
	"os"

	helloworld "temporal-helloworld/workflow"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "temporal-frontend:7233"
	}

	c, err := client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	log.Printf("Connected to Temporal at %s", hostPort)
	log.Println("Starting hello-world worker on task queue 'hello-world-task-queue'...")

	w := worker.New(c, "hello-world-task-queue", worker.Options{})
	w.RegisterWorkflow(helloworld.HelloWorldWorkflow)
	w.RegisterActivity(helloworld.HelloWorldActivity)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker stopped with error: %v", err)
	}
}
