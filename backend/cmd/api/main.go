package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
)

// newRouter builds and returns the application router.
func newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", handler.HealthCheck)
	return r
}

func main() {
	r := newRouter()

	// Detect Lambda environment by checking for AWS_LAMBDA_FUNCTION_NAME.
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		adapter := chiadapter.New(r)
		lambda.Start(adapter.ProxyWithContext)
	} else {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		addr := fmt.Sprintf(":%s", port)
		log.Printf("starting local server on %s", addr)
		if err := http.ListenAndServe(addr, r); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}
}
