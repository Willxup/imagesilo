package indexstate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

type Result struct {
	Delivery delivery.LoadResult
	Sessions int
	Tokens   int
}

type snapshot struct {
	delivery delivery.Snapshot
	sessions map[[32]byte]auth.SessionIdentity
	tokens   map[[32]byte]apitoken.Identity
}

type Rebuilder struct {
	db           *sql.DB
	storage      *storage.Filesystem
	authRepo     *auth.Repository
	tokenRepo    *apitoken.Repository
	delivery     *delivery.Index
	sessions     *auth.SessionIndex
	tokens       *apitoken.Index
	barrier      *indexbarrier.Barrier
	loadSnapshot func(context.Context, time.Time) (snapshot, Result, error)
}

func NewRebuilder(
	db *sql.DB,
	filesystem *storage.Filesystem,
	authRepository *auth.Repository,
	tokenRepository *apitoken.Repository,
	deliveryIndex *delivery.Index,
	sessionIndex *auth.SessionIndex,
	tokenIndex *apitoken.Index,
	barrier *indexbarrier.Barrier,
) *Rebuilder {
	r := &Rebuilder{
		db: db, storage: filesystem, authRepo: authRepository, tokenRepo: tokenRepository,
		delivery: deliveryIndex, sessions: sessionIndex, tokens: tokenIndex, barrier: barrier,
	}
	r.loadSnapshot = r.load
	return r
}

func (r *Rebuilder) Rebuild(ctx context.Context, now time.Time) (Result, error) {
	releaseRebuild := r.barrier.BeginRebuild()
	defer releaseRebuild()

	loaded, result, err := r.loadSnapshot(ctx, now)
	if err != nil {
		return Result{}, err
	}
	r.delivery.ReplaceAll(loaded.delivery.Targets, loaded.delivery.Aliases)
	r.sessions.Replace(loaded.sessions)
	r.tokens.Replace(loaded.tokens)
	return result, nil
}

func (r *Rebuilder) load(ctx context.Context, now time.Time) (snapshot, Result, error) {
	deliverySnapshot, deliveryResult, err := delivery.Build(ctx, r.db, r.storage)
	if err != nil {
		return snapshot{}, Result{}, err
	}
	sessions, err := r.authRepo.ListActiveSessions(ctx, now)
	if err != nil {
		return snapshot{}, Result{}, fmt.Errorf("load session index: %w", err)
	}
	tokens, err := r.tokenRepo.ListActive(ctx, now)
	if err != nil {
		return snapshot{}, Result{}, fmt.Errorf("load API token index: %w", err)
	}
	return snapshot{delivery: deliverySnapshot, sessions: sessions, tokens: tokens}, Result{
		Delivery: deliveryResult,
		Sessions: len(sessions),
		Tokens:   len(tokens),
	}, nil
}
