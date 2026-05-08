package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/awsclient"
	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/queue"
	"github.com/eriksteenman/reign-game/backend/internal/replenish"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/worker"
)

// newUUIDv4 generates a UUID v4 string using crypto/rand.
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// newRouter builds and returns the application router with all routes mounted
// under the /api prefix. The prefix separates API traffic from SPA routes
// (e.g., the frontend's /admin page vs. the /api/admin/* backend endpoints).
//
// Public /api/* routes run anonymously; admin routes are grouped under
// /api/admin and wrapped in auth.RequireAuth + auth.RequireAdmin so every
// admin route inherits the middleware chain by construction (BM-05). Adding
// a new admin route is a single r.Method(...) call inside the admin group.
func newRouter(repo *repository.PuzzleRepository, pub *queue.Publisher) *chi.Mux {
	// Build the reactive-replenish hook once. nil when SQS is not
	// configured — the drain handlers treat nil as no-op so the player
	// path keeps working in local-dev without LocalStack SQS.
	replenishHook := buildReplenishHook(repo, pub)

	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handler.HealthCheck)
		r.Get("/puzzles/generate", handler.GenerateHandler)

		if repo != nil {
			r.Get("/puzzles/next", handler.ServeHandler(repo, replenishHook))
			r.Put("/puzzles/{id}/status", handler.StatusHandler(repo))

			// Public modes listing for the landing page. Narrower than
			// /admin/pool — no thresholds, ready counts, or maxAttempts.
			r.Get("/config/modes", handler.ConfigModesHandler(repo))

			// Daily challenge routes. Anonymous-or-user via OptionalAuth:
			// the handlers branch on auth.UserIDFromContext to scope
			// per-flow storage (anon device-linked vs. signed-in user).
			// r.Method is used (not r.Get / r.Post) because the daily
			// handler factories return http.Handler, not http.HandlerFunc.
			r.Route("/daily", func(r chi.Router) {
				r.Use(auth.OptionalAuth(auth.NewClerkSessionVerifier()))
				r.Method(http.MethodGet, "/{date}", handler.DailyGetHandler(repo, time.Now, replenishHook))
				r.Method(http.MethodPost, "/{date}/result", handler.DailySubmitHandler(repo, time.Now))
			})

			// Admin routes live behind the Clerk auth middleware chain.
			// Middleware order is (RequireAuth, RequireAdmin) per BM-03 —
			// reversed or missing pieces panic on first admin request so
			// the mistake surfaces immediately in tests.
			r.Route("/admin", func(r chi.Router) {
				r.Use(auth.RequireAuth(auth.NewClerkSessionVerifier()))
				r.Use(auth.RequireAdmin)

				r.Get("/pool", handler.AdminPoolHandler(repo))
				r.Put("/config/{size}/{mode}", handler.UpdateConfigHandler(repo))
				r.Post("/config", handler.CreateConfigHandler(repo))
				r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
				if pub != nil {
					r.Post("/replenish", handler.ReplenishHandler(repo, repo, pub))
				}
			})
		}
	})

	return r
}

// initClerk bootstraps the Clerk SDK for the lifetime of this process.
// Fails hard (log.Fatal) when neither CLERK_SECRET_KEY nor
// CLERK_SECRET_PARAM_NAME is set — see auth.LoadClerkSecret for the
// resolution order. The SDK reads the key from package-level state
// (clerk.SetKey), so this must run before any request handler does.
func initClerk(ctx context.Context) {
	secret, err := auth.LoadClerkSecret(ctx)
	if err != nil {
		log.Fatalf("auth bootstrap: %v", err)
	}
	clerk.SetKey(secret)
	log.Printf("auth bootstrap: clerk SDK initialized")
}

// newSQSClient creates an SQS client, optionally with a custom endpoint
// for local development (e.g., LocalStack).
func newSQSClient(cfg *aws.Config) *sqs.Client {
	endpoint := os.Getenv("SQS_ENDPOINT")
	if endpoint != "" {
		return sqs.NewFromConfig(*cfg, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	return sqs.NewFromConfig(*cfg)
}

func main() {
	ctx := context.Background()
	generatorMode := os.Getenv("GENERATOR_MODE")

	switch {
	case generatorMode == "sqs":
		// SQS consumer mode: generate puzzles from queue messages.
		cfg, err := awsclient.LoadAWSConfig(ctx)
		if err != nil {
			log.Fatalf("failed to load AWS config: %v", err)
		}

		tableName := os.Getenv("PUZZLE_TABLE_NAME")
		if tableName == "" {
			tableName = "puzzle-pool"
		}

		dynamoClient := awsclient.NewDynamoDBClient(&cfg)
		repo := repository.NewPuzzleRepository(dynamoClient, tableName)
		w := worker.NewGeneratorWorker(repo, newUUIDv4)

		if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
			// Lambda environment: start SQS event handler.
			lambda.Start(w.HandleSQSEvent)
		} else {
			// Local dev: start SQS poller.
			queueURL := os.Getenv("SQS_QUEUE_URL")
			if queueURL == "" {
				log.Fatal("SQS_QUEUE_URL is required for local SQS consumer mode")
			}

			sqsClient := newSQSClient(&cfg)

			runLocalPoller(ctx, sqsClient, queueURL, w.HandleSQSEvent)
		}
	case os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "":
		// Lambda API mode: HTTP routes via API Gateway proxy.
		cfg, err := awsclient.LoadAWSConfig(ctx)
		if err != nil {
			log.Fatalf("failed to load AWS config: %v", err)
		}

		tableName := os.Getenv("PUZZLE_TABLE_NAME")
		if tableName == "" {
			tableName = "puzzle-pool"
		}
		queueURL := os.Getenv("SQS_QUEUE_URL")

		dynamoClient := awsclient.NewDynamoDBClient(&cfg)
		repo := repository.NewPuzzleRepository(dynamoClient, tableName)

		var pub *queue.Publisher
		if queueURL != "" {
			sqsClient := newSQSClient(&cfg)
			pub = queue.NewPublisher(sqsClient, queueURL)
		}

		// Clerk must be initialized before the first request arrives,
		// because admin routes invoke the SDK via RequireAuth.
		initClerk(ctx)

		r := newRouter(repo, pub)
		adapter := chiadapter.New(r)
		lambda.Start(adapter.ProxyWithContext)
	default:
		// Local dev HTTP server mode.
		var repo *repository.PuzzleRepository
		var pub *queue.Publisher

		// Set up AWS clients if endpoints are configured.
		dynamoEndpoint := os.Getenv("DYNAMODB_ENDPOINT")
		sqsEndpoint := os.Getenv("SQS_ENDPOINT")

		if dynamoEndpoint != "" || sqsEndpoint != "" {
			cfg, err := awsclient.LoadAWSConfig(ctx)
			if err != nil {
				log.Fatalf("failed to load AWS config: %v", err)
			}

			if dynamoEndpoint != "" {
				tableName := os.Getenv("PUZZLE_TABLE_NAME")
				if tableName == "" {
					tableName = "puzzle-pool"
				}
				dynamoClient := awsclient.NewDynamoDBClient(&cfg)
				repo = repository.NewPuzzleRepository(dynamoClient, tableName)
			}

			queueURL := os.Getenv("SQS_QUEUE_URL")
			if sqsEndpoint != "" && queueURL != "" {
				sqsClient := newSQSClient(&cfg)
				pub = queue.NewPublisher(sqsClient, queueURL)
			}
		}

		// Clerk must be initialized before the first request arrives.
		// Local dev reads CLERK_SECRET_KEY from backend/.env.local; a
		// missing or empty value causes log.Fatal by design (BM-10).
		initClerk(ctx)

		// Warm up the DDB client by issuing one throwaway Query before
		// the HTTP server starts accepting requests. The first SDK
		// call after Clerk init carries a multi-second penalty
		// (transport + connection-pool warm-up + the Clerk wrapping
		// effect documented in loadAWSConfig); paying it here means
		// the first user-facing request hits a warm SDK and lands in
		// tens of milliseconds instead of seconds. Failures are
		// logged WARN and ignored — LocalStack may be slow to start
		// during `task dev:up`, in which case the user will pay the
		// warm-up cost on the first request as before.
		if repo != nil {
			warmStart := time.Now()
			_, warmErr := repo.GetAllConfigs(ctx)
			if warmErr != nil {
				log.Printf("WARN: ddb warm-up failed (will pay cost on first request): err=%v warm_ms=%d", warmErr, time.Since(warmStart).Milliseconds())
			} else {
				log.Printf("ddb warm-up: warm_ms=%d", time.Since(warmStart).Milliseconds())
			}
		}

		r := newRouter(repo, pub)

		port := os.Getenv("PORT")
		if port == "" {
			port = "5181"
		}
		addr := fmt.Sprintf("127.0.0.1:%s", port)
		log.Printf("api: starting local server on %s", addr)
		if err := http.ListenAndServe(addr, r); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}
}

// runLocalPoller wraps signal handling and polling in a separate function so
// that defer cancel() is scoped correctly and does not trigger exitAfterDefer
// lint warnings in main.
func runLocalPoller(ctx context.Context, sqsClient worker.SQSConsumerAPI, queueURL string, handleFn func(context.Context, events.SQSEvent) error) {
	pollerCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	worker.RunLocalPoller(pollerCtx, sqsClient, queueURL, handleFn)
}

// buildReplenishHook constructs the drain-site closure that spawns the
// reactive top-up goroutine. Returns nil when repo or pub is nil so
// local-dev runs without SQS keep working — drain handlers are
// nil-tolerant.
func buildReplenishHook(repo *repository.PuzzleRepository, pub *queue.Publisher) func(size int, mode string) {
	if repo == nil || pub == nil {
		return nil
	}
	return replenish.NewAsyncHook(replenish.ReactiveDeps{
		Configs:   repo,
		Claimer:   repo,
		Publisher: pub,
	}, "reactive replenish")
}
