package app

import (
	"net/http"
	"strings"
	"sync"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// First-run setup: until the two people exist, the web app shows a setup
// screen instead of the sign-in form, so nobody needs a terminal or the
// admin page to get started. The first person also becomes the admin.
// Once anyone exists the endpoint refuses, so it can only ever run once.

type setupPerson struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var setupMu sync.Mutex

func setupNeeded(app core.App) (bool, error) {
	n, err := app.CountRecords("users")
	return n == 0, err
}

func registerSetupRoutes(se *core.ServeEvent) {
	se.Router.GET("/api/splitwiz/setup", func(e *core.RequestEvent) error {
		needed, err := setupNeeded(e.App)
		if err != nil {
			return e.InternalServerError("", err)
		}
		return e.JSON(http.StatusOK, map[string]bool{"needed": needed})
	})

	se.Router.POST("/api/splitwiz/setup", func(e *core.RequestEvent) error {
		var body struct {
			People []setupPerson `json:"people"`
		}
		if err := e.BindBody(&body); err != nil || len(body.People) != 2 {
			return e.BadRequestError("Fill in both people.", err)
		}
		for i := range body.People {
			p := &body.People[i]
			p.Name = strings.TrimSpace(p.Name)
			p.Email = strings.ToLower(strings.TrimSpace(p.Email))
			if p.Name == "" || p.Email == "" {
				return e.BadRequestError("Fill in a name and an email for both people.", nil)
			}
			if len(p.Password) < 8 {
				return e.BadRequestError("Passwords need at least 8 characters.", nil)
			}
		}
		if body.People[0].Email == body.People[1].Email {
			return e.BadRequestError("The two people need different emails.", nil)
		}

		setupMu.Lock()
		defer setupMu.Unlock()
		needed, err := setupNeeded(e.App)
		if err != nil {
			return e.InternalServerError("", err)
		}
		if !needed {
			return e.ForbiddenError("Splitwiz is already set up.", nil)
		}

		err = e.App.RunInTransaction(func(tx core.App) error {
			return createPeople(tx, body.People)
		})
		if err != nil {
			return e.BadRequestError("Could not save: "+err.Error(), err)
		}
		e.App.Logger().Info("First-run setup done", "admin", body.People[0].Email)
		return e.NoContent(http.StatusNoContent)
	})
}

// createPeople adds both users and, unless one exists already, an admin
// account with the first person's email and password.
func createPeople(tx core.App, people []setupPerson) error {
	users, err := tx.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	for _, p := range people {
		r := core.NewRecord(users)
		r.Set("name", p.Name)
		r.SetEmail(p.Email)
		r.SetPassword(p.Password)
		r.SetVerified(true)
		r.Set("seen_at", types.NowDateTime())
		if err := tx.Save(r); err != nil {
			return err
		}
	}

	// PocketBase keeps a placeholder installer account until a real admin
	// exists; saving one removes it
	admins, err := tx.CountRecords(core.CollectionNameSuperusers,
		dbx.Not(dbx.HashExp{"email": core.DefaultInstallerEmail}))
	if err != nil || admins > 0 {
		return err
	}
	superusers, err := tx.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		return err
	}
	admin := core.NewRecord(superusers)
	admin.SetEmail(people[0].Email)
	admin.SetPassword(people[0].Password)
	return tx.Save(admin)
}
