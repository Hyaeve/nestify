package sqlite

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

func Test115MountRoundTripAndSecretRetention(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	mount, err := store.CreateMount(model.CreateMountInput{Name: "115", Provider: "115", Device: "ios",
		Cookie: "UID=1; CID=2; SEID=secret", RequestIntervalMS: 1500, BasePath: "/media"})
	if err != nil {
		t.Fatal(err)
	}
	if mount.Provider != "115" || !mount.HasCookie || mount.Device != "ios" || mount.RequestIntervalMS != 1500 {
		t.Fatalf("bad mount: %+v", mount)
	}
	items, err := store.ListMounts()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(items)
	if strings.Contains(string(body), "secret") || strings.Contains(string(body), `"cookie":`) {
		t.Fatal("cookie leaked in list")
	}
	disabled := false
	_, err = store.UpdateMount(mount.ID, model.UpdateMountInput{Name: "115", Provider: "115", Device: "ios",
		RequestIntervalMS: 1500, Enabled: &disabled, BasePath: "/media"})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := store.GetMountCredential(mount.ID)
	if err != nil || credential.Cookie != "UID=1; CID=2; SEID=secret" || credential.Mount.Enabled {
		t.Fatalf("toggle lost credentials: %v", err)
	}
	_, err = store.UpdateMount(mount.ID, model.UpdateMountInput{Name: "DAV", Provider: "webdav", Host: "localhost"})
	if err != nil {
		t.Fatal(err)
	}
	credential, err = store.GetMountCredential(mount.ID)
	if err != nil || credential.Cookie != "" {
		t.Fatal("switching provider retained CK")
	}
}

func Test115ColumnsMigrateLegacyMount(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE webdav_mounts (id INTEGER PRIMARY KEY, name TEXT, provider TEXT, auth_type TEXT, token TEXT);
		INSERT INTO webdav_mounts VALUES(1, 'legacy', 'openlist', 'token', 'old-token');`)
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	for range 2 {
		if err := store.ensureMountProviderColumns(); err != nil {
			t.Fatal(err)
		}
	}
	var cookie, device, token string
	var interval int
	if err := db.QueryRow(`SELECT cookie,device,request_interval_ms,token FROM webdav_mounts WHERE id=1`).Scan(&cookie, &device, &interval, &token); err != nil {
		t.Fatal(err)
	}
	if cookie != "" || device != "web" || interval != 1000 || token != "old-token" {
		t.Fatal("migration damaged existing mount")
	}
}
