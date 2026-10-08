//go:generate go run ./cmd/reghook/reghook.go

package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"golang.org/x/sync/errgroup"

	"github.com/ExplorViz/metrics-service/internal/catalog"
	"github.com/ExplorViz/metrics-service/internal/values"
)

//go:embed resources/banner.txt
var banner string

func main() {
	fs := ff.NewFlagSet("metrics-service")
	var (
		httpPort   = fs.Int('p', "port", 8082, "port to listen on for incoming HTTP requests")
		dbHostAddr = fs.String('a', "db-addr", "localhost:19000", "network endpoint at which the Clickhouse database runs")
		dbName     = fs.StringLong("db-name", "default", "name of the Clickhouse database to use")
		dbUser     = fs.String('u', "db-user", "default", "username to use with the Clickhouse instance")
		dbPass     = fs.String('P', "db-pass", "", "password to use with the Clickhouse instance (insecure, prefer using env var)")
		logLevel   = fs.StringEnum('l', "log-level", "log level: info, error, debug", "info", "error", "debug")
	)

	if err := ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("EXPLORVIZ")); err != nil {
		fmt.Println(err)
		fmt.Printf("%s\n", ffhelp.Flags(fs))
		os.Exit(0)
	}

	switch *logLevel {
	case "info":
		slog.SetLogLoggerLevel(slog.LevelInfo)
	case "error":
		slog.SetLogLoggerLevel(slog.LevelError)
	case "debug":
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	conn, err := dbConnect(*dbHostAddr, *dbName, *dbUser, *dbPass)
	if err != nil {
		slog.Error("failed to establish database connection", "error", err, "hostAddress", *dbHostAddr)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	catalogRepo := catalog.Repository{Conn: conn}
	catalogHandler := catalog.NewHandler(catalogRepo)
	catalogHandler.Register(mux)

	valuesRepo := values.Repository{Conn: conn}
	valuesHandler := values.NewHandler(valuesRepo)
	valuesHandler.Register(mux)

	srv := &http.Server{Addr: ":" + strconv.Itoa(*httpPort), Handler: corsHandler(addContentTypeJSON(mux))}

	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(srv.ListenAndServe)

	go func() {
		sigs := make(chan os.Signal, 2)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

		<-sigs
		slog.Info("received interrupt signal; gracefully stopping ...")
		if err := srv.Shutdown(ctx); err != nil && err != ctx.Err() {
			slog.Warn("error occurred during server shutdown", "error", err)
		}
		cancel()

		<-sigs
		slog.Info("received second interrupt signal; exiting immediately")
		os.Exit(1)
	}()

	fmt.Print(banner)

	if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
		slog.Error("unexpected server shutdown", "error", err)
	}
}

func dbConnect(hostAddr string, dbName string, user string, pass string) (driver.Conn, error) {
	var (
		ctx       = context.Background()
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr: []string{hostAddr},
			Auth: clickhouse.Auth{
				Database: dbName,
				Username: user,
				Password: pass,
			},
		})
	)

	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		}
		return nil, err
	}
	return conn, nil
}

func addContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func corsHandler(next http.Handler) http.Handler {
	allowedOrigins := map[string]struct{}{
		"http://localhost:4200":                                 {},
		"http://localhost:8080":                                 {},
		"https://demo.explorviz.uni-kiel.de":                    {},
		"https://explorviz.sustainkieker.kieker-monitoring.net": {},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
