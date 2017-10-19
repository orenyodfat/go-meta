// This file is part of the go-meta library.
//
// Copyright (C) 2017 JAAK MUSIC LTD
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// If you have any questions please contact yo@jaak.io

package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	meta "github.com/meta-network/go-meta"
	"github.com/meta-network/go-meta/cwr"
	"github.com/meta-network/go-meta/ern"
	musicbrainz "github.com/meta-network/go-meta/musicbrainz"
	graphql "github.com/neelance/graphql-go"
)

type testIndex struct {
	indexs  map[string]*sql.DB
	schemas map[string]*graphql.Schema
	store   *meta.Store
	tmpDir  string
}

func (t *testIndex) cleanup() {
	if t.indexs != nil {
		for _, v := range t.indexs {
			v.Close()
		}
	}
	if t.tmpDir != "" {
		os.RemoveAll(t.tmpDir)
	}
}

// TestMetaAPI tests querying a recording via the GraphQL API.
func TestMetaRecordingAPI(t *testing.T) {
	// create a test index of registeredWorks
	x, err := newTestData()
	if err != nil {
		t.Fatal(err)
	}
	defer x.cleanup()

	// start the API server
	s, err := newTestAPI(x.indexs, x.store, x.schemas)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// define a function to execute and assert an artist GraphQL query
	assertQuery := func(expected string, query string, args ...interface{}) {
		query = fmt.Sprintf(query, args...)
		data, _ := json.Marshal(map[string]string{"query": query})
		req, err := http.NewRequest("POST", s.URL+"/graphql", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("unexpected HTTP status: %s", res.Status)
		}
		var r graphql.Response
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
		if len(r.Errors) > 0 {
			t.Fatalf("unexpected errors in API response: %v", r.Errors)
		}

		fmt.Println(string(r.Data))

	}
	assertQuery("", `{ recording(id:%q)
		               {  genre{value sources{value source score}}
									   displayTitle{value sources{value source score}}
										 identifiers{id type}} }`,
		"CASE00000001")

}

func newTestAPI(indexs map[string]*sql.DB, store *meta.Store, schemas map[string]*graphql.Schema) (*httptest.Server, error) {
	api, err := NewAPI(indexs, store, schemas)
	if err != nil {
		return nil, err
	}
	return httptest.NewServer(api), nil
}

func newTestData() (x *testIndex, err error) {
	x = &testIndex{}
	x.indexs = make(map[string]*sql.DB)
	x.schemas = make(map[string]*graphql.Schema)
	defer func() {
		if err != nil {
			x.cleanup()
		}
	}()

	x.tmpDir, err = ioutil.TempDir("", "api-test")
	if err != nil {
		return nil, err
	}
	x.store = meta.NewMapDatastore()

	for name, indexF := range map[string]func() (schema *graphql.Schema, err error){
		"ern":         x.createTestErnIndex,
		"cwr":         x.createTestCWRIndex,
		"musicbrainz": x.createTestMusicBrainzIndex,
	} {
		schema, err := indexF()
		if err != nil {
			return nil, err
		}
		x.schemas[name] = schema
	}
	return x, nil
}

func (x *testIndex) createTestCWRIndex() (schema *graphql.Schema, err error) {
	// create a test SQLite3 db
	db, err := sql.Open("sqlite3", filepath.Join(x.tmpDir, "cwr.db"))
	if err != nil {
		return nil, err
	}
	converter := cwr.NewConverter(x.store)

	f, err := os.Open(filepath.Join("../cwr/testdata", "example_nwr.cwr"))

	if err != nil {
		return nil, err
	}
	defer f.Close()

	cwrCid, err := converter.ConvertCWR(f, "testC")
	if err != nil {
		return nil, err
	}

	// create a stream of CWR
	stream := make(chan *cid.Cid)
	go func() {
		defer close(stream)
		stream <- cwrCid
	}()
	// index the stream of CWR txs
	indexer, err := cwr.NewIndexer(db, x.store)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := indexer.Index(ctx, stream); err != nil {
		return nil, err
	}
	schema, err = graphql.ParseSchema(
		cwr.GraphQLSchema,
		cwr.NewResolver(db, x.store),
	)
	return
}

func (x *testIndex) createTestMusicBrainzIndex() (schema *graphql.Schema, err error) {
	// load the test artists
	var artists []*musicbrainz.Artist
	f, err := os.Open("../musicbrainz/testdata/artists.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for {
		var artist *musicbrainz.Artist
		err := dec.Decode(&artist)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		artist.Source = "testA"
		artists = append(artists, artist)
	}
	cids := make([]*cid.Cid, len(artists))
	for i, artist := range artists {
		obj, err := x.store.Put(artist)
		if err != nil {
			return nil, err
		}
		cids[i] = obj.Cid()
	}

	// create a stream
	stream := make(chan *cid.Cid, len(artists))
	go func() {
		defer close(stream)
		for _, cid := range cids {
			stream <- cid
		}
	}()

	// create a test SQLite3 db
	x.tmpDir, err = ioutil.TempDir("", "musicbrainz-index-test")
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", filepath.Join(x.tmpDir, "musicbraniz.db"))
	if err != nil {
		return nil, err
	}

	// index the artists
	indexer, err := musicbrainz.NewIndexer(db, x.store)
	if err != nil {
		return nil, err
	}
	if err := indexer.IndexArtists(context.Background(), stream); err != nil {
		return nil, err
	}
	schema, err = graphql.ParseSchema(
		musicbrainz.GraphQLSchema,
		musicbrainz.NewResolver(db, x.store),
	)
	return
}

func (x *testIndex) createTestErnIndex() (schema *graphql.Schema, err error) {
	// convert the test ERNs to META objects
	erns := []string{
		"Profile_AudioAlbumMusicOnly.xml",
		//"Profile_AudioSingle.xml",
		//"Profile_AudioAlbum_WithBooklet.xml",
		//"Profile_AudioSingle_WithCompoundArtistsAndTerritorialOverride.xml",
		//	"Profile_AudioBook.xml",
	}
	converter := ern.NewConverter(x.store)
	cids := make(map[string]*cid.Cid, len(erns))
	for _, path := range erns {
		f, err := os.Open(filepath.Join("../ern/testdata", path))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		cid, err := converter.ConvertERN(f, "testB")
		if err != nil {
			return nil, err
		}
		cids[path] = cid
	}

	// create a stream of ERNs
	stream := make(chan *cid.Cid, len(erns))
	go func() {
		defer close(stream)
		for _, cid := range cids {
			stream <- cid
		}
	}()
	db, err := sql.Open("sqlite3", filepath.Join(x.tmpDir, "ern.db"))
	if err != nil {
		return nil, err
	}
	//defer db.Close()

	// index the stream of ERNs
	indexer, err := ern.NewIndexer(db, x.store)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := indexer.Index(ctx, stream); err != nil {
		return nil, err
	}
	schema, err = graphql.ParseSchema(
		ern.GraphQLSchema,
		ern.NewResolver(db, x.store),
	)
	return
}
