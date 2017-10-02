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

package eidr

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"

	"github.com/ipfs/go-cid"
	"github.com/meta-network/go-meta"
	"github.com/meta-network/go-meta/doi"
	"github.com/meta-network/go-meta/log"
)

var (
	parselog *log.ParseLogger
)

func init() {
	parselog = log.New(os.Stderr, "eidr")
}

// Indexer is a META indexer which indexes a stream of META objects
// representing MusicBrainz Artists into a SQLite3 database, getting the
// associated META objects from a META store.
type Indexer struct {
	db    *sql.DB
	store *meta.Store
}

// NewIndexer returns an Indexer which updates the indexes in the given SQLite3
// database connection, getting META objects from the given META store.
func NewIndexer(indexDB *sql.DB, store *meta.Store) (*Indexer, error) {
	// migrate the db to ensure it has an up-to-date schema
	if err := migrations.Run(indexDB); err != nil {
		return nil, err
	}

	return &Indexer{
		db:    indexDB,
		store: store,
	}, nil
}

func (i *Indexer) Index(ctx context.Context, stream chan *cid.Cid) error {
	for {
		select {
		case cid, ok := <-stream:
			if !ok {
				return nil
			}
			obj, err := i.store.Get(cid)
			if err != nil {
				return err
			}
			if err := i.index(obj); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Based on EIDR 2.0 Data Types
func (i *Indexer) index(eidrobj *meta.Object) error {

	topgraph := meta.NewGraph(i.store, eidrobj)

	// eidr objects are divided in baseobject and extraobject
	// we extract these top level objects first
	base, err := topgraph.Get("FullMetadata", "BaseObjectData")
	if meta.IsPathNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	var extracid *cid.Cid
	var extraobj *meta.Object
	extra, err := topgraph.Get("FullMetadata", "ExtraObjectMetadata")
	if err != nil && !meta.IsPathNotFound(err) {
		return err
	} else {
		var ok bool
		extracid, ok = extra.(*cid.Cid)
		if !ok {
			return nil
		}
		parselog.Log(log.LL_DEBUG, "eidr", fmt.Sprintf("extradata cid %v", *extracid))
		extraobj, err = i.store.Get(extracid)
		if err != nil {
			return err
		}
	}
	basecid, ok := base.(*cid.Cid)
	if !ok {
		return nil
	}
	baseobj, err := i.store.Get(basecid)
	if err != nil {
		return err
	}

	// PARSE BASE OBJECT (common for all types)

	var baseObject struct {
		simple            map[string]string
		id                doi.ID
		status            string
		structuralType    string
		associatedOrg     []AssociatedOrg
		referentType      string
		resourceName      string
		resourceNameClass *string
		resourceNameLang  string
		alternateID       []AlternateID
		approximateLength *string
		parentId          *doi.ID
	}
	// simplified handling of key/value pairs
	baseObject.simple = make(map[string]string)

	// iterate all elements
	var m map[string]interface{}
	baseobj.Decode(&m)
	for field, _ := range m {
		parselog.Log(log.LL_WARN, basecid.String(), field)
		l, err := baseobj.GetLink(field)
		if err != nil {
			// don't fail on optional fields: alternateID
			// pass @value through as we use it for the simple handler
			if field == "AlternateID" || (field[0:1] == "@" && field != "@value") {
				continue
			}
			fmt.Printf("linknotfound: %v", err)
			return err
		}
		// get current object and graph
		lobj, err := i.store.Get(l.Cid)
		if err != nil {
			return err
		}
		graph := meta.NewGraph(i.store, lobj)
		toptype, err := graph.Get("@type")
		if err != nil {
			return err
		}
		v, err := graph.Get("@value")
		switch toptype.(string) {
		case "Credits":
			// todo
			break
		case "Administrators":
			// todo
			break
		case "ID":
			baseObject.id = doi.ID(v.(string))
			break
		case "ResourceName":
			lang, err := lobj.GetString("lang")
			if err != nil {
				return err
			}
			class, _ := lobj.GetString("titleClass") // optional
			baseObject.resourceName = v.(string)
			baseObject.resourceNameLang = lang
			baseObject.resourceNameClass = &class
			break
		case "AlternateID":
			t, err := graph.Get("http://www.w3.org/2001/XMLSchema-instance:type")
			if err != nil {
				return err
			}
			baseObject.alternateID = append(baseObject.alternateID, AlternateID{
				ID:   v.(string),
				Type: t.(string),
			})
			if t.(string) == "Proprietary" {
				d, err := graph.Get("domain")
				if err != nil {
					return err
				}
				domain, _ := d.(string)
				baseObject.alternateID[len(baseObject.alternateID)-1].Domain = &domain
			}
			// TODO: add relation field (missing in first sample)
			break
		case "AssociatedOrg":
			t, err := graph.Get("role")
			if err != nil {
				return err
			}
			l, err := lobj.GetLink("DisplayName")
			if err != nil {
				return err
			}
			lobj, err = i.store.Get(l.Cid)
			if err != nil {
				return err
			}
			graph := meta.NewGraph(i.store, lobj)
			v, err = graph.Get("@value")
			baseObject.associatedOrg = append(baseObject.associatedOrg, AssociatedOrg{
				DisplayName: v.(string),
				Role:        t.(string),
			})
			// TODO: add id and idtype (missing in first sample)
		default:
			baseObject.simple[field] = v.(string)
		}
	}

	// PARSE EXTRA METADATA ("DERIVED TYPES")

	// we infer the type later to choose which table to insert into
	var extratype interface{}

	// expect only one instance of extra metadata per base object
	if extraobj != nil {
		for _, field := range []string{
			"Clip",
			"Edit",
			"Season",
			"EpisodeInfo",
			"Manifestation",
		} {
			e, err := extraobj.GetLink(field)
			if err != nil {
				continue
			}
			eobj, err := i.store.Get(e.Cid)
			graph := meta.NewGraph(i.store, eobj)

			switch field {
			case "EpisodeInfo":
				episodeobj := episode{}
				parent, err := graph.Get("Parent", "@value")
				if err != nil {
					return err
				}
				var parentid doi.ID = doi.ID(parent.(string))
				baseObject.parentId = &parentid

				// sequenceinfo is an optional field
				// there may be several items per episode
				sepcid, err := eobj.GetLink("SequenceInfo")
				if err == nil {
					episodeobj.SequenceInfo = &SequenceInfo{}
					seqobj, err := i.store.Get(sepcid.Cid)
					if err != nil {
						return err
					}
					var m map[string]interface{}
					err = seqobj.Decode(&m)
					if err != nil {
						return err
					}
					for field, _ := range m {
						if field[0:1] == "@" {
							continue
						}
						sub, err := seqobj.GetLink(field)
						if err != nil {
							return err
						}
						subobj, err := i.store.Get(sub.Cid)
						vs, err := subobj.GetString("@value")
						if err != nil {
							return err
						}
						// domain is optional
						d, _ := subobj.GetString("domain")
						seq := &Sequence{
							Value:  vs,
							Domain: d,
						}
						switch field {
						case "DistributionNumber":
							episodeobj.SequenceInfo.DistributionNumber = seq
							break
						case "HouseSequence":
							episodeobj.SequenceInfo.HouseSequence = seq
							break
						}
					}
				}
				extratype = episodeobj
			}
		}
	}

	// INSERT INTO INDEX DB
	// TODO: rollback on unsuccessful reset

	// insert baseobject single item fields
	_, err = i.db.Exec(
		"INSERT INTO baseobject (doi_id, structural_type, referent_type, resource_name, resource_name_lang, resource_name_class, status) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		baseObject.id,
		baseObject.simple["StructuralType"],
		baseObject.simple["ReferentType"],
		baseObject.resourceName,
		baseObject.resourceNameLang,
		baseObject.resourceNameClass,
		baseObject.simple["Status"])
	if err != nil {
		return err
	}
	// insert multiple value base object fields
	for _, org := range baseObject.associatedOrg {
		_, err = i.db.Exec(
			"INSERT INTO org (id, idtype, display_name, role, base_doi_id) VALUES ($1, $2, $3, $4, $5)",
			org.ID,
			org.IDType,
			org.DisplayName,
			org.Role,
			baseObject.id)
		if err != nil {
			return err
		}
	}
	for _, altid := range baseObject.alternateID {
		_, err = i.db.Exec(
			"INSERT INTO alternateid (id, type, domain, relation, base_doi_id) VALUES ($1, $2, $3, $4, $5)",
			altid.ID,
			altid.Type,
			altid.Domain,
			altid.Relation,
			baseObject.id)

		if err != nil {
			return err
		}
	}

	// process extrametadata
	if extratype != nil {
		t := reflect.TypeOf(extratype)
		var extraid int64

		// sort on extra metadata object type
		switch t.Name() {
		case "episodeInfo":
			o, _ := extratype.(episode)
			r, err := i.db.Exec(
				"INSERT INTO xobject_episode (episode_class, time_slot) VALUES ($1, $2)",
				o.EpisodeClass,
				o.TimeSlot)
			if err != nil {
				return err
			}
			extraid, err = r.LastInsertId()
			if err != nil {
				return err
			}

			// index object ideosyncracies
			if o.SequenceInfo != nil {
				if o.SequenceInfo.DistributionNumber != nil {
					_, err = i.db.Exec(
						"INSERT INTO xobject_episode_sequenceinfo (episode_id, type, value, domain) VALUES ($1, $2, $3, $4)",
						extraid,
						"DistributionNumber",
						o.SequenceInfo.DistributionNumber.Value,
						o.SequenceInfo.DistributionNumber.Domain)
					if err != nil {
						return err
					}
				}
				if o.SequenceInfo.HouseSequence != nil {
					_, err = i.db.Exec(
						"INSERT INTO xobject_episode_sequenceinfo (episode_id, type, value, domain) VALUES ($1, $2, $3, $4)",
						extraid,
						"HouseSequence",
						o.SequenceInfo.HouseSequence.Value,
						o.SequenceInfo.HouseSequence.Domain)
					if err != nil {
						return err
					}
				}
				// TODO: loop for alternatenumber
			}
			break
		}
		// TODO: check availablility of parent (should probably be linked OR cleaned after the fact)
		_, err := i.db.Exec(
			"INSERT INTO xobject_baseobject_link (base_doi_id, xobject_id, parent_doi_id, xobject_type) VALUES ($1, $2, $3, $4)",
			baseObject.id,
			extraid,
			baseObject.parentId,
			t.Name())
		if err != nil {
			return err
		}
	}

	parselog.Log(log.LL_WARN, basecid.String(), fmt.Sprintf("%v %v", baseObject, extra))

	return nil
}
