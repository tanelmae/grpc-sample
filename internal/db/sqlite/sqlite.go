package sqlite

import (
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	"github.com/tanelmae/grpc-sample/internal/db"
	"github.com/tanelmae/grpc-sample/pb"
)

func New(dbPath string) (*SQLiteDB, error) {
	sqliteDB, err := sqlx.Open("sqlite3", dbPath)

	if err != nil {
		return nil, errors.Wrap(err, "failed to open sqlite DB")
	}

	return &SQLiteDB{
		db: sqliteDB,
	}, nil
}

type SQLiteDB struct {
	db *sqlx.DB
}

func (sqlite *SQLiteDB) Close() {
	sqlite.db.Close()
}

func (sqlite *SQLiteDB) DailyScores(start, end time.Time) ([]*pb.PeriodScore, error) {
	ratings := []*pb.PeriodScore{}
	err := sqlite.db.Select(&ratings,
		`SELECT rating_categories.id, rating_categories.name,
		strftime('%Y-%m-%d', tickets.created_at) as period,
		round(AVG((rating * weight)+rating)/AVG(($1 * weight)+$1)*100) as score
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
		WHERE tickets.created_at BETWEEN $2 AND $3
		GROUP BY period, name;`,
		db.MaxRating, start.Format(db.SimpleDateFormat), end.Format(db.SimpleDateFormat))

	if err != nil {
		return ratings, err
	}
	return ratings, nil
}

func (sqlite *SQLiteDB) WeeklyScores(start, end time.Time) ([]*pb.PeriodScore, error) {
	ratings := []*pb.PeriodScore{}
	err := sqlite.db.Select(&ratings,
		`SELECT rating_categories.id, rating_categories.name,
		strftime('%W', tickets.created_at) as period,
		round(AVG((rating * weight)+rating)/AVG(($1 * weight)+$1)*100) as score
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
		WHERE tickets.created_at BETWEEN $2 AND $3
		GROUP BY period, name;`, db.MaxRating, start.Format(db.SimpleDateFormat), end.Format(db.SimpleDateFormat))

	if err != nil {
		return ratings, err
	}

	return ratings, nil
}

func (sqlite *SQLiteDB) RatingCounts(start, end time.Time) ([]*pb.CategoryCount, error) {
	counts := []*pb.CategoryCount{}
	err := sqlite.db.Select(&counts,
		`SELECT rating_categories.id, rating_categories.name,
		count(rating_category_id) as count
		FROM ratings
		INNER JOIN tickets ON ratings.ticket_id=tickets.id
		INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
		WHERE tickets.created_at BETWEEN $1 AND $2
		GROUP BY name
		ORDER BY rating_categories.id ASC;`,
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
func (sqlite *SQLiteDB) TicketScores(from time.Time, to time.Time) ([]*pb.TicketScore, error) {
	scores := []*pb.TicketScore{}
	err := sqlite.db.Select(&scores,
		`SELECT ticket_id, rating_categories.name,
		round(AVG((rating * weight)+rating)/AVG(($1 * weight)+$1)*100) as score
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

func (sqlite *SQLiteDB) RatingCategories() ([]string, error) {
	categories := []string{}
	err := sqlite.db.Select(&categories,
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
func (sqlite *SQLiteDB) OveralScore(from time.Time, to time.Time) (int32, error) {
	var score int32
	err := sqlite.db.Get(&score,
		`SELECT round(AVG(rating * weight)/AVG($1 * weight)*100) as score
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
func (sqlite *SQLiteDB) PeriodOverPeriod(
	firstFrom time.Time, firstTo time.Time,
	secondFrom time.Time, secondTo time.Time) ([]*pb.CategoryDiff, error) {

	out := []*pb.CategoryDiff{}
	err := sqlite.db.Select(&out,
		`SELECT id, name, (score_2-score_1) as diff
		FROM (SELECT rating_categories.id as id, rating_categories.name as name,
			ifnull(round(AVG(rating * weight)/AVG($1 * weight)*100),0) as score_1
			FROM ratings
			INNER JOIN tickets ON ratings.ticket_id=tickets.id
			INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
			WHERE tickets.created_at BETWEEN $2 AND $3
			GROUP BY name
			ORDER BY rating_categories.id ASC)
		INNER JOIN (SELECT rating_categories.id as id_2,
			ifnull(round(AVG(rating * weight)/AVG($1 * weight)*100),0) as score_2
			FROM ratings
			INNER JOIN tickets ON ratings.ticket_id=tickets.id
			INNER JOIN rating_categories ON ratings.rating_category_id=rating_categories.id
			WHERE tickets.created_at BETWEEN $4 AND $5
			GROUP BY name
			ORDER BY rating_categories.id ASC) ON id = id_2
		GROUP BY name;`,
		db.MaxRating, firstFrom.Format(db.SimpleDateFormat), firstTo.Format(db.SimpleDateFormat),
		secondFrom.Format(db.SimpleDateFormat), secondTo.Format(db.SimpleDateFormat))

	if err != nil {
		return out, err
	}

	return out, nil
}
