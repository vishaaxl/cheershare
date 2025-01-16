package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
	"github.com/vishaaxl/cheershare/internal/data"
)

/*
config struct:
- Holds application-wide configuration settings such as:
  - `port`: The port number on which the server will listen.
  - `env`: The current environment (e.g., "development", "production").
  - `db`: Database-specific configurations.
  - `redis`: Redis-specific configurations.
*/
type config struct {
	port  int
	env   string
	db    db
	redis redisConfig
}

type db struct {
	dsn          string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  time.Duration
}

type redisConfig struct {
	addr     string
	password string
	db       int
}

/*
applications struct:
- Encapsulates the application's dependencies, including:
  - `config`: The application's configuration settings.
  - `logger`: A logger instance to handle log messages.
  - `redis`: A Redis client instance for caching.
*/
type application struct {
	wg     sync.WaitGroup
	config config
	models data.Models

	logger *log.Logger
	cache  *redis.Client
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	ensureUploadDirExists()
	/*
	   Configuration includes:
	   - `port`: The port on which the server will run (default is 4000).
	   - `env`: The environment mode (e.g., "development", "production").
	   - `db`: Database connection settings, including the DSN, connection limits, and idle timeout.
	   - `redis`: Redis connection settings, including server address, password, and database index.
	*/
	cfg := &config{
		port: 4000,
		env:  "development",
		db: db{
			dsn:          "user=postgres password=mysecretpassword dbname=cheershare sslmode=disable",
			maxOpenConns: 25,
			maxIdleConns: 25,
			maxIdleTime:  time.Minute,
		},
		redis: redisConfig{
			addr:     "localhost:6379",
			password: "mysecretpassword",
			db:       0,
		},
	}

	/*
	   Logger settings:
	   - Prefix: "INFO\t" indicates informational logs.
	   - Flags: Includes date and time for log entries.
	*/
	logger := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

	/*
	   - connectDB establishes a connection using the database configuration.
	   - If the connection fails, the application logs a fatal error and exits.
	   - If successful, logs a success message and defers closing the connection.
	*/
	db, err := connectDB(cfg.db)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %s", err)
	}
	logger.Println("Connected to PostgreSQL database")
	defer db.Close()

	/*
	   - connectRedis establishes a connection using the Redis configuration.
	   - If the connection fails, the application logs a fatal error and exits.
	   - If successful, logs a success message and defers closing the connection.
	*/
	redisClient, err := connectRedis(cfg.redis)
	if err != nil {
		logger.Fatalf("Failed to connect to Redis: %s", err)
	}
	logger.Println("Connected to Redis server")
	defer redisClient.Close()

	app := &application{
		config: *cfg,
		logger: logger,
		cache:  redisClient,
		models: data.NewModels(db),
	}

	router := httprouter.New()
	router.HandlerFunc(http.MethodPost, "/signup", app.handleUserSignupAndVerification)
	router.HandlerFunc(http.MethodPost, "/upload-creative", app.requireAuthenticatedUser(app.uploadCreativeHandler))

	/*
	   Server configuration:
	   - Address: Uses the configured port from `config`.
	   - Handler: Routes handled by `httprouter`.
	   - Timeouts: Configured for idle, read, and write operations.
	   - Error logging: Logs errors during server startup or operation.
	*/
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.recoverPanic(app.authenticate(router)),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Log server start information
	app.logger.Printf("Starting server on port %d in %s mode", app.config.port, app.config.env)

	// Start the server and handle potential errors
	err = srv.ListenAndServe()
	if err != nil {
		app.logger.Fatalf("Could not start server: %s", err)
	}
}

func connectDB(cfg db) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.dsn)

	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.maxOpenConns)
	db.SetMaxIdleConns(cfg.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.maxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func connectRedis(cfg redisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.addr,
		Password: cfg.password,
		DB:       cfg.db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return client, nil
}
