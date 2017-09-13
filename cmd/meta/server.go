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

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/julienschmidt/httprouter"
	"github.com/meta-network/go-meta"
	"github.com/meta-network/go-meta/musicbrainz"
	"github.com/meta-network/go-meta/xml"
)

type Server struct {
	router *httprouter.Router
	store  *meta.Store
}

func NewServer(store *meta.Store, musicbrainzDB *sql.DB) (*Server, error) {
	srv := &Server{
		router: httprouter.New(),
		store:  store,
	}
	srv.router.GET("/object/:cid", srv.HandleGetObject)
	srv.router.POST("/import/xml", srv.HandleImportXML)
	if musicbrainzDB != nil {
		api, err := musicbrainz.NewAPI(musicbrainzDB, store)
		if err != nil {
			return nil, err
		}
		srv.router.Handler("GET", "/musicbrainz/*path", http.StripPrefix("/musicbrainz", api))
		srv.router.Handler("POST", "/musicbrainz/*path", http.StripPrefix("/musicbrainz", api))
	}
	return srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.router.ServeHTTP(w, req)
}

func (s *Server) HandleImportXML(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var context []*cid.Cid
	if c := req.URL.Query().Get("context"); c != "" {
		for _, v := range strings.Split(c, ",") {
			cid, err := cid.Decode(v)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid CID in context value %q: %s", v, err), http.StatusBadRequest)
				return
			}
			context = append(context, cid)
		}
	}

	obj, err := metaxml.EncodeXML(req.Body, context, s.store.Put)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(obj)
}

func (s *Server) HandleGetObject(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
	cid, err := cid.Decode(p.ByName("cid"))
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid CID %q: %s", p.ByName("cid"), err), http.StatusBadRequest)
		return
	}

	obj, err := s.store.Get(cid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(obj)
}