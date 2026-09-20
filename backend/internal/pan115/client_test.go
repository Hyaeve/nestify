package pan115

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type testTransport func(*http.Request) (*http.Response, error)

func (transport testTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func mockAPI(t *testing.T, handler func(*http.Request) any) {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = testTransport(func(request *http.Request) (*http.Response, error) {
		body, err := json.Marshal(handler(request))
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(string(body))), Request: request}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
}

func TestCookieAndDeviceValidation(t *testing.T) {
	for _, value := range []string{"", "UID=1; CID=2", "UID=1;\r\nCID=2; SEID=3"} {
		if _, err := ParseCookie(value); err == nil {
			t.Fatalf("accepted invalid CK %q", value)
		}
	}
	credential, err := ParseCookie("UID=1_A1; CID=two; SEID=three==; KID=four; ")
	if err != nil || credential.SEID != "three==" {
		t.Fatalf("parse CK: %v", err)
	}
	if ValidDevice("windows") || !ValidDevice("web") || !ValidDevice("ios") {
		t.Fatal("device allowlist mismatch")
	}
}

func TestListPaginationAndRootResolution(t *testing.T) {
	var offsets []string
	mockAPI(t, func(request *http.Request) any {
		if !strings.Contains(request.Header.Get("Cookie"), "SEID=secret") {
			t.Error("missing SDK credentials")
		}
		id := request.URL.Query().Get("cid")
		offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
		if id == "0" {
			return map[string]any{"state": true, "cid": "0", "offset": 0, "count": 1,
				"data": []any{map[string]any{"cid": "20", "pid": "0", "n": "media"}}}
		}
		if id != "20" {
			t.Fatalf("unexpected directory %s", id)
		}
		offsets = append(offsets, strconv.Itoa(offset))
		count := 500
		if offset == 500 {
			count = 1
		}
		items := make([]any, count)
		for index := range items {
			items[index] = map[string]any{"fid": strconv.Itoa(index + offset + 100), "cid": id, "n": "movie" + strconv.Itoa(index+offset), "s": 20}
		}
		return map[string]any{"state": true, "cid": id, "offset": offset, "count": 501, "data": items}
	})
	client := &Client{Cookie: "UID=100_A1; CID=cid; SEID=secret", Device: "web", Root: "/media"}
	files, err := client.List(context.Background(), "")
	if err != nil || len(files) != 501 {
		t.Fatalf("pagination: count=%d err=%v", len(files), err)
	}
	if strings.Join(offsets, ",") != "0,500" {
		t.Fatalf("offsets=%v", offsets)
	}
	if _, err := client.List(context.Background(), "../outside"); err == nil {
		t.Fatal("path traversal allowed")
	}
}

func TestListRejectsMismatchedAndFailedPages(t *testing.T) {
	for _, body := range []string{
		`{"state":true,"cid":"99","offset":0,"count":0,"data":[]}`,
		`{"state":true,"cid":"0","offset":0,"count":1,"data":[]}`,
		`{"state":false,"errno":99,"error":"SEID=secret"}`,
	} {
		t.Run(body, func(t *testing.T) {
			mockAPI(t, func(*http.Request) any { return json.RawMessage(body) })
			client := &Client{Cookie: "UID=1_A1; CID=2; SEID=secret", Device: "web"}
			_, err := client.List(context.Background(), "")
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
}

func TestDeleteProtectsRootAndNonemptyDirectories(t *testing.T) {
	deletes := 0
	mockAPI(t, func(request *http.Request) any {
		if request.Method == http.MethodPost {
			deletes++
			return map[string]any{"state": true}
		}
		id := request.URL.Query().Get("cid")
		items := []any{map[string]any{"cid": "2", "pid": "0", "n": "folder"}}
		if id == "2" {
			items = []any{map[string]any{"fid": "3", "cid": "2", "n": "keep.txt"}}
		}
		return map[string]any{"state": true, "cid": id, "offset": 0, "count": len(items), "data": items}
	})
	client := &Client{Cookie: "UID=1_A1; CID=2; SEID=secret", Device: "web"}
	for _, path := range []string{"", "/", "folder"} {
		if err := client.Delete(context.Background(), path, true); err == nil {
			t.Fatalf("unsafe delete %q", path)
		}
	}
	if deletes != 0 {
		t.Fatalf("unexpected DELETE calls=%d", deletes)
	}
	if err := client.Delete(context.Background(), "folder/keep.txt", false); err != nil {
		t.Fatal(err)
	}
	if deletes != 1 {
		t.Fatal("file was not deleted")
	}
}

func TestRequestGateSerializesAndCancels(t *testing.T) {
	gate := newGate()
	var mutex sync.Mutex
	var times []time.Time
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := gate.wait(context.Background(), 20*time.Millisecond); err != nil {
				t.Error(err)
				return
			}
			mutex.Lock()
			times = append(times, time.Now())
			mutex.Unlock()
		}()
	}
	workers.Wait()
	if len(times) != 4 || times[3].Sub(times[0]) < 55*time.Millisecond {
		t.Fatalf("not throttled: %v", times)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := gate.wait(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("not canceled: %v", err)
	}
}

func TestSameAccountSharesTransportGate(t *testing.T) {
	first, err := NewDriver(context.Background(), "UID=42_A1; CID=2; SEID=a", "web", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewDriver(context.Background(), "UID=42_A2; CID=3; SEID=b", "ios", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	firstTransport := first.Client.GetClient().Transport.(throttledTransport)
	secondTransport := second.Client.GetClient().Transport.(throttledTransport)
	if firstTransport.gate != secondTransport.gate {
		t.Fatal("account limiter is not shared")
	}
}
