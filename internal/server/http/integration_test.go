package http_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/db"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
	srvhttp "github.com/Hennnnnnn/DevWorkspace/internal/server/http"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

// End-to-end test. Runs on a fresh SQLite file by default (no external DB).
// Set DEVSYNC_TEST_DATABASE_URL to a postgres:// DSN to run against Postgres too.
func TestFullLifecycle(t *testing.T) {
	driver, dsn := "sqlite", ""
	if pg := os.Getenv("DEVSYNC_TEST_DATABASE_URL"); pg != "" {
		driver, dsn = "pgx", pg
	} else {
		path := filepath.Join(t.TempDir(), "test.db")
		dsn = "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, driver, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := store.New(ctx, driver, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer st.Close()

	ts := httptest.NewServer(srvhttp.New(st).Handler())
	defer ts.Close()

	// --- admin registers ---
	admin := newTestDevice(t)
	registerDevice(t, ts.URL, "alice", admin)
	// Bootstrap admin directly (simulating server CLI).
	adminUser, err := st.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetUserStatus(ctx, adminUser.ID, "active")
	devs, _ := st.ListDevices(ctx, adminUser.ID)
	_ = st.SetDeviceStatus(ctx, devs[0].ID, "active")
	_ = st.SetUserAdmin(ctx, adminUser.ID, true)

	// whoami works and shows admin+active.
	var who protocol.WhoAmIResponse
	admin.get(t, ts.URL, "alice", "/whoami", nil, &who)
	if !who.IsAdmin || who.Status != "active" {
		t.Fatalf("expected active admin, got %+v", who)
	}

	// --- create team + vault ---
	admin.post(t, ts.URL, "alice", "/admin/create-team", protocol.CreateTeamRequest{Name: "eng"}, nil)

	vk, _ := crypto.NewVaultKey()
	adminDev := devs[0]
	sealed, _ := crypto.SealVaultKey(vk, boxKey(adminDev.BoxPubKey))
	admin.post(t, ts.URL, "alice", "/admin/create-vault", protocol.CreateVaultRequest{
		Team: "eng", Name: "secrets",
		Shares: []protocol.VaultKeyShare{{DeviceID: adminDev.ID, KeyVersion: 1, EncryptedKey: sealed}},
	}, nil)

	// --- push a file ---
	plain := []byte("API_KEY=super-secret")
	ct, _ := crypto.EncryptBlob(vk, plain)
	var push protocol.PushResponse
	admin.post(t, ts.URL, "alice", "/files/push", protocol.PushRequest{
		Vault: "secrets",
		File:  protocol.FilePush{Path: ".env", KeyVersion: 1, Ciphertext: ct, BaseVersion: 0},
	}, &push)
	if push.Version != 1 {
		t.Fatalf("expected version 1, got %d", push.Version)
	}

	// Stale push rejected.
	code := admin.postRaw(t, ts.URL, "alice", "/files/push", protocol.PushRequest{
		Vault: "secrets",
		File:  protocol.FilePush{Path: ".env", KeyVersion: 1, Ciphertext: ct, BaseVersion: 0},
	})
	if code != http.StatusConflict {
		t.Fatalf("expected 409 on stale push, got %d", code)
	}

	// --- pull + decrypt ---
	var pull protocol.PullResponse
	admin.get(t, ts.URL, "alice", "/files/pull", map[string]string{"vault": "secrets", "path": ".env"}, &pull)
	got, err := crypto.DecryptBlob(vk, pull.Ciphertext)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("pull/decrypt mismatch: %v", err)
	}

	// --- anti-replay: old timestamp rejected ---
	code = admin.getRawTS(t, ts.URL, "alice", "/whoami", time.Now().Unix()-1000)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on stale timestamp, got %d", code)
	}

	// --- bad signature rejected ---
	other := newTestDevice(t)
	code = other.getRawAs(t, ts.URL, "alice", admin.fp, "/whoami")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong-key signature, got %d", code)
	}

	t.Log("full lifecycle passed")
}

// --- test device helper ---

type testDevice struct {
	kp *crypto.KeyPair
	fp string
}

func newTestDevice(t *testing.T) *testDevice {
	t.Helper()
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return &testDevice{kp: kp, fp: crypto.Fingerprint(kp.SignPub)}
}

func boxKey(b []byte) [32]byte {
	var k [32]byte
	copy(k[:], b)
	return k
}

func registerDevice(t *testing.T, base, user string, d *testDevice) {
	t.Helper()
	body, _ := json.Marshal(protocol.RegisterRequest{
		Username: user, DeviceName: "test", SignPubKey: d.kp.SignPub,
		BoxPubKey: d.kp.BoxPub[:], Fingerprint: d.fp,
	})
	resp, err := http.Post(base+"/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("register failed %d: %s", resp.StatusCode, b)
	}
}

func (d *testDevice) signReq(req *http.Request, user string, body []byte) {
	ts := time.Now().Unix()
	msg := protocol.SigningString(req.Method, req.URL.RequestURI(), protocol.BodyHash(body), ts)
	req.Header.Set(protocol.HeaderUser, user)
	req.Header.Set(protocol.HeaderDevice, d.fp)
	req.Header.Set(protocol.HeaderTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(protocol.HeaderSignature, crypto.Sign(d.kp.SignPriv, msg))
}

func (d *testDevice) post(t *testing.T, base, user, path string, in, out any) {
	t.Helper()
	d.doJSON(t, http.MethodPost, base, user, path, in, out)
}

func (d *testDevice) postRaw(t *testing.T, base, user, path string, in any) int {
	t.Helper()
	body, _ := json.Marshal(in)
	req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(body))
	d.signReq(req, user, body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func (d *testDevice) doJSON(t *testing.T, method, base, user, path string, in, out any) {
	t.Helper()
	var body []byte
	if in != nil {
		body, _ = json.Marshal(in)
	}
	req, _ := http.NewRequest(method, base+path, bytes.NewReader(body))
	d.signReq(req, user, body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s -> %d: %s", method, path, resp.StatusCode, b)
	}
	if out != nil {
		json.NewDecoder(resp.Body).Decode(out)
	}
}

func (d *testDevice) get(t *testing.T, base, user, path string, query map[string]string, out any) {
	t.Helper()
	u := base + path
	if len(query) > 0 {
		u += "?"
		first := true
		for k, v := range query {
			if !first {
				u += "&"
			}
			u += k + "=" + v
			first = false
		}
	}
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	d.signReq(req, user, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s -> %d: %s", path, resp.StatusCode, b)
	}
	if out != nil {
		json.NewDecoder(resp.Body).Decode(out)
	}
}

func (d *testDevice) getRawTS(t *testing.T, base, user, path string, ts int64) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+path, nil)
	msg := protocol.SigningString(req.Method, req.URL.RequestURI(), protocol.BodyHash(nil), ts)
	req.Header.Set(protocol.HeaderUser, user)
	req.Header.Set(protocol.HeaderDevice, d.fp)
	req.Header.Set(protocol.HeaderTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(protocol.HeaderSignature, crypto.Sign(d.kp.SignPriv, msg))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// getRawAs signs with this device's key but claims another fingerprint (attack).
func (d *testDevice) getRawAs(t *testing.T, base, user, claimFP, path string) int {
	t.Helper()
	ts := time.Now().Unix()
	req, _ := http.NewRequest(http.MethodGet, base+path, nil)
	msg := protocol.SigningString(req.Method, req.URL.RequestURI(), protocol.BodyHash(nil), ts)
	req.Header.Set(protocol.HeaderUser, user)
	req.Header.Set(protocol.HeaderDevice, claimFP)
	req.Header.Set(protocol.HeaderTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(protocol.HeaderSignature, crypto.Sign(d.kp.SignPriv, msg))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

var _ = ed25519.PublicKeySize
