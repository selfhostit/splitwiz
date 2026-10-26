// Package app wires Splitwiz's rules and endpoints into PocketBase.
package app

import (
	"errors"
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"

	"github.com/selfhostit/splitwiz/ledger"
)

// Register binds the Splitwiz hooks and routes to a PocketBase app.
func Register(app core.App) {
	// run scheduled jobs (backups) on the container's local time, set by TZ
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		e.App.Cron().SetTimezone(time.Local)
		return nil
	})

	registerRecurring(app)
	registerComments(app)
	registerNotifications(app)

	app.OnRecordValidate("expenses").BindFunc(func(e *core.RecordEvent) error {
		if err := computeOwed(e.Record); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordValidate("settlements").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("from_user") == e.Record.GetString("to_user") {
			return validation.Errors{"to_user": validation.NewError("same_user", "Pick the other person.")}
		}
		return e.Next()
	})

	app.OnRecordCreateRequest("expenses", "settlements").BindFunc(func(e *core.RecordRequestEvent) error {
		if e.Auth != nil {
			e.Record.Set("created_by", e.Auth.Id)
		}
		return e.Next()
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		registerRoutes(se)
		return se.Next()
	})
}

// computeOwed stores how much the non-payer owes for this expense, so the
// client can never send an inconsistent value.
func computeOwed(r *core.Record) error {
	mode := r.GetString("split_mode")
	if mode == "" {
		mode = ledger.ModeEqual
		r.Set("split_mode", mode)
	}

	owed, err := ledger.OwedCents(
		int64(r.GetInt("amount_cents")),
		mode,
		r.GetFloat("payer_percent"),
		int64(r.GetInt("other_share_cents")),
	)
	if err != nil {
		field := "split_mode"
		switch {
		case errors.Is(err, ledger.ErrAmount):
			field = "amount_cents"
		case errors.Is(err, ledger.ErrPercent):
			field = "payer_percent"
		case errors.Is(err, ledger.ErrOtherShare):
			field = "other_share_cents"
		}
		return validation.Errors{field: validation.NewError("invalid_split", err.Error())}
	}

	r.Set("owed_cents", owed)
	return nil
}
