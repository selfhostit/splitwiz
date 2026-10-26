package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Web push straight from the NAS to the installed app: no account or third
// party service. The keys that sign the messages are made on first use and
// kept in pb_data, so they move with backups.

type vapidKeys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

var (
	keysMu sync.Mutex
	keys   *vapidKeys
)

func pushKeys(app core.App) (*vapidKeys, error) {
	keysMu.Lock()
	defer keysMu.Unlock()
	if keys != nil {
		return keys, nil
	}

	path := filepath.Join(app.DataDir(), "push_keys.json")
	k := &vapidKeys{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, k); err != nil {
			return nil, err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		priv, pub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			return nil, err
		}
		k = &vapidKeys{Public: pub, Private: priv}
		b, _ := json.Marshal(k)
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	keys = k
	return k, nil
}

// pushContact goes in every message so push services know whom to reach;
// SPLITWIZ_PUSH_CONTACT can set a mailto: or https: address of your own.
func pushContact() string {
	if c := os.Getenv("SPLITWIZ_PUSH_CONTACT"); c != "" {
		return c
	}
	return "https://github.com/selfhostit/splitwiz"
}

// notification is what the service worker (web/public/sw.js) shows.
type notification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	// opened when the notification is tapped
	URL string `json:"url"`
	// a newer notification with the same tag replaces the older one
	Tag string `json:"tag,omitempty"`
}

var pushClient = &http.Client{Timeout: 15 * time.Second}

// sendPush delivers n to every phone of the given user, in the background.
// Phones that unsubscribed or reinstalled the app are forgotten.
func sendPush(app core.App, userID string, n notification) {
	subs, err := app.FindRecordsByFilter("push_subscriptions", "user = {:user}", "", 0, 0, map[string]any{"user": userID})
	if err != nil || len(subs) == 0 {
		return
	}
	k, err := pushKeys(app)
	if err != nil {
		app.Logger().Error("push keys", "error", err)
		return
	}
	payload, _ := json.Marshal(n)

	go func() {
		for _, s := range subs {
			res, err := webpush.SendNotification(payload, &webpush.Subscription{
				Endpoint: s.GetString("endpoint"),
				Keys:     webpush.Keys{P256dh: s.GetString("p256dh"), Auth: s.GetString("auth")},
			}, &webpush.Options{
				HTTPClient:      pushClient,
				Subscriber:      pushContact(),
				VAPIDPublicKey:  k.Public,
				VAPIDPrivateKey: k.Private,
				TTL:             24 * 60 * 60,
				Urgency:         webpush.UrgencyNormal,
			})
			if err != nil {
				app.Logger().Warn("push failed", "error", err)
				continue
			}
			res.Body.Close()
			switch {
			case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
				_ = app.Delete(s)
			case res.StatusCode >= 400:
				app.Logger().Warn("push rejected", "status", res.StatusCode)
			}
		}
	}()
}

// notifyOthers sends a notification to everyone except `actor`, built per
// person so it can say "you owe".
func notifyOthers(app core.App, actor string, build func(to string) notification) {
	users, err := loadUsers(app)
	if err != nil {
		return
	}
	for _, u := range users {
		if u.Id != actor {
			sendPush(app, u.Id, build(u.Id))
		}
	}
}

type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func registerPushRoutes(g *router.RouterGroup[*core.RequestEvent]) {
	g.GET("/push/key", func(e *core.RequestEvent) error {
		k, err := pushKeys(e.App)
		if err != nil {
			return e.InternalServerError("", err)
		}
		return e.JSON(http.StatusOK, map[string]string{"key": k.Public})
	})

	g.POST("/push/subscribe", func(e *core.RequestEvent) error {
		var body pushSubscription
		if err := e.BindBody(&body); err != nil || body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
			return e.BadRequestError("Missing subscription.", err)
		}
		rec, err := e.App.FindFirstRecordByData("push_subscriptions", "endpoint", body.Endpoint)
		if err != nil {
			col, err := e.App.FindCollectionByNameOrId("push_subscriptions")
			if err != nil {
				return e.InternalServerError("", err)
			}
			rec = core.NewRecord(col)
			rec.Set("endpoint", body.Endpoint)
		}
		rec.Set("user", e.Auth.Id)
		rec.Set("p256dh", body.Keys.P256dh)
		rec.Set("auth", body.Keys.Auth)
		if err := e.App.Save(rec); err != nil {
			return e.BadRequestError("Could not save.", err)
		}
		return e.NoContent(http.StatusNoContent)
	})

	g.POST("/push/unsubscribe", func(e *core.RequestEvent) error {
		var body pushSubscription
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("", err)
		}
		rec, err := e.App.FindFirstRecordByData("push_subscriptions", "endpoint", body.Endpoint)
		if err == nil && rec.GetString("user") == e.Auth.Id {
			if err := e.App.Delete(rec); err != nil {
				return e.InternalServerError("", err)
			}
		}
		return e.NoContent(http.StatusNoContent)
	})

	g.POST("/push/test", func(e *core.RequestEvent) error {
		sendPush(e.App, e.Auth.Id, notification{
			Title: "Splitwiz",
			Body:  "Notifications are working on this phone.",
			URL:   "/",
			Tag:   "test",
		})
		return e.NoContent(http.StatusNoContent)
	})
}
