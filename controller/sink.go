package main

import (
	ct "github.com/flynn/flynn/controller/types"
	"github.com/flynn/flynn/pkg/postgres"
	"github.com/flynn/flynn/pkg/random"
	"github.com/jackc/pgx"
)

type SinkRepo struct {
	db *postgres.DB
}

func NewSinkRepo(db *postgres.DB) *SinkRepo {
	return &SinkRepo{
		db: db,
	}
}

func (r *SinkRepo) Add(data interface{}) error {
	s := data.(*ct.Sink)
	// TODO(jpg): Actually validate
	if s.ID == "" {
		s.ID = random.UUID()
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	err = tx.QueryRow("sink_insert").Scan(&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		tx.Rollback()
		return err
	}
	// create sink event
	if err := createEvent(tx.Exec, &ct.Event{
		AppID:      "",
		ObjectID:   s.ID,
		ObjectType: ct.EventTypeSink,
	}, s); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func scanSinks(rows *pgx.Rows) ([]*ct.Sink, error) {
	var sinks []*ct.Sink
	for rows.Next() {
		sink, err := scanSink(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		sinks = append(sinks, sink)
	}
	return sinks, rows.Err()
}

func scanSink(s postgres.Scanner) (*ct.Sink, error) {
	sink := &ct.Sink{}
	err := s.Scan(&sink.ID, &sink.Kind, &sink.Config, &sink.CreatedAt, &sink.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			err = ErrNotFound
		}
		return nil, err
	}
	return sink, err
}

func (r *SinkRepo) Get(id string) (interface{}, error) {
	row := r.db.QueryRow("sink_select", id)
	return scanSink(row)
}

func (r *SinkRepo) List() (interface{}, error) {
	rows, err := r.db.Query("sink_list")
	if err != nil {
		return nil, err
	}
	return scanSinks(rows)
}

func (r *SinkRepo) Remove(id string) error {
	data, err := r.Get(id)
	sink := data.(*ct.Sink)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	err = tx.Exec("sink_delete", sink.ID)
	if err != nil {
		tx.Rollback()
		return err
	}
	// create sink remove event
	if err := createEvent(tx.Exec, &ct.Event{
		AppID:      "",
		ObjectID:   sink.ID,
		ObjectType: ct.EventTypeSinkDeletion,
	}, sink); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
