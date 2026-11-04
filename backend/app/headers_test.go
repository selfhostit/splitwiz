package app

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestSecureHeaders(t *testing.T) {
	s := tests.ApiScenario{
		Name:            "the web app only runs its own scripts",
		Method:          http.MethodGet,
		URL:             "/page",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"page"},
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			a, err := tests.NewTestApp()
			if err != nil {
				t.Fatal(err)
			}
			a.OnServe().BindFunc(func(se *core.ServeEvent) error {
				se.Router.GET("/page", func(e *core.RequestEvent) error {
					return e.String(http.StatusOK, "page")
				}).BindFunc(SecureHeaders)
				return se.Next()
			})
			return a
		},
		AfterTestFunc: func(t testing.TB, _ *tests.TestApp, res *http.Response) {
			if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self';") {
				t.Errorf("Content-Security-Policy = %q", csp)
			}
			if v := res.Header.Get("X-Content-Type-Options"); v != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q", v)
			}
		},
	}
	s.Test(t)
}
