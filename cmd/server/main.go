package main

import (
	"fmt"

	"github.com/namsral/flag"

	"github.com/tanelmae/grpc-sample/internal/db"
	"github.com/tanelmae/grpc-sample/internal/db/psql"
	"github.com/tanelmae/grpc-sample/internal/db/sqlite"
	"github.com/tanelmae/grpc-sample/internal/service"
	"go.uber.org/zap"
)

func main() {
	dbFile := flag.String("db-file", "database.db", "Path to SQLite file path")
	grpcPort := flag.Int("grpc-port", 8080, "Service port to listen for GRPC requests")
	httpPort := flag.Int("http-port", 8081, "Service port to listen for HTTP requests")
	docsPath := flag.String("docs-path", "./pb", "Documentation html and proto definition directory")
	dbHost := flag.String("db-host", "localhost", "PostgreSQL address")
	dbPort := flag.Int("db-port", 5432, "PostgreSQL port")
	dbName := flag.String("db-name", "", "PostgreSQL database name")
	dbUser := flag.String("db-user", "", "PostgreSQL user")
	dbPassword := flag.String("db-password", "", "PostgreSQL password")
	flag.Parse()

	zap.NewDevelopmentConfig()
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	var svcDB db.ServiceDB
	if *dbUser != "" &&
		*dbPassword != "" &&
		*dbName != "" {

		svcDB, err = psql.New(*dbHost, *dbUser, *dbPassword, *dbName, *dbPort)
		if err != nil {
			logger.Fatal("failed to connect to PostgreSQL", zap.Error(err))
		}
	} else {
		svcDB, err = sqlite.New(*dbFile)
		if err != nil {
			logger.Fatal("failed to open database", zap.Error(err))
		}
	}

	s := service.New(logger, svcDB)
	s.Run(
		fmt.Sprintf(":%d", *grpcPort),
		fmt.Sprintf(":%d", *httpPort),
		*docsPath,
	)
	_ = logger.Sync()
}
