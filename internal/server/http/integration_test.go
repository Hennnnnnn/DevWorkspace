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

// End-to-end test with team-scoped roles (no global admin).
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

	// --- alice registers (auto-active) ---
	alice := newTestDevice(t)
	registerDevice(t, ts.URL, "alice", alice)
	aliceUser, _ := st.GetUserByUsername(ctx, "alice")
	devs, _ := st.ListDevices(ctx, aliceUser.ID)

	// whoami — status active, no team roles yet.
	var who protocol.WhoAmIResponse
	alice.get(t, ts.URL, "alice", "/whoami", nil, &who)
	if who.Status != "active" || len(who.TeamRoles) != 0 {
		t.Fatalf("expected active with no team roles, got %+v", who)
	}

	// --- create team → alice auto team_admin ---
	var team protocol.Team
	alice.post(t, ts.URL, "alice", "/teams/create", protocol.CreateTeamRequest{Team: "eng"}, &team)
	if team.Name != "eng" || team.Creator != "alice" {
		t.Fatalf("bad team: %+v", team)
	}

	// whoami now shows team role.
	alice.get(t, ts.URL, "alice", "/whoami", nil, &who)
	if len(who.TeamRoles) != 1 || who.TeamRoles[0].Team != "eng" || who.TeamRoles[0].Role != "admin" {
		t.Fatalf("expected team_roles [{eng admin}], got %+v", who.TeamRoles)
	}

	// --- create vault as team_admin ---
	vk, _ := crypto.NewVaultKey()
	sealed, _ := crypto.SealVaultKey(vk, boxKey(devs[0].BoxPubKey))
	alice.post(t, ts.URL, "alice", "/teams/vaults/create", protocol.CreateVaultRequest{
		Team: "eng", Name: "secrets",
		Shares: []protocol.VaultKeyShare{{DeviceID: devs[0].ID, KeyVersion: 1, EncryptedKey: sealed}},
	}, nil)

	// --- push a file ---
	plain := []byte("API_KEY=super-secret")
	ct, _ := crypto.EncryptBlob(vk, plain)
	var push protocol.PushResponse
	alice.post(t, ts.URL, "alice", "/files/push", protocol.PushRequest{
		Vault: "secrets",
		File:  protocol.FilePush{Path: ".env", KeyVersion: 1, Ciphertext: ct, BaseVersion: 0},
	}, &push)
	if push.Version != 1 {
		t.Fatalf("expected version 1, got %d", push.Version)
	}

	// Stale push rejected.
	code := alice.postRaw(t, ts.URL, "alice", "/files/push", protocol.PushRequest{
		Vault: "secrets",
		File:  protocol.FilePush{Path: ".env", KeyVersion: 1, Ciphertext: ct, BaseVersion: 0},
	})
	if code != http.StatusConflict {
		t.Fatalf("expected 409 on stale push, got %d", code)
	}

	// --- pull + decrypt ---
	var pull protocol.PullResponse
	alice.get(t, ts.URL, "alice", "/files/pull", map[string]string{"vault": "secrets", "path": ".env"}, &pull)
	got, err := crypto.DecryptBlob(vk, pull.Ciphertext)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("pull/decrypt mismatch: %v", err)
	}

	// --- invite budi ---
	var inviteResp protocol.InviteTokenResponse
	alice.post(t, ts.URL, "alice", "/teams/invite", protocol.InviteRequest{Team: "eng", Username: "budi"}, &inviteResp)
	if inviteResp.Token == "" {
		t.Fatal("expected invite token")
	}

	// --- budi registers + claims invite → active + member ---
	budi := newTestDevice(t)
	registerDevice(t, ts.URL, "budi", budi)
	budi.post(t, ts.URL, "budi", "/teams/claim", protocol.ClaimInviteRequest{Token: inviteResp.Token}, nil)

	// budi whoami → active, TeamRoles has {eng, member}
	budi.get(t, ts.URL, "budi", "/whoami", nil, &who)
	if who.Status != "active" {
		t.Fatalf("budi not active: %+v", who)
	}
	if len(who.TeamRoles) != 1 || who.TeamRoles[0].Team != "eng" || who.TeamRoles[0].Role != "member" {
		t.Fatalf("budi team roles: %+v", who.TeamRoles)
	}

	// --- alice grants budi vault access ---
	// budi needs an active device for sealing.
	budiUser, _ := st.GetUserByUsername(ctx, "budi")
	budiDevs, _ := st.ListDevices(ctx, budiUser.ID)
	sealedForBudi, _ := crypto.SealVaultKey(vk, boxKey(budiDevs[0].BoxPubKey))
	alice.post(t, ts.URL, "alice", "/teams/vaults/grant", protocol.GrantRequest{
		Team: "eng", Vault: "secrets", Username: "budi",
		Shares: []protocol.VaultKeyShare{{DeviceID: budiDevs[0].ID, KeyVersion: 1, EncryptedKey: sealedForBudi}},
	}, nil)

	// --- budi pulls file ---
	budi.get(t, ts.URL, "budi", "/files/pull", map[string]string{"vault": "secrets", "path": ".env"}, &pull)
	got, err = crypto.DecryptBlob(vk, pull.Ciphertext)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("budi pull/decrypt mismatch: %v", err)
	}

	// --- team_admin check: non-admin member tries grant → 403 ---
	code = budi.postRaw(t, ts.URL, "budi", "/teams/vaults/grant", protocol.GrantRequest{
		Team: "eng", Vault: "secrets", Username: "alice",
	})
	if code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin grant, got %d", code)
	}

	// --- anti-replay: old timestamp rejected ---
	code = alice.getRawTS(t, ts.URL, "alice", "/whoami", time.Now().Unix()-1000)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on stale timestamp, got %d", code)
	}

	// --- bad signature rejected ---
	other := newTestDevice(t)
	code = other.getRawAs(t, ts.URL, "alice", alice.fp, "/whoami")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong-key signature, got %d", code)
	}

	t.Log("full lifecycle passed — team-scoped roles")
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

func TestAuthMiddleware(t *testing.T) {
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

	alice := newTestDevice(t)
	registerDevice(t, ts.URL, "alice", alice)

	// Test 1: missing auth headers
	resp, _ := http.Get(ts.URL + "/whoami")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth headers, got %d", resp.StatusCode)
	}

	// Test 2: bad timestamp format
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/whoami", nil)
	req.Header.Set(protocol.HeaderDevice, alice.fp)
	req.Header.Set(protocol.HeaderTimestamp, "not-a-number")
	req.Header.Set(protocol.HeaderSignature, "AA==")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad timestamp, got %d", resp.StatusCode)
	}

	// Test 3: unknown device fingerprint
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/whoami", nil)
	req.Header.Set(protocol.HeaderDevice, "SHA256:unknownfp")
	req.Header.Set(protocol.HeaderTimestamp, strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set(protocol.HeaderSignature, "AA==")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown device, got %d", resp.StatusCode)
	}

	// Test 4: invalid signature base64
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/whoami", nil)
	req.Header.Set(protocol.HeaderDevice, alice.fp)
	req.Header.Set(protocol.HeaderTimestamp, strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set(protocol.HeaderSignature, "!!!bad-base64!!!")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature base64, got %d", resp.StatusCode)
	}

	// Test 5: anti-replay — timestamp too old
	code := alice.getRawTS(t, ts.URL, "alice", "/whoami", time.Now().Unix()-1000)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for stale timestamp, got %d", code)
	}

	// Test 6: anti-replay — timestamp too new
	code = alice.getRawTS(t, ts.URL, "alice", "/whoami", time.Now().Unix()+1000)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for future timestamp, got %d", code)
	}

	// Test 7: timestamp right at boundary — within skew should pass
	code = alice.getRawTS(t, ts.URL, "alice", "/whoami", time.Now().Unix())
	if code != http.StatusOK {
		t.Fatalf("expected 200 for current timestamp, got %d", code)
	}
}

var _ = ed25519.PublicKeySize
