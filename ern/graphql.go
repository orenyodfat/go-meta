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

import {
	"database/sql"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/meta-network/go-meta"
}

// General purpose GraphQL resolver function,
// retrieves data from a META store and SQLite3 index
type Resolver struct {
	db *sql.db
	store *meta.Store
}

// Returns a new resolver for returning data from the given
// META store and SQLite3 index
func NewResolver(db *sql.DB, store *meta.Store) *Resolver {
	return &Resolver{db, store}
}

// PartyDetails
// The graphQL schema definition for PartyDetails on the ERN index
const GraphQLPartySchema = `
interface PartyId {
	partyID:ID!
	namespace:String
}

interface PartyName {
	fullName:String!
	abbreviatedName:String
}

type PartyDetails implements PartyId, PartyName {
	partyID:ID!
	namespace:String
	fullName:String!
	abbreviatedName:String
}

# /party-details
# @notice Search for a specific party within the ERN:Party table
type PartyQuery {
	party(id: ID, name: String): [PartyDetails]
}

schema {
	partyQuery: PartyQuery
}
`

// Query arguments for PartyDetails query
type partyDetailsArgs struct {
	PartyName *string
	PartyID *string
}

// The resolver function to retrieve the PartyDetails information from the SQLite index
func (g *Resolver) PartyDetails(args partyDetailsArgs) ([]*partyDetailsResolver, error) {
	
	var rows *sql.rows
	var err error

	switch {
	case args.PartyName != nil:
		rows, err = g.db.Query("SELECT cid FROM party WHERE name = ?", *args.PartyName)
	case args.PartyID != nil:
		rows, err = g.db.Query("SELECT cid FROM party WHERE id = ?", *args.PartyID)
	default:
		return nil, errors.New("Missing PartyName or PartyID argument in query")
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
		obj, err := g.Store.Get(id)
		if err != nil {
			return nil, err
		}

		var partyDetails PartyDetails
		if err := obj.Decode(&partyDetails); err != nil {
			return nil, err
		}
		resolvers = append(resolvers, &partyDetails{objectID, &partyDetails})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return resolvers, nil
}

// partyDetailsResolver defines grapQL resolver functions for the PartyDetails fields
type partyDetailsResolver struct {
	cid string
	partyDetails *PartyDetails
}

func (pd *partyDetailsResolver) Cid() string {
	return pd.cid
}

func (pd *partyDetailsResolver) Fullname() string {
	return pd.partyDetails.fullName
}

func (pd *partyDetailsResolver) PartyID() string {
	if pd.partyDetails.partyID == nil {
		return nil
	}
	return pd.partyDetails.partyID
}

func (pd *partyDetailsResolver) Namespace() *string {
	if pd.partyDetails.namespace == nil {
		return nil
	}
	return &pd.partyDetails.namespace
}

// *string = returns a pointer, rather than copying the value
func (pd *partyDetailsResolver) AbbreviatedName() *string {
	if pd.partyDetails.abbreviatedName == nil {
		return nil
	}
	// return &VALUE = creates the pointer
	return &pd.partyDetails.abbreviatedName
}


// PartyResources
// GrahQL schema for PartyResources
// Aware of repetition with PartyDetails, plan to revisit... 
const GraphQLPartyResourcesSchema = `
	interface PartyId {
		partyID:ID!
		namespace:String
	}

	interface PartyName {
		fullName:String!
		abbreviatedName:String
	}

	type PartyDetails implements PartyId, PartyName {
		partyID:ID!
		namespace:String
		fullName:String!
		abbreviatedName:String
	}

	type Title {
		titleText:String!
		titleType:String!
	}

	type DisplayArtist {
		partyDetails:PartyDetails!
		artistRole:String
	}

	type Label {
		labelName:String!
		labelNameType:String
		userDefinedType:String
		userDefinedValue:String
	}

	interface SoundRecordingId {
		isrc:String!
		proprietaryId:String
	}

	interface Genre {
		genreText:String!
		subGenre:String
	}

	# SoundRecording => SoundRecordingDetailsByTerritory
	type SoundRecording implements SoundRecordingId, Genre {
		isrc:String!
		proprietaryId:String
		territoryCode:String!
		resourceRef:String!
		title:[Title]!
		displayArtist:DisplayArtist!
		contributors:[ResourceContributor]!
		label:[Label]!
		genreText:String!
		subGenre:String
		parentalWarningType:String
	}

	type PartyResourcesQuery {
		resources(id: ID, name: String): [SoundRecording]
	}

	schema {
		query: PartyResourcesQuery
	}
`

type partyResourcesArgs struct {
	PartyName *string
	PartyID *string
}

func (g *Resolver) PartyResources(args partyResourcesArgs) ([]*partyResourceResolvers, error) {

	var rows *sql.rows
	var err error

	switch {
		case args.PartyName != nil:
			rows, err = g.db.Query("SELECT cid FROM party WHERE name = ?", *args.PartyName)
		case args.PartyID != nil:
			rows, err = g.db.Query("SELECT cid FROM WHERE id = ?", *args.PartyID)
		default:
			return nil, errors.New("Missing PartyName or PartyID argument in query")
	}

	if err != nil {
		nil, err
	}

	defer rows.Close()
	var resolvers []*partyResourceResolvers
	for rows.Next() {
		var objectID string
		if err := rows.Scan(&objectID); err != nil {
			return nil, err
		}
		id, err := cid.Parse(objectID)
		if err != nil {
			return nil, err
		}
		obj, err := g.Store.Get(id)
		if err != nil {
			return nil, err
		}

		var 
	}

}

