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

package ern

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/meta-network/go-meta"
)

// General purpose GraphQL resolver function,
// retrieves data from a META store and SQLite3 index
type Resolver struct {
	db    *sql.DB
	store *meta.Store
}

// Returns a new resolver for returning data from the given
// META store and SQLite3 index
func NewResolver(db *sql.DB, store *meta.Store) *Resolver {
	return &Resolver{db, store}
}

// PartyDetails
// The graphQL schema definition for PartyDetails on the ERN index
const GraphQLPartyDetailsSchema = `
	schema {
		query: Query
	}

	type Query {
		partyDetails(id: String, name: String): [PartyDetails]!
		soundRecording(id: String, title: String): [SoundRecording]!
	}

	type PartyDetails {
		cid: String!
		partyID: String!
		fullName: String!
	}

	type ResourceContributor {
		partyDetails: PartyDetails!
		role: String!
	}

	type SoundRecording {
		artistName: String!
		# contributors: [ResourceContributor]!
		genre: String!
		# parentalWarningType: String
		resourceReference: String!
		# subGenre: String
		soundRecordingId: String!
		territoryCode: String!
		title: String!
	}
`

// Query arguments for PartyDetails query
type partyDetailsArgs struct {
	Name *string
	ID   *string
}

// Find a value within a MetaObject and decode it to a specific schema, then return result
// metaObj - the meta object to be decoded
// v - the schema defintion to decode to
// path - the tree path (that is found in the source data document (ERN))
func DecodeResolverObj(i *Resolver, metaObj *meta.Object, v interface{}, path ...string) (err error) {
	graph := meta.NewGraph(i.store, metaObj)

	defer func() {
		if err != nil {
			err = fmt.Errorf("Error decoding %s into %T: %s", path, v, err)
		}
	}()

	x, err := graph.Get(path...)
	if meta.IsPathNotFound(err) {
		return err
	} else if err != nil {
		return err
	}
	id, ok := x.(*cid.Cid)
	if !ok {
		return fmt.Errorf("Expected %s to be *cid.Cid, got %T", path, x)
	}

	obj, err := i.store.Get(id)
	if err != nil {
		return err
	}
	return obj.Decode(v)
}

// The resolver function to retrieve the PartyDetails information from the SQLite index
func (g *Resolver) PartyDetails(args partyDetailsArgs) ([]*partyDetailsResolver, error) {

	var rows *sql.Rows
	var err error

	switch {
	case args.Name != nil:
		rows, err = g.db.Query("SELECT cid FROM party WHERE name = ?", *args.Name)
	case args.ID != nil:
		rows, err = g.db.Query("SELECT cid FROM party WHERE id = ?", *args.ID)
	default:
		return nil, errors.New("Missing Name or ID argument in query")
	}

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var resolvers []*partyDetailsResolver
	for rows.Next() {
		var objectID string
		if err := rows.Scan(&objectID); err != nil {
			return nil, err
		}
		id, err := cid.Parse(objectID)
		if err != nil {
			return nil, err
		}

		obj, err := g.store.Get(id)
		if err != nil {
			return nil, err
		}

		var DdexPartyId struct {
			Value string `json:"@value"`
		}
		if err := DecodeResolverObj(g, obj, &DdexPartyId, "PartyId"); err != nil {
			pid := &DdexPartyId
			pid.Value = "000"
		}

		var DdexPartyName struct {
			Value string `json:"@value"`
		}
		if err := DecodeResolverObj(g, obj, &DdexPartyName, "PartyName", "FullName"); err != nil {
			return nil, err
		}
		// Not keen on the below, but refinement will take time :)
		var partyDetails PartyDetails
		partyDetails.PartyId = DdexPartyId.Value
		partyDetails.PartyName = DdexPartyName.Value
		resolvers = append(resolvers, &partyDetailsResolver{objectID, &partyDetails})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return resolvers, nil
}

// partyDetailsResolver defines grapQL resolver functions for the PartyDetails fields
type partyDetailsResolver struct {
	cid          string
	partyDetails *PartyDetails
}

func (pd *partyDetailsResolver) Cid() string {
	return pd.cid
}

func (pd *partyDetailsResolver) Fullname() string {
	return pd.partyDetails.PartyName
}

func (pd *partyDetailsResolver) PartyId() string {
	return pd.partyDetails.PartyId
}

/**
 * SoundRecording
 */

type soundRecordingArgs struct {
	ID   *string
	Title   *string
}

type soundRecordingResolver struct {
	cid string
	soundRecording *SoundRecording
}

func (sr *soundRecordingResolver) Cid() string {
	return sr.cid
}

func (sr *soundRecordingResolver) ArtistName() string {
	return sr.soundRecording.ArtistName
}

func (sr *soundRecordingResolver) Genre() string {
	return sr.soundRecording.GenreText
}

func (sr *soundRecordingResolver) ParentalWarningType() string {
	return sr.soundRecording.ParentalWarningType
}

func (sr *soundRecordingResolver) ResourceReference() string {
	return sr.soundRecording.ResourceReference
}

func (sr *soundRecordingResolver) SoundRecordingId() string {
	return sr.soundRecording.SoundRecordingId
}

func (sr *soundRecordingResolver) SubGenre() string {
	return sr.soundRecording.SubGenre
}

func (sr *soundRecordingResolver) TerritoryCode() string {
	return sr.soundRecording.TerritoryCode
}

func (sr *soundRecordingResolver) Title() string {
	return sr.soundRecording.ReferenceTitle
}

func (g *Resolver) SoundRecording(args soundRecordingArgs) ([]*soundRecordingResolver, error) {
	var rows *sql.Rows
	var err error

	switch {
	case args.ID != nil:
		rows, err = g.db.Query("SELECT cid FROM sound_recording WHERE id = ?", *args.ID)
	case args.Title != nil:
		rows, err = g.db.Query("SELECT cid FROM sound_recording WHERE title = ?", *args.Title)
	default:
		return nil, errors.New("Missing ID or Title argument in query")
	}

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var response []*soundRecordingResolver

	for rows.Next() {
		var objectID string

		if err := rows.Scan(&objectID); err != nil {
			return nil, err
		}

		id, err := cid.Parse(objectID)

		if err != nil {
			return nil, err
		}

		obj, err := g.store.Get(id)

		if err != nil {
			return nil, err
		}

		var ArtistName struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &ArtistName, "SoundRecordingDetailsByTerritory", "DisplayArtist", "PartyName", "FullName"); err != nil {
			return nil, err
		}

		var GenreText struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &GenreText, "SoundRecordingDetailsByTerritory", "Genre", "GenreText"); err != nil {
			return nil, err
		}

		var ParentalWarningType struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &ParentalWarningType, "SoundRecordingDetailsByTerritory", "ParentalWarningType"); err != nil {
			return nil, err
		}

		var ReferenceTitle struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &ReferenceTitle, "ReferenceTitle", "TitleText"); err != nil {
			return nil, err
		}

		var ResourceReference struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &ResourceReference, "ResourceReference"); err != nil {
			return nil, err
		}

		var SoundRecordingId struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &SoundRecordingId, "SoundRecordingId", "ISRC"); err != nil {
			return nil, err
		}

		var SubGenre struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &SubGenre, "SoundRecordingDetailsByTerritory", "Genre", "SubGenre"); err != nil {
			return nil, err
		}

		var TerritoryCode struct {
			Value string `json:"@value"`
		}

		if err := DecodeResolverObj(g, obj, &TerritoryCode, "SoundRecordingDetailsByTerritory", "TerritoryCode"); err != nil {
			return nil, err
		}

		var soundRecording SoundRecording
		soundRecording.ArtistName = ArtistName.Value
		soundRecording.GenreText = GenreText.Value
		soundRecording.ParentalWarningType = ParentalWarningType.Value
		soundRecording.ReferenceTitle = ReferenceTitle.Value
		soundRecording.ResourceReference = ResourceReference.Value
		soundRecording.SoundRecordingId = SoundRecordingId.Value
		soundRecording.SubGenre = SubGenre.Value
		soundRecording.TerritoryCode = TerritoryCode.Value

		response = append(response, &soundRecordingResolver{objectID, &soundRecording})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return response, nil
}
