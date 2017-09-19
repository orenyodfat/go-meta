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

package musicbrainz

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ipfs/go-cid"
	"github.com/julienschmidt/httprouter"
	"github.com/meta-network/go-meta"
	"github.com/neelance/graphql-go"
	"github.com/neelance/graphql-go/relay"
)

// API is a http.Handler which serves GraphQL query responses using a Resolver.
type API struct {
	db     *sql.DB
	store  *meta.Store
	router *httprouter.Router
}

func NewAPI(db *sql.DB, store *meta.Store) (*API, error) {
	schema, err := graphql.ParseSchema(
		GraphQLSchema,
		NewResolver(db, store),
	)
	if err != nil {
		return nil, err
	}
	api := &API{
		db:     db,
		store:  store,
		router: httprouter.New(),
	}
	api.router.GET("/", api.HandleIndex)
	api.router.POST("/connect", api.HandleConnect)
	api.router.Handler("POST", "/graphql", &relay.Handler{Schema: schema})
	return api, nil
}

func (a *API) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	a.router.ServeHTTP(w, req)
}

func (a *API) HandleIndex(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(indexHTML)
}

func (a *API) HandleConnect(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// open a connection to PostgreSQL
	uri := req.URL.Query().Get("postgres-uri")
	if uri == "" {
		http.Error(w, "missing postgres-uri query param", http.StatusBadRequest)
		return
	}
	db, err := sql.Open("postgres", uri)
	if err != nil {
		http.Error(w, fmt.Sprintf("error connecting to %q: %s", uri, err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// create the indexer
	indexer, err := NewIndexer(a.db, a.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// convert the artists into a stream
	stream := make(chan *cid.Cid)
	go func() {
		defer close(stream)
		if err := NewConverter(db, a.store).ConvertArtists(req.Context(), stream); err != nil {
			log.Error("error converting MusicBrainz data", "uri", uri, "err", err)
		}
	}()

	// index the stream
	if err := indexer.IndexArtists(req.Context(), stream); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

var indexHTML = []byte(`
<!DOCTYPE html>
<html>
	<head>
		<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.10.2/graphiql.css" />
		<script src="https://cdnjs.cloudflare.com/ajax/libs/fetch/1.1.0/fetch.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react/15.5.4/react.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react/15.5.4/react-dom.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.10.2/graphiql.js"></script>
	</head>
	<body style="width: 100%; height: 100%; margin: 0; overflow: hidden;">
		<div id="graphiql" style="height: 100vh;">Loading...</div>
		<script>
			function graphQLFetcher(graphQLParams) {
				return fetch("graphql", {
					method: "post",
					body: JSON.stringify(graphQLParams),
					credentials: "include",
				}).then(function (response) {
					return response.text();
				}).then(function (responseBody) {
					try {
						return JSON.parse(responseBody);
					} catch (error) {
						return responseBody;
					}
				});
			}
			ReactDOM.render(
				React.createElement(GraphiQL, {fetcher: graphQLFetcher}),
				document.getElementById("graphiql")
			);
		</script>
	</body>
</html>
`)
