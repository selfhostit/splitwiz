package app

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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
//
// Setup asks for a one-time code that the server prints to its log and keeps
// in pb_data/setup_code, where scripts/install.sh reads it. Only someone who
// can see the machine can set it up, so a stranger who finds a fresh install
// on the internet can't add themselves.

// SetupCodeFile holds the setup code inside the data folder until setup is done.
const SetupCodeFile = "setup_code"

// letters and digits that can't be mistaken for each other
const setupCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// setupCode returns the code that first-run setup asks for, making it the
// first time.
func setupCode(app core.App) (string, error) {
	path := filepath.Join(app.DataDir(), SetupCodeFile)
	if b, err := os.ReadFile(path); err == nil {
		if code := strings.TrimSpace(string(b)); code != "" {
			return code, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = setupCodeAlphabet[int(b[i])%len(setupCodeAlphabet)]
	}
	code := string(b[:4]) + "-" + string(b[4:])
	return code, os.WriteFile(path, []byte(code+"\n"), 0o600)
}

// sameSetupCode compares codes the way people type them: any case, with or
// without the dash or spaces.
func sameSetupCode(typed, code string) bool {
	norm := func(s string) []byte {
		return []byte(strings.NewReplacer("-", "", " ", "").Replace(strings.ToUpper(s)))
	}
	return subtle.ConstantTimeCompare(norm(typed), norm(code)) == 1
}

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

func registerSetupRoutes(se *core.ServeEvent) error {
	if needed, err := setupNeeded(se.App); err != nil {
		return err
	} else if needed {
		code, err := setupCode(se.App)
		if err != nil {
			return err
		}
		// on stdout, so `docker logs splitwiz` shows it
		fmt.Printf("\nFirst-run setup code: %s\nThe setup screen asks for it when you add the two people.\n\n", code)
	}

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
			Code   string        `json:"code"`
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
		code, err := setupCode(e.App)
		if err != nil {
			return e.InternalServerError("", err)
		}
		if !sameSetupCode(body.Code, code) {
			return e.ForbiddenError("That setup code is wrong. The installer shows it, and so does Splitwiz's log (docker logs splitwiz).", nil)
		}

		err = e.App.RunInTransaction(func(tx core.App) error {
			return createPeople(tx, body.People)
		})
		if err != nil {
			return e.BadRequestError("Could not save: "+err.Error(), err)
		}
		e.App.Logger().Info("First-run setup done", "admin", body.People[0].Email)
		_ = os.Remove(filepath.Join(e.App.DataDir(), SetupCodeFile))
		return e.NoContent(http.StatusNoContent)
	})
	return nil
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
