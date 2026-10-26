package app

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	_ "github.com/selfhostit/splitwiz/migrations"
)

func newTestApp(t *testing.T) core.App {
	t.Helper()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	return app
}

func TestCreatePeople(t *testing.T) {
	app := newTestApp(t)
	// what PocketBase adds on first start while no admin exists
	superusers, _ := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	installer := core.NewRecord(superusers)
	installer.SetEmail(core.DefaultInstallerEmail)
	installer.SetRandomPassword()
	if err := app.Save(installer); err != nil {
		t.Fatal(err)
	}
	if needed, err := setupNeeded(app); err != nil || !needed {
		t.Fatalf("setup should be needed on a new install, got %v %v", needed, err)
	}

	people := []setupPerson{
		{Name: "Alex", Email: "alex@example.com", Password: "password-1"},
		{Name: "Sam", Email: "sam@example.com", Password: "password-2"},
	}
	if err := createPeople(app, people); err != nil {
		t.Fatal(err)
	}

	if needed, _ := setupNeeded(app); needed {
		t.Fatal("setup should not be needed once people exist")
	}
	sam, err := app.FindAuthRecordByEmail("users", "sam@example.com")
	if err != nil || !sam.ValidatePassword("password-2") || sam.GetString("name") != "Sam" {
		t.Fatalf("second person not saved correctly: %v", err)
	}
	admin, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "alex@example.com")
	if err != nil || !admin.ValidatePassword("password-1") {
		t.Fatalf("first person should also be the admin: %v", err)
	}
	if n, _ := app.CountRecords(core.CollectionNameSuperusers); n != 1 {
		t.Fatalf("installer placeholder should be gone, got %d admins", n)
	}
}

func TestCreatePeopleKeepsExistingAdmin(t *testing.T) {
	app := newTestApp(t)
	superusers, _ := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	admin := core.NewRecord(superusers)
	admin.SetEmail("owner@example.com")
	admin.SetPassword("owner-password")
	if err := app.Save(admin); err != nil {
		t.Fatal(err)
	}

	people := []setupPerson{
		{Name: "Alex", Email: "alex@example.com", Password: "password-1"},
		{Name: "Sam", Email: "sam@example.com", Password: "password-2"},
	}
	if err := createPeople(app, people); err != nil {
		t.Fatal(err)
	}
	if n, _ := app.CountRecords(core.CollectionNameSuperusers); n != 1 {
		t.Fatalf("want the one existing admin, got %d", n)
	}
}
