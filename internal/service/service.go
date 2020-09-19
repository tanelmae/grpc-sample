package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tanelmae/grpc-sample/internal/db"
	"github.com/tanelmae/grpc-sample/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc/health/grpc_health_v1"
)

/*
	Configure with options
*/

func New(logger *zap.Logger, db db.ServiceDB) Service {
	return Service{
		log: logger,
		db:  db,
	}
}

type Service struct {
	log *zap.Logger
	db  db.ServiceDB
}

func (s *Service) Run(grpcAddress, httpAddress, apiDocsPath string) {
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		s.log.Info("Failed to listen", zap.String("address", grpcAddress))
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor))
	pb.RegisterTicketServiceServer(grpcServer, s)
	grpc_prometheus.Register(grpcServer)

	grpc_health_v1.RegisterHealthServer(grpcServer, s)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer close(stop)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			s.log.Fatal("grpc server failure",
				zap.Error(err))
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/docs", http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		http.ServeFile(res, req, filepath.Join(apiDocsPath, "/index.html"))
	}))
	mux.Handle("/proto", http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		http.ServeFile(res, req, filepath.Join(apiDocsPath, "/service.proto"))
	}))

	srv := http.Server{Addr: httpAddress, Handler: mux}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Fatal("HTTP server failure",
				zap.Error(err))
		}
	}()

	s.log.Info("service started")
	sig := <-stop
	s.log.Info("signal received", zap.String("signal", sig.String()))

	_ = srv.Shutdown(context.Background())
	grpcServer.GracefulStop()
	s.db.Close()

	s.log.Info("service stopped")
}

func (s *Service) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// Should check something meaningful here
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Service) Watch(req *grpc_health_v1.HealthCheckRequest, ws grpc_health_v1.Health_WatchServer) error {
	// Should check something meaningful here
	return nil
}

/*
Aggregated category scores over a period of time
E.g. what have the daily ticket scores been for a past week or what were the scores between 1st and 31st of January.
For periods longer than one month weekly aggregates should be returned instead of daily values.
*/
func (s *Service) CategoryScores(ctx context.Context, in *pb.TimePeriod) (*pb.CategoryScoresOut, error) {
	startTime := in.From.AsTime()
	endTime := in.To.AsTime()
	s.log.Info("category scores",
		zap.String("from", startTime.String()),
		zap.String("to", endTime.String()),
	)

	var err error
	out := pb.CategoryScoresOut{}

	if endTime.After(startTime.AddDate(0, 1, 0)) {
		out.Period = pb.CategoryScoresOut_WEEK
		out.Scores, err = s.db.WeeklyScores(startTime, endTime)
		if err != nil {
			s.log.Error("DB error", zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to read weekly scores from DB")
		}
	} else {
		out.Period = pb.CategoryScoresOut_DAY
		out.Scores, err = s.db.DailyScores(startTime, endTime)
		if err != nil {
			s.log.Error("DB error", zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to read daily scores from DB")
		}
	}

	out.Counts, err = s.db.RatingCounts(startTime, endTime)
	if err != nil {
		s.log.Error("DB error", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to read rating counts from DB")
	}
	return &out, nil
}

/*
Scores by ticket. Aggregate scores for categories within defined period by ticket.
E.g. what aggregate category scores tickets have within defined rating time range have.
*/
func (s *Service) TicketScores(ctx context.Context, in *pb.TimePeriod) (*pb.TicketScoresOut, error) {
	from := in.From.AsTime()
	to := in.To.AsTime()
	s.log.Info("ticket scores",
		zap.String("from", from.Format(time.RFC3339)),
		zap.String("to", to.Format(time.RFC3339)),
	)

	var err error
	out := pb.TicketScoresOut{}

	out.Scores, err = s.db.TicketScores(from, to)
	if err != nil {
		s.log.Error("DB error", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to read tickets score from the database")
	}

	out.Categories, err = s.db.RatingCategories()
	if err != nil {
		s.log.Error("DB error", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to read categories from the database")
	}

	return &out, nil
}

/*
Overal quality score. What is the overall aggregate score for a period.
E.g. the overall score over past week has been 96%.
*/
func (s *Service) OveralScore(ctx context.Context, in *pb.TimePeriod) (*pb.OveralScoreOut, error) {
	from := in.From.AsTime()
	to := in.To.AsTime()
	s.log.Info("overal scores",
		zap.String("from", from.Format(time.RFC3339)),
		zap.String("to", to.Format(time.RFC3339)),
	)

	var err error
	out := pb.OveralScoreOut{}

	out.Score, err = s.db.OveralScore(from, to)
	if err != nil {
		s.log.Error("DB error", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to read overall score from the database")
	}

	return &out, nil
}

/*
Period over Period score change. What has been the change from selected period over previous period.
E.g. current week vs. previous week or December vs. January change in percentages.
*/
func (s *Service) PeriodOverPeriod(ctx context.Context, in *pb.TimePeriods) (*pb.PeriodOverPeriodOut, error) {
	firstFrom := in.First.From.AsTime()
	firstTo := in.First.To.AsTime()
	secondFrom := in.Second.From.AsTime()
	secondTo := in.Second.To.AsTime()

	s.log.Info("period over period",
		zap.String("first period",
			fmt.Sprintf("%s - %s", firstFrom.Format(time.RFC3339), firstTo.Format(time.RFC3339))),
		zap.String("second period",
			fmt.Sprintf("%s - %s", secondFrom.Format(time.RFC3339), secondTo.Format(time.RFC3339))),
	)

	var err error
	out := pb.PeriodOverPeriodOut{}
	out.Changes, err = s.db.PeriodOverPeriod(
		firstFrom, firstTo, secondFrom, secondTo,
	)

	if err != nil {
		s.log.Error("DB error", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to read period scores from the database")
	}

	return &out, nil
}
