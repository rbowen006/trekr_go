//go:build integration

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// fakeEmbedQueue records EnqueueListingEmbed calls, standing in for the asynq
// client so the enqueue seam can be asserted without Redis.
type fakeEmbedQueue struct {
	mu  sync.Mutex
	ids []int64
}

func (f *fakeEmbedQueue) EnqueueListingEmbed(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, id)
	return nil
}

func (f *fakeEmbedQueue) calls() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.ids...)
}

// createListing posts a valid multipart listing and returns its id.
func createListing(t *testing.T, baseURL, ownerAuth string) int64 {
	t.Helper()
	body, ct := buildListingMultipart(t, map[string]string{
		"listing[title]":         "Cozy Caravan",
		"listing[description]":   "A lovely caravan",
		"listing[rv_type]":       "caravan",
		"listing[town]":          "Byron Bay",
		"listing[state]":         "NSW",
		"listing[postcode]":      "2481",
		"listing[price_per_day]": "150",
		"listing[max_guests]":    "4",
	}, "test.png", []byte("pngdata"))

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/listings", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", ownerAuth)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var got struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	return got.ID
}

func TestListingWrite_EnqueuesEmbedOnCreateAndUpdate(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir() // keep uploaded blobs out of the repo
	queue := &fakeEmbedQueue{}
	app.EmbedQueue = queue
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "embq-owner")
	auth := testutil.AuthHeader(t, app, owner)

	id := createListing(t, server.URL, auth)
	require.Equal(t, []int64{id}, queue.calls(), "create enqueues one embed for the new listing")

	resp := doAuthJSON(t, http.MethodPut, fmt.Sprintf("%s/api/v1/listings/%d", server.URL, id),
		auth, `{"listing":{"title":"Changed"}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, []int64{id, id}, queue.calls(), "update enqueues a second embed for the listing")
}
