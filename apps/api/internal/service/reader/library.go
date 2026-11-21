package reader

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	model "anime-agent/api/internal/model/reader"
	library "anime-agent/api/internal/repository/reader"
	queries "anime-agent/database/queries/reader"
)

func ImportLibrary(ctx context.Context, db *pgxpool.Pool, id, choice string, bookmarks, positions []model.Entry) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		// The first completed decision wins, including a simultaneous skip on another device.
		tag, err := tx.Exec(ctx, queries.ChooseImport, id, choice)
		if err != nil || tag.RowsAffected() == 0 || choice == "skip" {
			return err
		}
		for _, e := range bookmarks {
			if err := library.SaveBookmark(ctx, tx, id, e); err != nil {
				return err
			}
		}
		for _, e := range positions {
			if err := library.SavePosition(ctx, tx, id, e); err != nil {
				return err
			}
		}
		return nil
	})
}
