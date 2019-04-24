package models

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"godiscourse/internal/durable"
	"godiscourse/internal/session"
	"io"
	"strings"
	"time"

	"github.com/gofrs/uuid"
)

// SolidStatisticID is used to generate a solid id from name
const SolidStatisticID = "540cbd3c-f4eb-479c-bcd8-b5629af57267"

// Statistic is the body of statistic
type Statistic struct {
	StatisticID string    `sql:"statistic_id,pk"`
	Name        string    `sql:"name,notnull"`
	Count       int64     `sql:"count,notnull"`
	CreatedAt   time.Time `sql:"created_at"`
	UpdatedAt   time.Time `sql:"updated_at"`
}

var statisticColumns = []string{"statistic_id", "name", "count", "created_at", "updated_at"}

func (s *Statistic) values() []interface{} {
	return []interface{}{s.StatisticID, s.Name, s.Count, s.CreatedAt, s.UpdatedAt}
}

func upsertStatistic(ctx context.Context, tx *sql.Tx, name string) (*Statistic, error) {
	id, _ := generateStatisticID(SolidStatisticID, name)
	s, err := findStatistic(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	var count int64
	switch name {
	case "users":
		count, err = usersCount(ctx, tx)
	case "topics":
		count, err = topicsCount(ctx, tx)
	case "comments":
		count, err = commentsCount(ctx, tx)
	default:
		return nil, session.BadDataError(ctx)
	}
	if err != nil {
		return nil, err
	}
	if s != nil {
		s.Count = count
		_, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE statistics SET count=$1 WHERE statistic_id=$2"), count, id)
		return s, err
	}
	s = &Statistic{
		StatisticID: id,
		Name:        name,
		Count:       int64(count),
	}
	cols, params := durable.PrepareColumnsWithValues(statisticColumns)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO statistics(%s) VALUES (%s)", cols, params), s.values()...); err != nil {
		return nil, err
	}
	return s, nil
}

func findStatistic(ctx context.Context, tx *sql.Tx, id string) (*Statistic, error) {
	if _, err := uuid.FromString(id); err != nil {
		return nil, nil
	}

	row := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM Statistics WHERE statistic_id=$1", strings.Join(statisticColumns, ",")), id)
	s, err := statisticFromRows(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func statisticFromRows(row durable.Row) (*Statistic, error) {
	var s Statistic
	err := row.Scan(&s.StatisticID, &s.Name, &s.Count, &s.CreatedAt, &s.UpdatedAt)
	return &s, err
}

func generateStatisticID(ID, name string) (string, error) {
	h := md5.New()
	io.WriteString(h, ID)
	io.WriteString(h, name)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id, err := uuid.FromBytes(sum)
	return id.String(), err
}
