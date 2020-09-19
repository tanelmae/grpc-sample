package db

import (
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/tanelmae/grpc-sample/pb"
)

const (
	SimpleDateFormat = "2006-01-02"
	MaxRating        = 5
)

type ServiceDB interface {
	Close()
	DailyScores(from time.Time, to time.Time) ([]*pb.PeriodScore, error)
	WeeklyScores(from time.Time, to time.Time) ([]*pb.PeriodScore, error)
	RatingCounts(from time.Time, to time.Time) ([]*pb.CategoryCount, error)
	TicketScores(from time.Time, to time.Time) ([]*pb.TicketScore, error)
	OveralScore(from time.Time, to time.Time) (int32, error)
	PeriodOverPeriod(
		firstFrom time.Time, firstTo time.Time,
		secondFrom time.Time, secondTo time.Time) ([]*pb.CategoryDiff, error)
	RatingCategories() ([]string, error)
}
