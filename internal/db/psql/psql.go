package psql

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/tanelmae/grpc-sample/internal/db"
	"github.com/tanelmae/grpc-sample/pb"
)

func New(host, user, password, dbname string, port int) (db.ServiceDB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err := sqlx.Connect("postgres", psqlInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open sqlite DB")

	}

	return &psqlDB{
		db: db,
	}, nil
}

type psqlDB struct {
	db *sqlx.DB
}

func (svc *psqlDB) Close() {
	svc.db.Close()
}

func (svc *psqlDB) DailyScores(start, end time.Time) ([]*pb.PeriodScore, error) {
	ratings := []*pb.PeriodScore{}
	err := svc.db.Select(&ratings,
		`SELECT rating_categories.id, rating_categories.name,
		to_char(tickets.created_at, 'YYYY-MM-DD') as period,
		round(AVG((rating * weight)+ rating)/AVG(($1::int * weight) + $1)*100) as score
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
		WHERE tickets.created_at BETWEEN $2 AND $3
		GROUP BY period, name, rating_categories.id
		ORDER BY period, rating_categories.id ASC;`,
		db.MaxRating, start.Format(db.SimpleDateFormat), end.Format(db.SimpleDateFormat))

	if err != nil {
		return ratings, err
	}
	return ratings, nil
}

func (svc *psqlDB) WeeklyScores(start, end time.Time) ([]*pb.PeriodScore, error) {
	ratings := []*pb.PeriodScore{}
	err := svc.db.Select(&ratings,
		`SELECT rating_categories.id, rating_categories.name,
		to_char(tickets.created_at, 'YYYY WW') as period,
		round(AVG((rating * weight)+rating)/AVG(($1::int * weight)+$1)*100) as score
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
		WHERE tickets.created_at BETWEEN $2 AND $3
		GROUP BY period, name, rating_categories.id
		ORDER BY period, rating_categories.id ASC;`,
		db.MaxRating, start.Format(db.SimpleDateFormat), end.Format(db.SimpleDateFormat))

	if err != nil {
		return ratings, err
	}

	return ratings, nil
}

func (svc *psqlDB) RatingCounts(start, end time.Time) ([]*pb.CategoryCount, error) {
	counts := []*pb.CategoryCount{}
	err := svc.db.Select(&counts,
		`SELECT rating_categories.id, rating_categories.name,
		count(rating_category_id) as count
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
		WHERE tickets.created_at BETWEEN $1 AND $2
		GROUP BY rating_categories.id, name;`,
		start.Format(db.SimpleDateFormat), end.Format(db.SimpleDateFormat))

	if err != nil {
		return nil, err
	}

	return counts, nil
}

/*
Aggregate scores for categories within defined period by ticket.
E.g. what aggregate category scores tickets have within defined rating time range have.
*/
func (svc *psqlDB) TicketScores(from time.Time, to time.Time) ([]*pb.TicketScore, error) {
	scores := []*pb.TicketScore{}
	err := svc.db.Select(&scores,
		`SELECT ticket_id, rating_categories.name,
		round(AVG((rating * weight) + rating)/AVG(($1 * weight) + $1)*100) as score
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
				WHERE tickets.created_at BETWEEN $2 AND $3
		GROUP BY ticket_id, name;`,
		db.MaxRating, from.Format(db.SimpleDateFormat), to.Format(db.SimpleDateFormat))

	if err != nil {
		return nil, err
	}

	return scores, nil
}

func (svc *psqlDB) RatingCategories() ([]string, error) {
	categories := []string{}
	err := svc.db.Select(&categories,
		`SELECT name FROM rating_categories;`)

	if err != nil {
		return nil, err
	}

	return categories, nil
}

/*
Overal quality score

What is the overall aggregate score for a period.
E.g. the overall score over past week has been 96%.
*/
func (svc *psqlDB) OveralScore(from time.Time, to time.Time) (int32, error) {
	var score int32
	err := svc.db.Get(&score,
		`SELECT round(AVG((rating * weight) + rating)/AVG(($1 * weight) + $1)*100) as score
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
		WHERE tickets.created_at BETWEEN $2 AND $3;`,
		db.MaxRating, from.Format(db.SimpleDateFormat), to.Format(db.SimpleDateFormat))

	if err != nil {
		return score, err
	}
	return score, err
}

/*
Period over Period score change

What has been the change from selected period over previous period.
E.g. current week vs. previous week or December vs. January change in percentages.
*/
func (svc *psqlDB) PeriodOverPeriod(
	firstFrom time.Time, firstTo time.Time,
	secondFrom time.Time, secondTo time.Time) ([]*pb.CategoryDiff, error) {

	out := []*pb.CategoryDiff{}
	err := svc.db.Select(&out,
		`SELECT first.id, first.name, (second.score-first.score) as diff
		FROM (SELECT rating_categories.id as id, rating_categories.name as name,
			round(AVG((rating * weight) + rating)/AVG(($1 * weight)+$1)*100) as score
			FROM ratings
			INNER JOIN tickets ON ratings.ticket_id=tickets.id
			INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
			WHERE tickets.created_at BETWEEN $2 AND $3
			GROUP BY rating_categories.id) as first
		INNER JOIN (SELECT rating_categories.id as id_2, rating_categories.name as name,
			round(AVG((rating * weight) + rating)/AVG(($1 * weight)+$1)*100) as score
			FROM ratings
			INNER JOIN tickets ON ratings.ticket_id=tickets.id
			INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
			WHERE tickets.created_at BETWEEN $4 AND $5
			GROUP BY rating_categories.id) AS second ON id = id_2;`,
		db.MaxRating, firstFrom.Format(db.SimpleDateFormat), firstTo.Format(db.SimpleDateFormat),
		secondFrom.Format(db.SimpleDateFormat), secondTo.Format(db.SimpleDateFormat))
	if err != nil {
		return out, err
	}
	return out, nil
}
